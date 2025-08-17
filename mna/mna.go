package mna

import (
	"circuit/graph"
	"circuit/types"
	"fmt"
	"math"

	"gonum.org/v1/gonum/mat"
)

// MMA 修正节点电压法实现
type MNA struct {
	*graph.Graph // 图表信息
	// 核心矩阵系统
	MatJ *mat.Dense    // 系统导纳矩阵(N×N维)
	MatX *mat.VecDense // 未知量向量(节点电压+支路电流)
	MatB *mat.VecDense // 右侧激励向量
	// 线性分析备份
	OrigJ *mat.Dense    // 原始矩阵备份(用于牛顿迭代回滚)
	OrigX *mat.VecDense // 未知量向量备份
	OrigB *mat.VecDense // 原始右侧向量备份
	// 因式分解
	Lu mat.LU // 因式分解
	// 阻尼Newton-Raphson参数
	DampingFactor    float64 // 阻尼因子
	MinDampingFactor float64 // 最小阻尼因子
	DampingReduction float64 // 阻尼减少因子
}

// NewMNA 创建
func NewMNA(graph *graph.Graph) (mna *MNA) {
	mna = &MNA{
		Graph:            graph,
		DampingFactor:    1.0,
		MinDampingFactor: 0.1,
		DampingReduction: 0.5,
	}
	// 初始化矩阵
	n := mna.NumNodes + mna.NumVoltageSources
	mna.MatJ = mat.NewDense(n, n, nil) // 初始化系统矩阵
	mna.MatB = mat.NewVecDense(n, nil) // 初始化激励向量
	mna.MatX = mat.NewVecDense(n, nil) // 初始化解向量
	// 初始化备份
	mna.OrigJ = mat.NewDense(n, n, nil)
	mna.OrigX = mat.NewVecDense(n, nil)
	mna.OrigB = mat.NewVecDense(n, nil)
	// 重置
	mna.Zero()
	return mna
}

// SetValue 设置元件的值
func (mna *MNA) SetValue(id types.ElementID, value types.ValueMap) {
	if v, ok := mna.ElementList[id]; ok {
		v.Value.SetValue(value)
	}
}

// GetValue 得到元件的值
func (mna *MNA) GetValue(id types.ElementID) (value types.ValueMap) {
	if v, ok := mna.ElementList[id]; ok {
		value = v.Value.GetValue()
	}
	return value
}

// Zero 重置
func (mna *MNA) Zero() {
	// 重置矩阵
	mna.MatX.Zero()
	mna.OrigX.Zero()
	mna.ElementGraph.Zero()
	// 节点重置
	for _, ele := range mna.ElementList {
		ele.Reset()
	}
	// 更新电路
	mna.StampUP()
}

// stampUP 更新电路
func (mna *MNA) StampUP() {
	// 重置矩阵
	mna.MatJ.Zero()
	mna.MatB.Zero()
	mna.OrigJ.Zero()
	mna.OrigB.Zero()
	mna.OrigX.Zero()
	// 加盖矩阵
	for _, ele := range mna.ElementList {
		ele.Stamp(mna) // 加盖线性元件贡献
	}
	// 性矩阵备份
	mna.OrigBackUP()
	mna.OrigX.CopyVec(mna.MatX)
}

// Orig 线性矩阵备份
func (mna *MNA) OrigBackUP() {
	mna.OrigJ.Copy(mna.MatJ)
	mna.OrigB.CopyVec(mna.MatB)
}

// OrigRestore 线性贡献矩阵还原
func (mna *MNA) OrigRestore() {
	mna.MatJ.Copy(mna.OrigJ)
	mna.MatB.CopyVec(mna.OrigB)
}

// 修改Solve方法，添加阻尼控制
func (mna *MNA) Solve() (ok bool, err error) {
	// 处理备份
	defer func() {
		if !ok {
			// 失败退回结果
			mna.MatX.CopyVec(mna.OrigX)
		}
	}()
	// 开始迭代
	for _, ele := range mna.ElementList {
		ele.StartIteration(mna)
	}
	mna.Iter = 0            // 迭代次数
	mna.DampingFactor = 1.0 // 重置阻尼因子
	prevResidual := 0.0     // 残差
	for ; mna.Iter < mna.MaxIter; mna.Iter++ {
		// 线性矩阵还原
		mna.OrigRestore()
		// 算计解
		for _, ele := range mna.ElementList {
			ele.DoStep(mna)
		}
		// 标准Newton-Raphson求解得到的完整步长解
		mna.Lu.Factorize(mna.MatJ)
		if err := mna.Lu.SolveVecTo(mna.MatX, false, mna.MatB); err != nil {
			return false, fmt.Errorf("矩阵求解失败: %v", err)
		}
		// mna.MatX = mna.OrigX + α × (mna.MatX - mna.OrigX) 阻尼实现
		mna.MatX.SubVec(mna.MatX, mna.OrigX)           // Δx = x_newton - x_old
		mna.MatX.ScaleVec(mna.DampingFactor, mna.MatX) // Δx = α × Δx
		mna.MatX.AddVec(mna.MatX, mna.OrigX)           // x_final = x_old + α × Δx
		mna.OrigX.CopyVec(mna.MatX)                    // 接受结果
		// 计算电流
		for _, ele := range mna.ElementList {
			ele.CalculateCurrent(mna)
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
		if maxResidual < mna.ConvergenceTol {
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
	for _, ele := range mna.ElementList {
		ele.StepFinished(mna)
	}
	// 检查矩阵
	if mna.Debug {
		// 检查电压源约束行
		for i := mna.NumNodes + 1; i < mna.MatJ.RawMatrix().Rows; i++ {
			if math.Abs(mna.MatJ.At(i, i)) < 1e-12 {
				return false, fmt.Errorf("电压源约束失效 at row%d", i)
			}
		}
		// 检查关键节点
		for i := 0; i < mna.NumNodes; i++ {
			if math.Abs(mna.MatJ.At(i, i)) < 1e-9 {
				return false, fmt.Errorf("弱节点%d (diag=%.1e)", i, mna.MatJ.At(i, i))
			}
		}
	}
	// 迭代失败
	if mna.Iter == mna.MaxIter && prevResidual > mna.ConvergenceTol {
		return false, nil
	}
	return true, nil
}

// 辅助方法：计算残差
func (mna *MNA) calculateResidual() float64 {
	maxResidual := 0.0
	for i := 0; i < mna.MatB.Len(); i++ {
		row := mna.MatJ.RowView(i)
		sum := 0.0
		for j := 0; j < row.Len(); j++ {
			sum += row.AtVec(j) * mna.MatX.AtVec(j)
		}
		res := math.Abs(sum - mna.MatB.AtVec(i))
		if res > maxResidual {
			maxResidual = res
		}
	}
	return maxResidual
}

// 返回电路节点数量,不包含电压数量
func (mna *MNA) GetNumNodes() int {
	return mna.NumNodes
}

// 返回电路电压数量
func (mna *MNA) GetNumVoltageSources() int {
	return mna.NumVoltageSources
}

// 返回节点电压
func (mna *MNA) GetVoltage(i types.NodeID) float64 {
	if i > types.ElementGndNodeID {
		return mna.MatX.AtVec(i)
	}
	return 0
}

// 设置节点电压
func (mna *MNA) SetVoltage(i types.NodeID, v float64) error {
	if i > types.ElementGndNodeID {
		mna.MatX.SetVec(i, mna.MatX.AtVec(i)+v)
	} else {
		return fmt.Errorf("设置节点电压 %b:%b 错误", i, v)
	}
	return nil
}

// 在矩阵A的(i,j)位置叠加值
func (mna *MNA) StampMatrix(i, j types.NodeID, v float64) error {
	if i > types.ElementGndNodeID && j > types.ElementGndNodeID {
		mna.MatJ.Set(i, j, mna.MatJ.At(i, j)+v)
	} else {
		return fmt.Errorf("矩阵加盖失败 %b:%b -> %b", i, j, v)
	}
	return nil
}

// 在右侧向量B的i位置叠加值
func (mna *MNA) StampRightSide(i types.VoltageID, v float64) error {
	if i > types.ElementGndNodeID {
		mna.MatB.SetVec(i, mna.MatB.AtVec(i)+v)
	} else {
		return fmt.Errorf("设置激励向量 %b:%b 错误", i, v)
	}
	return nil
}

// 加盖电阻元件
func (mna *MNA) StampResistor(n1, n2 types.NodeID, r float64) error {
	g := 1.0 / math.Max(r, 1e-12) // 防止除零
	return mna.StampConductance(n1, n2, g)
}

// 加盖电导元件
func (mna *MNA) StampConductance(n1, n2 types.NodeID, g float64) error {
	if n1 > types.ElementGndNodeID {
		if err := mna.StampMatrix(n1, n1, g); err != nil {
			return err
		}
	}
	if n2 > types.ElementGndNodeID {
		if err := mna.StampMatrix(n2, n2, g); err != nil {
			return err
		}
	}
	if n1 > types.ElementGndNodeID && n2 > types.ElementGndNodeID {
		if err := mna.StampMatrix(n1, n2, -g); err != nil {
			return err
		}
		if err := mna.StampMatrix(n2, n1, -g); err != nil {
			return err
		}
	}
	return nil
}

// 加盖电流源
func (mna *MNA) StampCurrentSource(n1, n2 types.NodeID, i float64) error {
	if n1 > types.ElementGndNodeID {
		if err := mna.StampRightSide(n1, -i); err != nil {
			return err
		}
	}
	if n2 > types.ElementGndNodeID {
		if err := mna.StampRightSide(n2, i); err != nil {
			return err
		}
	}
	return nil
}

// 加盖电压源
func (mna *MNA) StampVoltageSource(n1, n2 types.NodeID, vs types.VoltageID, v float64) error {
	vsRow := mna.NumNodes + vs
	if n1 > types.ElementGndNodeID {
		if err := mna.StampMatrix(vsRow, n1, 1); err != nil { // 约束方程
			return err
		}
		if err := mna.StampMatrix(n1, vsRow, 1); err != nil { // 电流变量
			return err
		}
	}
	if n2 > types.ElementGndNodeID {
		if err := mna.StampMatrix(vsRow, n2, -1); err != nil { // 约束方程
			return err
		}
		if err := mna.StampMatrix(n2, vsRow, -1); err != nil { // 电流变量
			return err
		}
	}
	return mna.StampRightSide(vsRow, v)
}

// 更新电压源值
func (mna *MNA) UpdateVoltageSource(n1, n2 types.NodeID, vs types.VoltageID, v float64) error {
	return mna.StampRightSide(mna.NumNodes+vs, v)
}

// StampVCVS 加盖电压控制电压源
func (mna *MNA) StampVCVS(n1, n2 types.NodeID, coef float64, vs types.VoltageID) error {
	vsRow := mna.NumNodes + vs
	// 控制电压方程
	if n1 > types.ElementGndNodeID {
		if err := mna.StampMatrix(vsRow, n1, coef); err != nil {
			return err
		}
	}
	if n2 > types.ElementGndNodeID {
		if err := mna.StampMatrix(vsRow, n2, -coef); err != nil {
			return err
		}
	}
	// 受控电压源约束
	return mna.StampMatrix(vsRow, vs, -1)
}

// StampVCCurrentSource 加盖电压控制电流源
func (mna *MNA) StampVCCurrentSource(cn1, cn2 types.NodeID, vn1, vn2 types.VoltageID, gain float64) error {
	// 控制电压差
	if cn1 > types.ElementGndNodeID && vn1 > types.ElementGndNodeID {
		if err := mna.StampMatrix(vn1, cn1, gain); err != nil {
			return err
		}
	}
	if cn1 > types.ElementGndNodeID && vn2 > types.ElementGndNodeID {
		if err := mna.StampMatrix(vn2, cn1, -gain); err != nil {
			return err
		}
	}
	if cn2 > types.ElementGndNodeID && vn1 > types.ElementGndNodeID {
		if err := mna.StampMatrix(vn1, cn2, -gain); err != nil {
			return err
		}
	}
	if cn2 > types.ElementGndNodeID && vn2 > types.ElementGndNodeID {
		if err := mna.StampMatrix(vn2, cn2, gain); err != nil {
			return err
		}
	}
	return nil
}

// StampCCCS 加盖电流控制电流源
func (mna *MNA) StampCCCS(n1, n2 types.NodeID, vs types.VoltageID, gain float64) error {
	vsRow := mna.NumNodes + vs
	// 控制电流方程
	if n1 > types.ElementGndNodeID {
		if err := mna.StampMatrix(n1, vsRow, gain); err != nil {
			return err
		}
	}
	if n2 > types.ElementGndNodeID {
		if err := mna.StampMatrix(n2, vsRow, -gain); err != nil {
			return err
		}
	}
	return nil
}

// String 输出结构
func (mna *MNA) DebugMNA() {
	if !mna.Debug {
		return
	}
	var str string
	// 初始化输出
	if mna.GoodIterations == 0 {
		str += fmt.Sprintln("节点ID: [元件列表]")
		for id, v := range mna.NodeList {
			str += fmt.Sprintf(" %d: %v\n", id, v)
		}
		str += fmt.Sprintln("元件ID: 元件类型 [元件数据] {引脚索引}")
		for id, v := range mna.ElementList {
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
	}
	// 周期输出
	str += fmt.Sprintf("------------------------------------------ 时间: %f 步进: %f 步数: %d 迭代: %d 阻尼: %f ----------------------------------------\n", mna.Time, mna.TimeStep, mna.GoodIterations, mna.Iter, mna.DampingFactor)
	str += fmt.Sprintln("系统矩阵: A")
	str += fmt.Sprintln(mat.Formatted(mna.MatJ))
	str += fmt.Sprintln("节点电压: x")
	str += fmt.Sprintln(mat.Formatted(mna.MatX))
	str += fmt.Sprintln("激励向量: b")
	str += fmt.Sprintln(mat.Formatted(mna.MatB))
	if mna.GoodIterations == 0 {
		str += fmt.Sprintln("系统矩阵(线性贡献): A")
		str += fmt.Sprintln(mat.Formatted(mna.OrigJ))
		str += fmt.Sprintln("激励向量(线性贡献): b")
		str += fmt.Sprint(mat.Formatted(mna.OrigB))
		str += "\n"
	}
	str += fmt.Sprintln("元件调试信息:")
	for i := types.ElementID(0); i < types.ElementID(mna.NumNodes+1); i++ {
		str += fmt.Sprintf("元件 %d 调试信息: [%s]\n", i, mna.ElementList[i].ElementFace.Debug(mna))
	}
	fmt.Println(str)
}
