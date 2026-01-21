package element

import (
	"bufio"
	"circuit/maths"
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
		for _, v := range con.Nodelist {
			ElementList[v.Base().NodeType].Reset(v)
		}
		con.MnaUpdateType.MnaType.A.Zero()
		con.MnaUpdateType.MnaType.Z.Zero()
		con.Update()
	case MarkUpdateElements:
		con.UpdateX()
		for _, elem := range con.Nodelist {
			elem.Base().Update()
		}
	case MarkRollbackElements:
		con.RollbackX()
		for _, elem := range con.Nodelist {
			elem.Base().Rollback()
		}
	case MarkStartIteration:
		for _, v := range con.Nodelist {
			ElementList[v.Base().NodeType].StartIteration(con, con.Time, v)
		}
	case MarkStamp:
		for _, v := range con.Nodelist {
			ElementList[v.Base().NodeType].Stamp(con, con.Time, v)
		}
	case MarkDoStep:
		for _, v := range con.Nodelist {
			ElementList[v.Base().NodeType].DoStep(con, con.Time, v)
		}
	case MarkCalculateCurrent:
		for _, v := range con.Nodelist {
			ElementList[v.Base().NodeType].CalculateCurrent(con, con.Time, v)
		}
	case MarkStepFinished:
		for _, v := range con.Nodelist {
			ElementList[v.Base().NodeType].StepFinished(con, con.Time, v)
		}
	default:
		log.Fatalf("未知 CallMark 操作: %d", mark)
	}
}

// LoadContext 加载仿真网表。
func LoadContext(scanner *bufio.Scanner) (con *Context, err error) {
	var nodesNum, voltageSourcesNum int
	con = &Context{}
	con.Nodelist, nodesNum, voltageSourcesNum, err = parseNetlist(scanner)
	if err != nil {
		return nil, err
	}
	// 总方程数量
	n := nodesNum + voltageSourcesNum
	// 创建可更新的矩阵和向量
	con.MnaUpdateType = &mna.MnaUpdateType[float64]{
		MnaType: &mna.MnaType[float64]{
			NodesNum:          nodesNum,
			VoltageSourcesNum: voltageSourcesNum,
		},
		A:     maths.NewUpdateMatrixPtr(maths.NewDenseMatrix[float64](n, n)),
		Z:     maths.NewUpdateVectorPtr(maths.NewDenseVector[float64](n)),
		X:     maths.NewDenseVector[float64](n),
		LastX: maths.NewDenseVector[float64](n),
	}
	// 设置引用
	con.MnaUpdateType.MnaType.A = con.MnaUpdateType.A
	con.MnaUpdateType.MnaType.Z = con.MnaUpdateType.Z
	con.MnaUpdateType.MnaType.X = con.MnaUpdateType.X
	return con, nil
}
