package mna

import (
	"circuit/types"
	"fmt"
	"math"
)

// Soluv 迭代
type Soluv struct {
	*Matrix // 矩阵
	*Value  // 数据
	ID      types.ElementID
	// 阻尼Newton-Raphson参数
	DampingFactor    float64 // 阻尼因子
	MinDampingFactor float64 // 最小阻尼因子
	DampingReduction float64 // 阻尼减少因子
}

// GetDampingFactor 得到阻尼因子
func (soluv *Soluv) GetDampingFactor() float64 {
	return soluv.DampingFactor
}

// Zero 重置
func (soluv *Soluv) Zero() {
	// 重置向量
	soluv.VecX.Clear()
	soluv.VecB.Clear()
	soluv.Current.Clear()
	// 重置矩阵
	soluv.MatJ.Clear()
	// 节点重置
	soluv.ElementGraph.Zero()
	soluv.Reset()
	// 更新电路
	soluv.StampUP()
}

// StampUP 更新电路
func (soluv *Soluv) StampUP() {
	// 重置矩阵和向量
	soluv.MatJ.Clear() // 会清空底层 OrigJ 的值
	soluv.VecB.Clear() // 会清空底层 Base 的值
	// 加盖矩阵
	soluv.MnaStamp()
	// 备份矩阵和向量
	soluv.MatJ.Update()
	soluv.VecB.Update() // 记录到底层
}

func (soluv *Soluv) GetCurrent(pin int) float64 {
	return soluv.Value.GetPinCurrent(soluv.ID, pin)
}
func (soluv *Soluv) SetCurrent(pin int, i float64) {
	soluv.Value.SetPinCurrent(soluv.ID, pin, i)
}
func (soluv *Soluv) GetValue(n int) float64 {
	return soluv.Value.GetValue(soluv.ID, n)
}
func (soluv *Soluv) SetValue(n int, v float64) {
	soluv.Value.SetValue(soluv.ID, n, v)
}
func (soluv *Soluv) SetValueBase(n int, v float64) {
	soluv.Value.SetValueBase(soluv.ID, n, v)
}
func (soluv *Soluv) MnaStartIteration() {
	m := len(soluv.ElementList)
	for soluv.ID = range m {
		if ele, ok := soluv.ElementList[soluv.ID]; ok {
			ele.StartIteration(soluv)
		}
	}
}
func (soluv *Soluv) Reset() {
	soluv.Value.Reset()
	m := len(soluv.ElementList)
	for soluv.ID = range m {
		if ele, ok := soluv.ElementList[soluv.ID]; ok {
			ele.Reset(soluv)
		}
	}
}
func (soluv *Soluv) MnaStamp() {
	m := len(soluv.ElementList)
	for soluv.ID = range m {
		if ele, ok := soluv.ElementList[soluv.ID]; ok {
			ele.Stamp(soluv)
		}
	}
}
func (soluv *Soluv) MnaDoStep() {
	m := len(soluv.ElementList)
	for soluv.ID = range m {
		if ele, ok := soluv.ElementList[soluv.ID]; ok {
			ele.DoStep(soluv)
		}
	}
}
func (soluv *Soluv) MnaCalculateCurrent() {
	m := len(soluv.ElementList)
	for soluv.ID = range m {
		if ele, ok := soluv.ElementList[soluv.ID]; ok {
			ele.CalculateCurrent(soluv)
		}
	}
}
func (soluv *Soluv) MnaStepFinished() {
	m := len(soluv.ElementList)
	for soluv.ID = range m {
		if ele, ok := soluv.ElementList[soluv.ID]; ok {
			ele.StepFinished(soluv)
		}
	}
}

// Solve 求解线性系统
func (soluv *Soluv) Solve() (ok bool, err error) {
	// 处理备份
	defer func() {
		// 检查状态
		if !ok {
			soluv.VecX.Rollback()
			soluv.Current.Rollback()
			// 迭代失败回退
			if err == nil {
				return
			}
		} else {
			soluv.VecX.Update()
			soluv.Current.Update()
		}
		// 检查矩阵
		if soluv.Debug != nil && soluv.Debug.IsDebug() {
			// 更新调试信息
			if math.Mod(soluv.MaxTimeStep, soluv.Time) == soluv.MaxTimeStep {
				soluv.Debug.Update(soluv)
			}
			// 检查关键节点
			for i := 0; i < soluv.NumNodes; i++ {
				if math.Abs(soluv.MatJ.Get(i, i)) < 1e-20 {
					ok = false
					err = fmt.Errorf("弱节点%d (diag=%.1e)", i, soluv.MatJ.Get(i, i))
				}
			}
		}
	}()
	// 开始迭代
	soluv.MnaStartIteration()
	soluv.Iter = 0            // 迭代次数
	soluv.DampingFactor = 1.0 // 重置阻尼因子
	prevResidual := 0.0       // 残差
	for ; soluv.Iter < soluv.MaxIter; soluv.Iter++ {
		// 设置为收敛状态
		soluv.Converged = true
		// 线性矩阵还原
		soluv.VecB.Rollback()
		soluv.MatJ.Rollback()
		// 非线性元件迭代
		soluv.MnaDoStep()
		// 重新分解
		if err := soluv.Lu.Decompose(soluv.MatJ); err != nil {
			return false, fmt.Errorf("矩阵分解失败: %v", err)
		}
		// 求解
		if err := soluv.Lu.SolveReuse(soluv.VecB, soluv.VecX); err != nil {
			return false, fmt.Errorf("矩阵求解失败: %v", err)
		}
		// mna.MatX = mna.OrigX + α × (mna.MatX  - mna.OrigX) 阻尼实现
		soluv.VecX.ApplyDamping(soluv.DampingFactor)
		// 计算电流
		soluv.MnaCalculateCurrent()
		// 计算残差
		maxResidual := soluv.calculateResidual()
		// 阻尼自适应调整
		if soluv.Iter > 0 && maxResidual > prevResidual {
			// 残差增大，减少阻尼因子
			soluv.DampingFactor = math.Max(soluv.DampingFactor*soluv.DampingReduction, soluv.MinDampingFactor)
		} else if maxResidual < prevResidual*0.5 {
			// 残差快速减小，可以增加阻尼因子
			soluv.DampingFactor = math.Min(soluv.DampingFactor*1.2, 1.0)
		} else {
			if soluv.Iter > 5 && math.Abs(maxResidual-prevResidual) < types.Tolerance {
				soluv.DampingFactor = math.Max(soluv.DampingFactor*0.95, soluv.MinDampingFactor)
			}
		}
		// 检查是否收敛
		if soluv.Converged {
			if maxResidual <= soluv.ConvergenceTol {
				break // 已经收敛
			}
			// 振荡检测逻辑保持不变
			if soluv.Iter > 0 {
				if maxResidual > prevResidual*1.5 {
					soluv.OscillationCount++
				} else if maxResidual < prevResidual*0.5 {
					soluv.OscillationCount = 0
				}
				if soluv.OscillationCount > soluv.OscillationCountMax {
					return false, fmt.Errorf("发散振荡 at iter=%d, res=%.3e", soluv.Iter, maxResidual)
				}
			}
		}
		prevResidual = maxResidual
	}
	// 调用结束
	soluv.MnaStepFinished()
	// 迭代失败
	if soluv.Iter == soluv.MaxIter && prevResidual > soluv.ConvergenceTol {
		return false, nil
	}
	return true, nil
}

// calculateResidual 计算残差
func (soluv *Soluv) calculateResidual() float64 {
	maxResidual := 0.0
	for i := 0; i < soluv.VecB.Length(); i++ {
		sum := 0.0
		cols, vals := soluv.MatJ.GetRow(i)
		for j, col := range cols {
			sum += vals[j] * soluv.VecX.Get(col)
		}
		res := math.Abs(sum - soluv.VecB.Get(i))
		if res > maxResidual {
			maxResidual = res
		}
	}
	return maxResidual
}

/*-------------------------------------------------------------------------------------------------------------------------------------------------*/

func (soluv *Soluv) GetJ() []float64 {
	// 返回稠密格式的矩阵数据
	dense := make([]float64, soluv.MatJ.Rows()*soluv.MatJ.Cols())
	for i := 0; i < soluv.MatJ.Rows(); i++ {
		for j := 0; j < soluv.MatJ.Cols(); j++ {
			dense[i*soluv.MatJ.Cols()+j] = soluv.MatJ.Get(i, j)
		}
	}
	return dense
}
func (soluv *Soluv) GetC() []float64 { return soluv.Current.ToDense() }
func (soluv *Soluv) GetX() []float64 { return soluv.VecX.ToDense() }
func (soluv *Soluv) GetB() []float64 { return soluv.VecB.ToDense() }

// String 输出结构
func (soluv *Soluv) String() string {
	var str string
	// 初始化输出
	m := types.ElementID(len(soluv.ElementList))
	if soluv.GoodIterations == 0 {
		str += fmt.Sprintln("节点ID: [元件列表]")
		for id, v := range soluv.NodeList {
			str += fmt.Sprintf(" %d: %v\n", id, v)
		}
		str += fmt.Sprintln("元件ID: 元件类型 [元件数据] {引脚索引}")
		for id := range m {
			v := soluv.ElementList[id]
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
	str += fmt.Sprintf("------------------------------------------ 时间: %f 步进: %f 步数: %d 迭代: %d 阻尼: %f ----------------------------------------\n", soluv.Time, soluv.TimeStep, soluv.GoodIterations, soluv.Iter, soluv.DampingFactor)
	str += fmt.Sprintln("系统矩阵: A")
	str += fmt.Sprintln(soluv.MatJ.String())
	str += fmt.Sprintln("节点电压: x")
	str += fmt.Sprintf("%v\n", soluv.VecX)
	str += fmt.Sprintln("激励向量: b")
	str += fmt.Sprintf("%v\n", soluv.VecB)
	str += fmt.Sprintln("系统矩阵(线性贡献): A")
	str += fmt.Sprintln(soluv.OrigJ.String())
	str += "\n"
	str += fmt.Sprintln("元件调试信息:")
	for i := range m {
		v := soluv.ElementList[i]
		str += fmt.Sprintf("元件 %d 调试信息: [%s]\n", i, v.ElementFace.Debug(soluv))
	}
	return str
}
