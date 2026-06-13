package element

import (
	"circuit/mna"
	"fmt"
	"reflect"
	"sync"
	"time"
)

// Context 上下文。
type Context struct {
	mna.Time                                                 // 时间接口。
	*mna.MnaUpdateType[float64]                              // 求解矩阵。
	Nodelist                    []NodeFace                   // 元件列表。
	WaitGroup                   sync.WaitGroup               // 并发限制。
	CompactNodeID               map[mna.NodeID]int           // 原始节点ID→紧凑索引的映射。
	HierarchicalNodeID          map[string]mna.NodeID        // 层级路径(如"X1.out")→紧凑节点ID的映射。
	ParallelOpts                *ParallelOptions             // 并行仿真选项，nil=串行模式。
	stampCaches                 map[NodeFace]*mna.StampCache // 元件盖章缓存。
	cacheTime                   float64                      // 缓存时间戳。
	cacheMu                     sync.Mutex                   // 缓存访问互斥锁。
	HasReactive                 bool                         // 电路中包含储能元件（电容/电感）
	// eventValues 并发安全的事件值映射
	eventValues sync.Map
	// eventTargets 事件名 → NodeValue 槽位指针列表（懒初始化一次）
	// 保护 eventTargets map 的并发读写（MarkReset 写 / PushEvents、PullEvents 读）
	eventTargets map[string][]*any
	eventMu      sync.RWMutex
	resumeMu     sync.Mutex // 保护 resumeCond 的条件变量互斥锁
	resumeCond   *sync.Cond // 仿真恢复条件变量，状态从 Paused 离开时广播
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
		con.eventValues.Clear()
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
func (con *Context) InitResumeCond() {
	con.resumeCond = sync.NewCond(&con.resumeMu)
}

// ResumeCond 返回恢复条件变量，供外部设置 notifier 使用。
// 若未初始化则返回 nil。
func (con *Context) ResumeCond() *sync.Cond {
	return con.resumeCond
}

// rebuildEventTargets 重建事件目标指针表。
// 在 MarkReset / MarkUpdateElements / MarkRollbackElements 后调用，
// 确保 eventTargets 中的指针指向最新的 NodeValue 内存地址。
func (con *Context) rebuildEventTargets() {
	newTargets := make(map[string][]*any)
	for _, elem := range con.Nodelist {
		cfg := elem.Config()
		if cfg == nil || len(cfg.EventSlots) == 0 {
			continue
		}
		node := elem.Base()
		for nameIdx, valueIdx := range cfg.EventSlots {
			eventName := ""
			if valueIdx >= 0 && valueIdx < len(node.NodeValue) && nameIdx >= 0 && nameIdx < len(node.NodeValue) {
				if s, ok := node.NodeValue[nameIdx].(string); ok {
					eventName = s
				}
				if eventName == "" {
					continue
				}
				ptr := &node.NodeValue[valueIdx]
				newTargets[eventName] = append(newTargets[eventName], ptr)
			}
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
		for _, ptr := range targets {
			if *ptr != nil && reflect.TypeOf(*ptr) != reflect.TypeOf(vAny) {
				continue
			}
			*ptr = vAny
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
		// 使用 cond 等待状态变化，避免 busy-wait 轮询
		if con.resumeCond != nil {
			con.resumeMu.Lock()
			for con.Time.Status() == mna.StatusPaused {
				con.resumeCond.Wait()
			}
			con.resumeMu.Unlock()
		} else {
			time.Sleep(10 * time.Millisecond)
		}
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
				for _, ptr := range targets {
					if ptr == nil {
						continue
					}
					vt := reflect.TypeOf(*ptr)
					nvt := reflect.TypeOf(newValue)
					if vt == nvt {
						*ptr = newValue
					}
				}
			}
		}
	}
}
