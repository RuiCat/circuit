// adapter.go 把真实 element.Context 接到 K5 契约 serialbridge.SlotIo 上：
// 按实例名定位元件 Node、按槽索引读 NodeValue、把宿主事件写进引擎事件表，
// 并驱动引擎走一个时间步后报告本步被消费的事件名。
//
// 槽索引口径（K2，见 serialbridge/slots.go，只读）：对 UART 实例即其 NodeValue 下标——
// 0 baud、1 ackEvent、2 VHigh、3 txByte、…、11 rxByte、12 dataReady、13 frameError、15 ackFlag。
// 本适配器不认识槽语义，只认"该实例自有的槽"：索引超出实例 NodeValue 长度、槽值为 nil、
// 槽值非数值一律返回错误（wrap serialbridge.ErrSlotValue，用 errors.Is 判定），不返回 0 冒充数据。
//
// 事件消费口径（K5 的 Advance 返回值）：引擎每步开头 Context.PushEvents 把有消费者且非 nil 的
// 事件写进 NodeValue 并把事件表置 nil（element/context.go:371-396），因此"本步是否被消费"在
// 真实 Context 上**可观测**，无需读私有字段：本适配器只对自己写过的名字判定
// Context.GetEvent(name) == nil。没有消费者的名字不会被置 nil ⇒ 永远不算被消费（桥按背压处理）。
package elementio

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"circuit/element"
	"circuit/serialbridge"
)

// 编译期断言：本适配器实现 K5 契约（serialbridge.SlotIo）。
var _ serialbridge.SlotIo = (*Adapter)(nil)

// 默认参数：Options 的零值字段取这些值。
const (
	// DefaultTargetTime 是引擎目标时间（秒）。适配器让引擎跑连续模式，
	// 该值只作为时间控制器的初值，不决定推进多少步。
	DefaultTargetTime = 1.0
	// DefaultStepTimeout 是 Advance 等待单步完成的上限：超时返回 ErrStepTimeout，
	// 不无界等待。
	DefaultStepTimeout = 5 * time.Second
	// DefaultCloseTimeout 是 Close 等待引擎 goroutine 退出的上限。
	DefaultCloseTimeout = 5 * time.Second
)

// 可判定错误：调用方一律用 errors.Is 判定，不解析错误文本。
var (
	// ErrNilContext：构造适配器时 element.Context 为空。
	ErrNilContext = errors.New("elementio: element.Context 为空")
	// ErrEmptyInstance：构造适配器时未给实例名。
	ErrEmptyInstance = errors.New("elementio: 实例名为空")
	// ErrInstanceMissing：顶层元件列表里不存在该实例名（无法驱动该元件）。
	ErrInstanceMissing = errors.New("elementio: 网表中不存在该实例")
	// ErrEmptyEventName：WriteEvent 的事件名为空。
	ErrEmptyEventName = errors.New("elementio: 事件名为空")
	// ErrAdapterClosed：适配器已关闭，不再接受写事件与推进（读槽仍可读）。
	ErrAdapterClosed = errors.New("elementio: 适配器已关闭")
	// ErrEngineStopped：仿真引擎已停止或已因错误退出，无法再推进。
	ErrEngineStopped = errors.New("elementio: 仿真引擎已停止")
	// ErrStepTimeout：单步等待超时（在途事件按"未被消费"处理，桥据此继续背压）。
	ErrStepTimeout = errors.New("elementio: 单步等待超时")
	// ErrCloseTimeout：关闭时引擎 goroutine 在期限内未退出（引擎可能仍在跑当前步）。
	ErrCloseTimeout = errors.New("elementio: 关闭等待引擎退出超时")
)

// Options 是适配器参数；零值即默认（见 Default* 常量）。
type Options struct {
	TargetTime   float64       // 引擎目标时间（秒）；<=0 取 DefaultTargetTime
	StepTimeout  time.Duration // Advance 单步等待上限；<=0 取 DefaultStepTimeout
	CloseTimeout time.Duration // Close 等待引擎退出上限；<=0 取 DefaultCloseTimeout
	// MinStep/MaxStep 是引擎步长上界（秒）。时序类元件（UART/TTL 门）必须给出上界，
	// 否则自适应步长会让采样点漂移（C1）。两者都 >0 且 MinStep<MaxStep 时才设置；
	// 否则沿用引擎默认（不把非法值静默写进引擎）。
	MinStep float64
	MaxStep float64
}

// Adapter 是 K5 契约（serialbridge.SlotIo）在真实 element.Context 上的实现：
// 构造时按实例名钉住一个元件 Node 并启动引擎 goroutine，Advance 每次推进一步。
// 并发约定：ReadSlot/WriteEvent 可从任意 goroutine 调用；Advance 内部串行化；
// Close 与其余方法并发安全。引擎侧只走引擎自身的收敛路径，不直接写解向量。
type Adapter struct {
	con      *element.Context
	engine   *engineDriver
	node     *element.Node
	instance string

	pendingMu sync.Mutex          // 保护 pending
	pending   map[string]struct{} // 已写事件里"尚未被引擎置 nil"的名字
	advanceMu sync.Mutex          // 串行化 Advance
	closedMu  sync.RWMutex        // 保护 closed
	closed    bool                // Close 已开始
	closeOnce sync.Once           // 保证 Stop 只发一次
	stepWait  time.Duration       // Advance 单步等待上限
	closeWait time.Duration       // Close 等待引擎退出上限
}

// NewAdapter 按实例名把适配器钉到一个真实元件上，并启动瞬态引擎（连续模式、暂停起步）：
// 只有 Advance 会触发单步，避免适配器一建立就自由跑。
// 返回错误的情形：con 为空、实例名为空、顶层元件列表里没有该实例、时间控制器创建失败。
func NewAdapter(con *element.Context, instance string, opts Options) (*Adapter, error) {
	if con == nil {
		return nil, ErrNilContext
	}
	if instance == "" {
		return nil, ErrEmptyInstance
	}
	node, err := findInstance(con, instance)
	if err != nil {
		return nil, err
	}
	target := opts.TargetTime
	if target <= 0 {
		target = DefaultTargetTime
	}
	stepWait := opts.StepTimeout
	if stepWait <= 0 {
		stepWait = DefaultStepTimeout
	}
	closeWait := opts.CloseTimeout
	if closeWait <= 0 {
		closeWait = DefaultCloseTimeout
	}
	eng, err := newEngineDriver(con, target, opts.MinStep, opts.MaxStep)
	if err != nil {
		return nil, err
	}
	return &Adapter{
		con:       con,
		engine:    eng,
		node:      node,
		instance:  instance,
		pending:   make(map[string]struct{}),
		stepWait:  stepWait,
		closeWait: closeWait,
	}, nil
}

// Instance 返回适配器钉住的实例名（供诊断与网表对照）。
func (a *Adapter) Instance() string { return a.instance }

// ReadSlot 读取实例 NodeValue[idx] 并归一成 float64：数值原样返回，bool 槽按 true=1/false=0。
// 索引越界（idx < 0 或超出该实例槽数）、槽值为 nil、槽值非数值时返回错误，绝不返回 0 冒充数据；
// 三类错误都用 %w 包住 serialbridge.ErrSlotValue，调用方用 errors.Is 判定。
func (a *Adapter) ReadSlot(idx int) (float64, error) {
	slotCount := len(a.node.NodeValue)
	if idx < 0 || idx >= slotCount {
		return 0, fmt.Errorf("%w: 实例 %s 槽 %d 越界（该实例共 %d 槽）",
			serialbridge.ErrSlotValue, a.instance, idx, slotCount)
	}
	raw := a.node.NodeValue[idx]
	if raw == nil {
		return 0, fmt.Errorf("%w: 实例 %s 槽 %d 为空值（nil）",
			serialbridge.ErrSlotValue, a.instance, idx)
	}
	value, ok := slotNumber(raw)
	if !ok {
		return 0, fmt.Errorf("%w: 实例 %s 槽 %d 类型 %T 不是数值",
			serialbridge.ErrSlotValue, a.instance, idx, raw)
	}
	return value, nil
}

// WriteEvent 把宿主事件写入引擎事件表（等价 element.Context.SetEvent），并把这个名字登记为
// "在途"：只有 Advance 观察到它被引擎消费（事件表被置 nil）才移出在途集合。
// 名字为空返回 ErrEmptyEventName；适配器已关闭返回 ErrAdapterClosed。
// 值本身就是引擎的单值存储语义：同名事件在一步内再次写入会覆盖前值（背压由桥负责，见 C15）。
func (a *Adapter) WriteEvent(name string, value float64) error {
	if name == "" {
		return ErrEmptyEventName
	}
	if a.isClosed() {
		return ErrAdapterClosed
	}
	// 登记与写入必须原子（相对 collectConsumed）：否则"登记在途 → 引擎先消费旧值 → 写新值"
	// 会让适配器把新值误报成已消费，桥随之解除背压，造成静默覆盖。
	a.pendingMu.Lock()
	defer a.pendingMu.Unlock()
	a.con.SetEvent(name, value)
	a.pending[name] = struct{}{}
	return nil
}

// Advance 推进一个仿真步，返回本步被引擎消费的事件名（按字典序）。
// 判定方式：本步开头的 PushEvents 会把取用的事件置 nil（element/context.go:371-396），
// 因此对每个在途名字读 Context.GetEvent(name) == nil 即"本步被消费"；未在集合中的在途名字
// 表示本步没被取走（背压），没有消费者的名字永远不会被置 nil，故永不报告为已消费。
// 有界性：等待单步完成、引擎退出、超时三条出口，绝不无界等待；超时返回 ErrStepTimeout
// （在途名字保持未消费状态，宁可保守也不误报）。
func (a *Adapter) Advance() ([]string, error) {
	a.advanceMu.Lock()
	defer a.advanceMu.Unlock()
	if a.isClosed() {
		return nil, ErrAdapterClosed
	}
	if err := a.engine.failure(); err != nil {
		return nil, err
	}
	a.engine.drainStepDone() // 丢掉上一轮超时残留的完成信号
	a.engine.stepOnce()
	if err := a.engine.awaitStep(a.stepWait); err != nil {
		return nil, err
	}
	return a.collectConsumed(), nil
}

// Close 停止引擎并等待其 goroutine 退出（幂等）。
// 引擎若正在跑当前步，等它跑完；超过 CloseTimeout 仍未退出返回 ErrCloseTimeout
// （不阻塞调用方）。关闭后 ReadSlot 仍可读（纯读），WriteEvent/Advance 返回 ErrAdapterClosed。
func (a *Adapter) Close() error {
	a.closeOnce.Do(func() {
		a.closedMu.Lock()
		a.closed = true
		a.closedMu.Unlock()
		a.engine.stop()
	})
	return a.engine.awaitExit(a.closeWait)
}

// isClosed 报告适配器是否已开始关闭。
func (a *Adapter) isClosed() bool {
	a.closedMu.RLock()
	defer a.closedMu.RUnlock()
	return a.closed
}

// collectConsumed 结算在途集合：事件表里已为 nil 的名字算本步被消费并从在途集合移除。
func (a *Adapter) collectConsumed() []string {
	a.pendingMu.Lock()
	defer a.pendingMu.Unlock()
	consumed := make([]string, 0, len(a.pending))
	for name := range a.pending {
		if a.con.GetEvent(name) == nil {
			consumed = append(consumed, name)
		}
	}
	for _, name := range consumed {
		delete(a.pending, name)
	}
	sort.Strings(consumed)
	return consumed
}

// findInstance 在顶层元件列表里按实例名定位元件 Node；找不到返回 ErrInstanceMissing。
// 只查顶层：层级子元件的名字（如 "X1.R1"）挂在父元件的 Children 下，不在 Nodelist 里。
func findInstance(con *element.Context, instance string) (*element.Node, error) {
	for _, face := range con.Nodelist {
		node := face.Base()
		if node != nil && node.InstanceName == instance {
			return node, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrInstanceMissing, instance)
}

// slotNumber 把槽值归一成 float64：数值直接返回，bool 按 true=1/false=0 返回，
// 其余类型（string 等）返回 ok=false，由调用方转成错误——绝不返回 0 冒充数据。
func slotNumber(raw any) (value float64, ok bool) {
	switch v := raw.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}
