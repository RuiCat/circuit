package mna

import (
	"circuit/graph"
	"circuit/mna/mat"
	"circuit/types"
	"fmt"
	"math"
)

// SparseMNA 稀疏矩阵优化节点电压法
type SparseMNA struct {
	*graph.Graph // 图表信息

	// 稀疏矩阵系统
	MatJ *mat.SparseMatrix // 系统导纳矩阵(N×N维)
	MatX []float64         // 未知量向量(节点电压+支路电流)
	MatB []float64         // 右侧激励向量

	// 线性分析备份
	OrigJ  *mat.SparseMatrix // 原始矩阵备份(用于牛顿迭代回滚)
	OrigX  []float64         // 未知量向量备份
	OrigXs []float64         // 未知量向量回退使用
	OrigB  []float64         // 原始右侧向量备份

	// LU分解
	Lu *mat.LU // LU分解器

	// 阻尼Newton-Raphson参数
	DampingFactor    float64 // 阻尼因子
	MinDampingFactor float64 // 最小阻尼因子
	DampingReduction float64 // 阻尼减少因子
}

// NewSparseMNA 创建稀疏矩阵优化的MNA
func NewSparseMNA(graph *graph.Graph) types.Stamp {
	mna := &SparseMNA{
		Graph:            graph,
		DampingFactor:    1.0,
		MinDampingFactor: 0.1,
		DampingReduction: 0.5,
	}

	// 初始化矩阵
	n := mna.NumNodes + mna.NumVoltageSources
	if n <= 0 {
		return nil
	}

	// 创建稀疏矩阵
	mna.MatJ = mat.NewSparseMatrix(n, n)
	mna.MatB = make([]float64, n)
	mna.MatX = make([]float64, n)
	// 构建
	mna.Lu = mat.NewLU(mna.MatJ)
	// 初始化备份
	mna.OrigJ = mat.NewSparseMatrix(n, n)
	mna.OrigB = make([]float64, n)
	mna.OrigX = make([]float64, n)
	mna.OrigXs = make([]float64, n)

	// 重置
	mna.Zero()
	return mna
}

func (mna *SparseMNA) GetJ() []float64 {
	// 返回稠密格式的矩阵数据（用于兼容性）
	dense := make([]float64, mna.MatJ.Rows()*mna.MatJ.Cols())
	for i := 0; i < mna.MatJ.Rows(); i++ {
		for j := 0; j < mna.MatJ.Cols(); j++ {
			dense[i*mna.MatJ.Cols()+j] = mna.MatJ.Get(i, j)
		}
	}
	return dense
}

func (mna *SparseMNA) GetX() []float64 { return mna.MatX }
func (mna *SparseMNA) GetB() []float64 { return mna.MatB }

// SetValue 设置元件的值
func (mna *SparseMNA) SetValue(id types.ElementID, value types.ValueMap) {
	if v, ok := mna.ElementList[id]; ok {
		v.Value.SetValue(value)
	}
}

// SetConverged 标记元件无法收敛
func (mna *SparseMNA) SetConverged() {
	mna.Converged = false
}

// GetGraph 获取底层
func (mna *SparseMNA) GetGraph() *types.ElementGraph {
	return &mna.ElementGraph
}

// GetDampingFactor 得到阻尼因子
func (mna *SparseMNA) GetDampingFactor() float64 {
	return mna.DampingFactor
}

// GetValue 得到元件的值
func (mna *SparseMNA) GetValue(id types.ElementID) (value types.ValueMap) {
	if v, ok := mna.ElementList[id]; ok {
		value = v.Value.GetValue()
	}
	return value
}

// Zero 重置
func (mna *SparseMNA) Zero() {
	// 重置向量
	for i := range mna.MatX {
		mna.MatX[i] = 0
		mna.OrigX[i] = 0
	}

	// 重置矩阵
	mna.MatJ.Clear()
	mna.OrigJ.Clear()

	// 节点重置
	mna.ElementGraph.Zero()
	m := types.ElementID(len(mna.ElementList))
	for i := range m {
		if ele, ok := mna.ElementList[i]; ok {
			ele.Current.Zero()
			ele.Reset()
		}
	}

	// 更新电路
	mna.StampUP()
}

// StampUP 更新电路
func (mna *SparseMNA) StampUP() {
	// 重置矩阵和向量
	mna.MatJ.Clear()
	for i := range mna.MatB {
		mna.MatB[i] = 0
	}

	// 加盖矩阵
	m := len(mna.ElementList)
	for i := range m {
		if ele, ok := mna.ElementList[i]; ok {
			ele.Stamp(mna) // 加盖线性元件贡献
		}
	}

	// 备份矩阵和向量
	mna.MatJ.Copy(mna.OrigJ)
	copy(mna.OrigB, mna.MatB)
	copy(mna.OrigX, mna.MatX)

}

// Solve 求解线性系统
func (mna *SparseMNA) Solve() (ok bool, err error) {
	// 处理备份
	copy(mna.OrigXs, mna.MatX)
	defer func() {
		// 检查状态
		if !ok {
			// 迭代失败回退
			if err == nil {
				copy(mna.MatX, mna.OrigXs)
				return
			}
		}
		// 检查矩阵
		if mna.Debug != nil && mna.Debug.IsDebug() {
			// 更新调试信息
			if math.Mod(mna.MaxTimeStep, mna.Time) == mna.MaxTimeStep {
				mna.Debug.Update(mna)
			}
			// 检查关键节点
			for i := 0; i < mna.NumNodes; i++ {
				if math.Abs(mna.MatJ.Get(i, i)) < 1e-20 {
					ok = false
					err = fmt.Errorf("弱节点%d (diag=%.1e)", i, mna.MatJ.Get(i, i))
				}
			}
		}
	}()

	// 开始迭代
	m := types.ElementID(len(mna.ElementList))
	for i := range m {
		if ele, ok := mna.ElementList[i]; ok {
			ele.StartIteration(mna)
		}
	}

	mna.Iter = 0            // 迭代次数
	mna.DampingFactor = 1.0 // 重置阻尼因子
	prevResidual := 0.0     // 残差

	for ; mna.Iter < mna.MaxIter; mna.Iter++ {
		// 设置为收敛状态
		mna.Converged = true

		// 线性矩阵还原
		mna.OrigJ.Copy(mna.MatJ)
		copy(mna.MatB, mna.OrigB)

		// 计算非线性元件贡献
		for i := range m {
			if ele, ok := mna.ElementList[i]; ok {
				ele.DoStep(mna)
			}
		}
		// 重新分解
		mna.Lu.Decompose(mna.MatJ)
		// 求解
		if err := mna.Lu.SolveReuse(mna.MatB, mna.MatX); err != nil {
			return false, fmt.Errorf("矩阵求解失败: %v", err)
		}
		// mna.MatX = mna.OrigX + α × (mna.MatX  - mna.OrigX) 阻尼实现
		for i := range mna.MatX {
			delta := mna.MatX[i] - mna.OrigX[i]
			mna.MatX[i] = mna.OrigX[i] + mna.DampingFactor*delta
		}

		// 接受结果
		copy(mna.OrigX, mna.MatX)

		// 计算电流
		for i := range m {
			if ele, ok := mna.ElementList[i]; ok {
				ele.CalculateCurrent(mna)
			}
		}
		// 计算残差
		maxResidual := mna.calculateResidual()
		// 阻尼自适应调整
		if mna.Iter > 0 && maxResidual > prevResidual {
			// 残差增大，减少阻尼因子
			mna.DampingFactor = math.Max(mna.DampingFactor*mna.DampingReduction, mna.MinDampingFactor)
		} else if maxResidual < prevResidual*0.5 {
			// 残差快速减小，可以增加阻尼因子
			mna.DampingFactor = math.Min(mna.DampingFactor*1.2, 1.0)
		}
		if maxResidual < mna.ConvergenceTol && mna.Converged {
			break // 已经收敛
		}
		// 振荡检测逻辑保持不变
		if mna.Iter > 0 {
			if maxResidual > prevResidual*1.5 {
				mna.OscillationCount++
			} else if maxResidual < prevResidual*0.5 {
				mna.OscillationCount = 0
			}
			if mna.OscillationCount > mna.OscillationCountMax {
				return false, fmt.Errorf("发散振荡 at iter=%d, res=%.3e", mna.Iter, maxResidual)
			}
		}
		prevResidual = maxResidual
	}
	// 调用结束
	for i := range m {
		if ele, ok := mna.ElementList[i]; ok {
			ele.StepFinished(mna)
		}
	}

	// 迭代失败
	if mna.Iter == mna.MaxIter && prevResidual > mna.ConvergenceTol {
		return false, nil
	}
	return true, nil
}

// 辅助方法：计算残差
func (mna *SparseMNA) calculateResidual() float64 {
	maxResidual := 0.0
	for i := 0; i < len(mna.MatB); i++ {
		sum := 0.0
		cols, vals := mna.MatJ.GetRow(i)
		for j, col := range cols {
			sum += vals[j] * mna.MatX[col]
		}
		res := math.Abs(sum - mna.MatB[i])
		if res > maxResidual {
			maxResidual = res
		}
	}
	return maxResidual
}

// 返回电路节点数量,不包含电压数量
func (mna *SparseMNA) GetNumNodes() int {
	return mna.NumNodes
}

// 返回电路电压数量
func (mna *SparseMNA) GetNumVoltageSources() int {
	return mna.NumVoltageSources
}

// 返回节点电压
func (mna *SparseMNA) GetVoltage(i types.NodeID) float64 {
	switch {
	case i == types.ElementGndNodeID:
		return 0
	case i >= 0 && i < mna.NumNodes:
		return mna.MatX[i]
	}
	return 0
}

// 设置节点电压
func (mna *SparseMNA) SetVoltage(i types.NodeID, v float64) {
	if i > types.ElementGndNodeID && i < mna.NumNodes {
		mna.MatX[i] += v
	}
}

// 在矩阵A的(i,j)位置叠加值
func (mna *SparseMNA) StampMatrix(i, j types.NodeID, v float64) {
	if i > types.ElementGndNodeID && j > types.ElementGndNodeID {
		mna.MatJ.Increment(i, j, v)
	}
}

// 在右侧向量B的i位置叠加值
func (mna *SparseMNA) StampRightSide(i types.NodeID, v float64) {
	if i > types.ElementGndNodeID {
		mna.MatB[i] += v
	}
}

// 加盖电阻元件
func (mna *SparseMNA) StampResistor(n1, n2 types.NodeID, r float64) {
	g := 1.0 / math.Max(r, 1e-12) // 防止除零
	mna.StampConductance(n1, n2, g)
}

// 加盖电导元件
func (mna *SparseMNA) StampConductance(n1, n2 types.NodeID, g float64) {
	mna.StampMatrix(n1, n1, g)
	mna.StampMatrix(n2, n2, g)
	mna.StampMatrix(n1, n2, -g)
	mna.StampMatrix(n2, n1, -g)
}

// 加盖电流源
func (mna *SparseMNA) StampCurrentSource(n1, n2 types.NodeID, i float64) {
	mna.StampRightSide(n1, -i)
	mna.StampRightSide(n2, i)
}

// 加盖电压源
func (mna *SparseMNA) StampVoltageSource(n1, n2 types.NodeID, vs types.VoltageID, v float64) {
	vn := mna.NumNodes + vs
	mna.StampMatrix(vn, n1, -1)
	mna.StampMatrix(vn, n2, 1)
	mna.StampRightSide(vn, v)
	mna.StampMatrix(n1, vn, 1)
	mna.StampMatrix(n2, vn, -1)
}

// 更新电压源值
func (mna *SparseMNA) UpdateVoltageSource(vs types.VoltageID, v float64) {
	mna.StampRightSide(mna.NumNodes+vs, v)
}

// StampVCVS 加盖电压控制电压源
func (mna *SparseMNA) StampVCVS(n1, n2 types.NodeID, vs types.VoltageID, coef float64) {
	vn := mna.NumNodes + vs
	mna.StampMatrix(vn, n1, coef)
	mna.StampMatrix(vn, n2, -coef)
}

// StampVCCurrentSource 加盖电压控制电流源
func (mna *SparseMNA) StampVCCurrentSource(cn1, cn2, vn1, vn2 types.NodeID, gain float64) {
	// 控制电压差
	mna.StampMatrix(cn1, vn1, gain)
	mna.StampMatrix(cn2, vn2, gain)
	mna.StampMatrix(cn1, vn2, -gain)
	mna.StampMatrix(cn2, vn1, -gain)
}

// StampCCCS 加盖电流控制电流源
func (mna *SparseMNA) StampCCCS(n1, n2 types.NodeID, vs types.VoltageID, gain float64) {
	vn := mna.NumNodes + vs
	// 控制电流方程
	mna.StampMatrix(n1, vn, gain)
	mna.StampMatrix(n2, vn, -gain)
}

// String 输出结构
func (mna *SparseMNA) String() string {
	var str string
	// 初始化输出
	str += fmt.Sprintln("节点ID: [元件列表]")
	for id, v := range mna.NodeList {
		str += fmt.Sprintf(" %d: %v\n", id, v)
	}
	str += fmt.Sprintln("元件ID: 元件类型 [元件数据] {引脚索引}")
	m := types.ElementID(len(mna.ElementList))
	for id := range m {
		v := mna.ElementList[id]
		str += fmt.Sprintf(" %d: %s [\n", id, v.Type())
		for k, kv := range v.Value.GetValue() {
			str += fmt.Sprintf("     %v:%v\n", k, kv)
		}
		str += " ] Pin: {\n"
		for k, kv := range v.Nodes {
			str += fmt.Sprintf("     %v->%v\n", k, kv)
		}
		str += " }\n"
	}
	// 周期输出
	str += fmt.Sprintf("------------------------------------------ 时间: %f 步进: %f 步数: %d 迭代: %d 阻尼: %f ----------------------------------------\n", mna.Time, mna.TimeStep, mna.GoodIterations, mna.Iter, mna.DampingFactor)
	str += fmt.Sprintln("系统矩阵: A")
	str += fmt.Sprintln(mna.MatJ.String())
	str += fmt.Sprintln("节点电压: x")
	str += fmt.Sprintf("%v\n", mna.MatX)
	str += fmt.Sprintln("激励向量: b")
	str += fmt.Sprintf("%v\n", mna.MatB)
	str += fmt.Sprintln("系统矩阵(线性贡献): A")
	str += fmt.Sprintln(mna.OrigJ.String())
	str += fmt.Sprintln("激励向量(线性贡献): b")
	str += fmt.Sprintf("%v\n", mna.OrigB)
	str += "\n"
	str += fmt.Sprintln("元件调试信息:")
	for i := range m {
		v := mna.ElementList[i]
		str += fmt.Sprintf("元件 %d 调试信息: [%s]\n", i, v.ElementFace.Debug(mna))
	}
	return str
}
