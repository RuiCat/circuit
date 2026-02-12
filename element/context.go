package element

import (
	"circuit/mna"
	"log"
	"sync"
)

// Context 上下文。
type Context struct {
	mna.Time                                   // 时间接口。
	*mna.MnaUpdateType[float64]                // 求解矩阵。
	Nodelist                    []NodeFace     // 元件列表。
	WaitGroup                   sync.WaitGroup // 并发限制。
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

// CallMark 统一调用。
func (con *Context) CallMark(mark Mark) {
	switch mark {
	case MarkReset:
		for i := range con.Nodelist {
			ElementList[con.Nodelist[i].Base().NodeType].Reset(con.Nodelist[i])
		}
		con.MnaUpdateType.MnaType.A.Zero()
		con.MnaUpdateType.MnaType.Z.Zero()
		con.Update()
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
			ElementList[con.Nodelist[i].Base().NodeType].StartIteration(con, con.Time, con.Nodelist[i])
		}
	case MarkStamp:
		for i := range con.Nodelist {
			ElementList[con.Nodelist[i].Base().NodeType].Stamp(con, con.Time, con.Nodelist[i])
		}
	case MarkDoStep:
		for i := range con.Nodelist {
			ElementList[con.Nodelist[i].Base().NodeType].DoStep(con, con.Time, con.Nodelist[i])
		}
	case MarkCalculateCurrent:
		for i := range con.Nodelist {
			ElementList[con.Nodelist[i].Base().NodeType].CalculateCurrent(con, con.Time, con.Nodelist[i])
		}
	case MarkStepFinished:
		for i := range con.Nodelist {
			ElementList[con.Nodelist[i].Base().NodeType].StepFinished(con, con.Time, con.Nodelist[i])
		}
	default:
		log.Fatalf("未知 CallMark 操作: %d", mark)
	}
}
