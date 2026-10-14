package element

import (
	"circuit/mna"
	"fmt"
	"math"
	"reflect"
	"sync"
)

// EngineConfig 引擎仿真配置：由 load 包在创建 Context 时从环境变量解析注入，
// 供瞬态引擎（element/time/simulation.go）与 MNA 存储选择读取，
// 避免引擎内部直接 os.Getenv（配置集中、可测试、可程序化设置）。
//
// 过渡期职责（契约 K3/C13）：本结构仍是唯一数据源；element/time/engineConfig.go 提供两个
// 绑定视图——BackendBinding（载入期绑定，不可变）与 StrategyBinding（运行期，每次仿真实读）。
// 每个字段的绑定时刻见行尾标注，两者不可混用：后端开关的运行期改写必须报错。
type EngineConfig struct {
	SparseLU    bool    // CIRCUIT_LUSPARSE：强制稀疏 LU（未设置时按矩阵密度自动选择）[载入期绑定]
	ForceDense  bool    // CIRCUIT_LUSDENSE：强制稠密 LU（覆盖自动选择；与 SparseLU 互斥）[载入期绑定]
	GminCont    bool    // CIRCUIT_GMCONT：Gmin 延续步进 [运行期·每次仿真实读]
	NaturalGmin float64 // CIRCUIT_NATGMIN：自然 gmin（对角叠加）[运行期·每次仿真实读]
	GminFinal   float64 // CIRCUIT_GMINFINAL：延续终值（<=0 用默认）[运行期·每次仿真实读]
	GlobalGmin  float64 // CIRCUIT_GLOBALGMIN：全局对角 gmin（0=关闭；支持显式数值）[运行期·每次仿真实读]
	GminDbg     bool    // CIRCUIT_GMIN_DBG：打印 gmin 步进轨迹 [运行期·每次仿真实读]
	NoEq        bool    // CIRCUIT_NOEQ：跳过行/列均衡化 [运行期·每次仿真实读]
}

// BackendBinding 引擎求解后端的绑定（契约 K4/K3）：稀疏/稠密 LU 的存储与求解后端选择。
//
// 绑定时刻是**载入期**（load.LoadString 调 Bind），创建后不可变：运行期改写
// 会改变已绑定后端时 Validate 报错（文案含"载入期绑定"），禁止静默成功、禁止部分生效。
// 未绑定的零值（Bound=false）只做互斥校验，供不经过 load 的构造路径使用。
//
// 定义在 element 包内以避免 element ↔ element/time 循环导入
// （element/time 的内部测试依赖 load，load 不能再依赖 element/time）；
// element/time/engineConfig.go 以类型别名转发本类型，契约里的 `time.BackendBinding` 仍可用。
type BackendBinding struct {
	SparseLU    bool // CIRCUIT_LUSPARSE：强制稀疏 LU
	ForceDense  bool // CIRCUIT_LUSDENSE：强制稠密 LU（与 SparseLU 互斥）
	Bound       bool // 是否已完成载入期绑定
	BoundSparse bool // 载入期实际绑定的矩阵存储（稀疏=true；判据 SparseLU || !ForceDense）

	boundExplicit   bool // 载入期是否由环境变量显式绑定（两个开关都未设=按密度自动）
	boundSparseLU   bool // 载入期绑定的 SparseLU
	boundForceDense bool // 载入期绑定的 ForceDense
}

// autoSparseDensity 自动选择阈值：矩阵非零密度低于此值用稀疏 LU。
// 基准（2026-08-22）：RTL 稀疏矩阵（每行 ~10 非零）稀疏 LU 全面占优
// （n=50 起即不慢于稠密，n=300 快 25 倍）；稠密矩阵（nnz~80%）稀疏退化
// 2~5 倍（fill-in 开销）。阈值 0.3 分隔两种形态。ResolveBackend 的唯一出处。
const autoSparseDensity = 0.3

// storageSparseOf 返回给定开关组合对应的矩阵存储：稀疏=true。
// 判据与 load.LoadContext 载入期选择存储时逐字一致（SparseLU || !ForceDense）。
func storageSparseOf(sparseLU, forceDense bool) bool {
	return sparseLU || !forceDense
}

// Bind 载入期绑定（由 load.LoadContext 在创建 Context 后调用一次）：
// 冻结后端值与"是否显式绑定"的事实，并以同一判据记录实际存储后端。
//
// 刻意**不**在此拒绝 SparseLU && ForceDense 同时为真：那是"CIRCUIT_LUSPARSE 与
// CIRCUIT_LUSDENSE 两个环境变量同时设置"的既存组合（本轮只登记，不改载入行为），
// 互斥校验由 Validate / ResolveBackend 在显式绑定路径与求解路径上执行。
func (b *BackendBinding) Bind(sparseLU, forceDense bool) {
	b.SparseLU, b.ForceDense = sparseLU, forceDense
	b.Bound = true
	b.BoundSparse = storageSparseOf(sparseLU, forceDense)
	b.boundExplicit = sparseLU || forceDense
	b.boundSparseLU, b.boundForceDense = sparseLU, forceDense
}

// Validate 校验后端绑定：
//  1. SparseLU 与 ForceDense 互斥（原注释声称互斥但代码不校验，靠 switch 顺序隐含裁决）；
//  2. 载入期绑定后（Bind 已调用）改写后端 → 报错，文案含"载入期绑定"，禁止部分生效：
//     - 载入期由环境变量显式绑定时，任何后端开关改动都算改写；
//     - 载入期未显式绑定（自动模式）时，只拒绝会改变已绑定**矩阵存储**的写入
//     （默认载入即稀疏存储，故 forceDense=true 报错；sparseLU=true 是无操作，按原语义放行）。
func (b *BackendBinding) Validate() error {
	if b.SparseLU && b.ForceDense {
		return fmt.Errorf("引擎后端绑定非法：SparseLU 与 ForceDense 互斥（CIRCUIT_LUSPARSE 与 CIRCUIT_LUSDENSE 不可同时为真）")
	}
	if !b.Bound {
		return nil
	}
	if b.boundExplicit {
		if b.SparseLU != b.boundSparseLU || b.ForceDense != b.boundForceDense {
			return fmt.Errorf("引擎后端为载入期绑定（载入时 CIRCUIT_LUSPARSE=%v / CIRCUIT_LUSDENSE=%v），运行期不可改写：sparseLU %v→%v，forceDense %v→%v",
				b.boundSparseLU, b.boundForceDense, b.boundSparseLU, b.SparseLU, b.boundForceDense, b.ForceDense)
		}
		return nil
	}
	if storageSparseOf(b.SparseLU, b.ForceDense) != b.BoundSparse {
		return fmt.Errorf("引擎后端存储为载入期绑定（载入时已按 SparseLU||!ForceDense 绑定存储=稀疏%v），运行期改写会改变已绑定后端：稀疏 %v→%v",
			b.BoundSparse, b.BoundSparse, storageSparseOf(b.SparseLU, b.ForceDense))
	}
	return nil
}

// ResolveBackend 返回该绑定下**实际生效**的稀疏标志（与 simulation.go 的 chooseLUSolver
// 逐分支一致）：SparseLU → 稀疏；ForceDense → 稠密；workers>1 → 并行稠密；
// 否则按矩阵非零密度 density 自动选择（< autoSparseDensity 用稀疏）。
//
// 先过 Validate：非法绑定（两个开关同时为真 / 运行期改写）报错，
// 而不是像原 switch 那样按顺序隐含裁决。
func (b BackendBinding) ResolveBackend(density float64, workers int) (sparse bool, err error) {
	if err := b.Validate(); err != nil {
		return false, err
	}
	switch {
	case b.SparseLU:
		return true, nil
	case b.ForceDense:
		return false, nil
	case workers > 1:
		return false, nil
	}
	if math.IsNaN(density) || density < 0 {
		return false, fmt.Errorf("矩阵非零密度非法：%v（无法据此选择 LU 后端）", density)
	}
	return density < autoSparseDensity, nil
}

// CurrentBackend 返回"当前后端值 + 载入期冻结记录"的绑定视图：
// 当前值取自 EngineCfg（运行期运行期写入点改的是它），冻结记录取自 con.Backend。
// 求解期（chooseLUSolver）与运行期试写（MCP）都用这个视图做校验。
func (con *Context) CurrentBackend() BackendBinding {
	b := con.Backend
	b.SparseLU, b.ForceDense = con.EngineCfg.SparseLU, con.EngineCfg.ForceDense
	return b
}

// Context 上下文。
type Context struct {
	mna.Time                                                 // 时间接口。
	*mna.MnaUpdateType[float64]                              // 求解矩阵。
	Nodelist                    []NodeFace                   // 元件列表。
	WaitGroup                   sync.WaitGroup               // 并发限制。
	CompactNodeID               map[mna.NodeID]int           // 原始节点ID→紧凑索引的映射。
	HierarchicalNodeID          map[string]mna.NodeID        // 层级路径(如"X1.out")→紧凑节点ID的映射。
	ParallelOpts                *ParallelOptions             // 并行仿真选项，nil=串行模式。
	EngineCfg                   EngineConfig                 // 引擎仿真配置（load 解析环境变量注入）。
	Backend                     BackendBinding               // 求解后端的载入期绑定记录（不可变；当前值见 CurrentBackend）。
	stampCaches                 map[NodeFace]*mna.StampCache // 元件盖章缓存。
	cacheTime                   float64                      // 缓存时间戳。
	cacheMu                     sync.Mutex                   // 缓存访问互斥锁。
	HasReactive                 bool                         // 电路中包含储能元件（电容/电感）
	// eventValues 并发安全的事件值映射
	eventValues sync.Map
	// eventTargets 事件名 → 消费者写入槽位列表（懒初始化一次）。
	// 保存 Node 指针 + 索引而非裸 *any 指针，写入前校验边界，避免 NodeValue 扩容后指针悬垂。
	// 保护 eventTargets map 的并发读写（MarkReset 写 / PushEvents、PullEvents 读）
	eventTargets map[string][]eventTarget
	eventMu      sync.RWMutex
	resumeMu     sync.Mutex // 保护 resumeCond 的条件变量互斥锁
	resumeCond   *sync.Cond // 仿真恢复条件变量，状态从 Paused 离开时广播
	resumeOnce   sync.Once  // 保证 resumeCond 惰性初始化仅执行一次
}

// eventTarget 表示事件消费者的一个写入槽位。
// 保存目标 Node 指针与 NodeValue 索引，写入前校验索引边界，
// 避免直接保存 &NodeValue[i] 裸指针在 NodeValue 扩容后悬垂。
type eventTarget struct {
	node     *Node
	valueIdx int
}

// ComputeStateDerivative 基于当前 MNA 解和元件状态计算状态导数向量 dx/dt。
// 通过 ElementFace.AddDerivative 接口分发，仅储能元件贡献非零导数。
func (con *Context) ComputeStateDerivative() []float64 {
	n := con.GetNodeNum() + con.GetVoltageSourcesNum()
	der := make([]float64, n)
	for _, elem := range con.Nodelist {
		cfg := elem.Config()
		if cfg != nil && cfg.IsFlag(FlagReactive) {
			elemFace, ok := getElementFace(elem.Type())
			if !ok {
				continue
			}
			elemFace.AddDerivative(con, con.Time, elem, der)
		}
	}
	return der
}

// HasReactiveElements 返回电路中是否包含储能元件（电容或电感）。
func (con *Context) HasReactiveElements() bool {
	return con.HasReactive
}

// Update 将对矩阵A和向量Z的暂存修改应用到底层数据结构中。
func (con *Context) Update() {
	con.MnaUpdateType.Update()
	for i := range con.Nodelist {
		con.Nodelist[i].Update()
	}
}

// Rollback 丢弃对矩阵A和向量Z的暂存修改，将其恢复到上次更新或初始状态。
func (con *Context) Rollback() {
	con.MnaUpdateType.Rollback()
	for i := range con.Nodelist {
		con.Nodelist[i].Rollback()
	}
}

// GetRawNodeVoltage 从原始节点ID获取电压。
func (con *Context) GetRawNodeVoltage(rawNodeID mna.NodeID) float64 {
	compactIdx, ok := con.CompactNodeID[rawNodeID]
	if !ok {
		return 0
	}
	return con.GetNodeVoltage(mna.NodeID(compactIdx))
}

// GetHierarchicalNodeVoltage 从层级路径（如 "X1.out"）获取电压。
func (con *Context) GetHierarchicalNodeVoltage(path string) float64 {
	compactIdx, ok := con.HierarchicalNodeID[path]
	if !ok {
		return 0
	}
	return con.GetNodeVoltage(compactIdx)
}

// CallMark 统一调用。
func (con *Context) CallMark(mark Mark) error {
	switch mark {
	case MarkReset:
		for i := range con.Nodelist {
			elemFace, ok := getElementFace(con.Nodelist[i].Base().NodeType)
			if !ok {
				continue
			}
			elemFace.Reset(con.Nodelist[i])
		}
		con.MnaUpdateType.MnaType.A.Zero()
		con.MnaUpdateType.MnaType.Z.Zero()
		con.Update()
		// 注意：不再 Clear eventValues——事件值跨仿真任务保留。
		// 交互式控制（TUI/MCP 的 `set <事件> <值>` 后 `step`/`run`）依赖：
		// 若每次任务启动清空，先 set 后 run 的事件会被丢弃（开关永远不闭合）。
		// 新网表加载（LoadString）创建新 Context，eventValues 天然为空；
		// PushEvents 消费后置 nil，任务间事件值保持为最后一次消费值。
		con.rebuildEventTargets()
	case MarkUpdateElements:
		con.UpdateX()
		for i := range con.Nodelist {
			con.Nodelist[i].Base().Update()
		}
	case MarkRollbackElements:
		con.RollbackX()
		for i := range con.Nodelist {
			con.Nodelist[i].Base().Rollback()
		}
	case MarkStartIteration:
		for i := range con.Nodelist {
			elemFace, ok := getElementFace(con.Nodelist[i].Base().NodeType)
			if !ok {
				continue
			}
			elemFace.StartIteration(con, con.Time, con.Nodelist[i])
		}
	case MarkStamp:
		for i := range con.Nodelist {
			elemFace, ok := getElementFace(con.Nodelist[i].Base().NodeType)
			if !ok {
				continue
			}
			elemFace.Stamp(con, con.Time, con.Nodelist[i])
		}
	case MarkDoStep:
		for i := range con.Nodelist {
			elemFace, ok := getElementFace(con.Nodelist[i].Base().NodeType)
			if !ok {
				continue
			}
			elemFace.DoStep(con, con.Time, con.Nodelist[i])
		}
	case MarkCalculateCurrent:
		for i := range con.Nodelist {
			elemFace, ok := getElementFace(con.Nodelist[i].Base().NodeType)
			if !ok {
				continue
			}
			elemFace.CalculateCurrent(con, con.Time, con.Nodelist[i])
		}
	case MarkStepFinished:
		for i := range con.Nodelist {
			elemFace, ok := getElementFace(con.Nodelist[i].Base().NodeType)
			if !ok {
				continue
			}
			elemFace.StepFinished(con, con.Time, con.Nodelist[i])
		}
	default:
		return fmt.Errorf("未知 CallMark 操作: %d", mark)
	}
	return nil
}

// SetEvent 设置事件值（可从任意 goroutine 安全调用）
func (con *Context) SetEvent(name string, value any) {
	con.eventValues.Store(name, value)
}

// GetEvent 读取指定事件名的当前值
// 若事件不存在返回 nil
func (con *Context) GetEvent(name string) any {
	v, ok := con.eventValues.Load(name)
	if !ok {
		return nil
	}
	return v
}

// InitResumeCond 初始化恢复条件变量（由 load 或 cmd 在创建 Context 后调用）。
// 幂等：已在 resumeMu 保护下判 nil，可安全地被仿真协程（resumeOnce）重复调用，
// 不会与外部 ResumeCond() 读取产生数据竞争。
func (con *Context) InitResumeCond() {
	con.resumeMu.Lock()
	if con.resumeCond == nil {
		con.resumeCond = sync.NewCond(&con.resumeMu)
	}
	con.resumeMu.Unlock()
}

// ResumeCond 返回恢复条件变量，供外部设置 notifier 使用。
// 若未初始化则返回 nil。加锁读取，与 InitResumeCond 的惰性初始化并发安全。
func (con *Context) ResumeCond() *sync.Cond {
	con.resumeMu.Lock()
	defer con.resumeMu.Unlock()
	return con.resumeCond
}

// rebuildEventTargets 重建事件目标槽位表。
// 在 MarkReset 后调用，收集各元件的消费者槽位（Node 指针 + 索引）。
// 通过索引定位，写入时再校验边界，NodeValue 扩容不会导致指针悬垂。
func (con *Context) rebuildEventTargets() {
	newTargets := make(map[string][]eventTarget)
	for _, elem := range con.Nodelist {
		cfg := elem.Config()
		if cfg == nil || len(cfg.EventSlots) == 0 {
			continue
		}
		node := elem.Base()
		for nameIdx, valueIdx := range cfg.EventSlots {
			// 只处理消费者（valueIdx >= 0）；生产者（负数）在 PullEvents 中按元件遍历处理。
			if valueIdx < 0 || valueIdx >= len(node.NodeValue) {
				continue
			}
			eventName := ""
			if nameIdx >= 0 && nameIdx < len(node.NodeValue) {
				if s, ok := node.NodeValue[nameIdx].(string); ok {
					eventName = s
				}
			}
			if eventName == "" {
				continue
			}
			// 保存索引而非 &NodeValue[valueIdx] 裸指针，NodeValue 扩容后索引仍指向同一逻辑槽位。
			newTargets[eventName] = append(newTargets[eventName], eventTarget{node: node, valueIdx: valueIdx})
		}
	}
	con.eventMu.Lock()
	con.eventTargets = newTargets
	con.eventMu.Unlock()
}

// PushEvents 将事件值同步到各元件的 NodeValue 中
func (con *Context) PushEvents() bool {
	con.eventMu.RLock()
	defer con.eventMu.RUnlock()
	for eventName, targets := range con.eventTargets {
		vAny, ok := con.eventValues.Load(eventName)
		if !ok {
			continue
		}
		// nil 表示已被消费，跳过
		if vAny == nil {
			continue
		}
		for _, t := range targets {
			if t.node == nil || t.valueIdx < 0 || t.valueIdx >= len(t.node.NodeValue) {
				continue // 槽位失效，跳过
			}
			cur := t.node.NodeValue[t.valueIdx]
			if cur != nil && reflect.TypeOf(cur) != reflect.TypeOf(vAny) {
				continue
			}
			t.node.NodeValue[t.valueIdx] = vAny
		}
		// 消费后设为 nil，防止下次重复推送
		con.eventValues.Store(eventName, nil)
	}
	// 连续模式状态检查：处理暂停/停止/单步
	for {
		switch con.Time.Status() {
		case mna.StatusStopped: // 停止运行
			return false
		case mna.StatusRunning: // 正常运行
			return true
		case mna.StatusStepping: // 单步运行
			con.Time.SetStatus(mna.StatusStepping, mna.StatusPaused)
			return true
		case mna.StatusPaused: // 暂停
		}
		// 惰性初始化 resumeCond（sync.Once 保证仅一次），避免未调用 InitResumeCond 时退化到 sleep 轮询。
		con.resumeOnce.Do(con.InitResumeCond)
		con.resumeMu.Lock()
		for con.Time.Status() == mna.StatusPaused {
			con.resumeCond.Wait()
		}
		con.resumeMu.Unlock()
	}
}

// PullEvents 将生产者的 NodeValue 直接推送到消费者 NodeValue
// EventSlots 中 valueIdx 为负数的条目表示生产者：srcIdx = -valueIdx
// 从 NodeValue[srcIdx] 读取值，以 NodeValue[nameIdx] 中的字符串作为事件名
// 直接写入所有监听该事件名的消费者的 NodeValue，不经过 eventValues
// 每个事件名应只有一个生产者（否则后执行的覆盖前者）
// 由仿真协程在每步成功后调用，位于 call(voltages) 之后
func (con *Context) PullEvents() {
	con.eventMu.RLock()
	defer con.eventMu.RUnlock()
	for _, elem := range con.Nodelist {
		cfg := elem.Config()
		if cfg == nil || len(cfg.EventSlots) == 0 {
			continue
		}
		node := elem.Base()
		for nameIdx, valueIdx := range cfg.EventSlots {
			// 只处理生产者（负数索引表示从元件读取值）
			if valueIdx >= 0 {
				continue
			}
			srcIdx := -valueIdx
			if srcIdx >= len(node.NodeValue) {
				continue
			}
			// 从 nameIdx 读取事件名称
			eventName := ""
			if nameIdx < len(node.NodeValue) {
				if s, ok := node.NodeValue[nameIdx].(string); ok {
					eventName = s
				}
			}
			if eventName == "" {
				continue
			}
			// 生产者的值直接推送到所有消费者的 NodeValue，不经过 eventValues
			newValue := node.NodeValue[srcIdx]
			// nil 表示跳过（如 MomentarySwitch 保持期中的 autoResetValue）
			if newValue == nil {
				continue
			}
			if targets, ok := con.eventTargets[eventName]; ok {
				for _, t := range targets {
					if t.node == nil || t.valueIdx < 0 || t.valueIdx >= len(t.node.NodeValue) {
						continue
					}
					vt := reflect.TypeOf(t.node.NodeValue[t.valueIdx])
					nvt := reflect.TypeOf(newValue)
					if vt == nvt {
						t.node.NodeValue[t.valueIdx] = newValue
					}
				}
			}
		}
	}
}
