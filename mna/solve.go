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
	MmxDampingFactor float64 // 最大尼因子
}

// GetDampingFactor 得到阻尼因子
func (soluv *Soluv) GetDampingFactor() float64 {
	return soluv.DampingFactor
}

// Zero 重置
func (soluv *Soluv) Zero() {
	// 重置向量
	soluv.VecX[0].Clear()
	soluv.VecX[1].Clear()
	soluv.VecX[2].Clear()
	soluv.VecB.Clear()
	soluv.Current.Clear()
	// 重置矩阵
	soluv.MatJ.Clear()
	// 节点重置
	soluv.ElementGraph.Zero()
	soluv.Reset()
	// 更新电路
	soluv.StampUP()
	// 拷贝求解值
	soluv.VecX[0].Copy(soluv.VecX[2])
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
	soluv.VecB.Update()
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
			ele.Update()
		}
	}
}
func (soluv *Soluv) MnaStamp() {
	m := len(soluv.ElementList)
	for soluv.ID = range m {
		if ele, ok := soluv.ElementList[soluv.ID]; ok {
			ele.Stamp(soluv)
			ele.Update()
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
func (soluv *Soluv) MnaStepFinished(is bool) {
	m := len(soluv.ElementList)
	// 检查状态
	if is {
		// 计算电流
		soluv.MnaCalculateCurrent()
		for soluv.ID = range m {
			if ele, ok := soluv.ElementList[soluv.ID]; ok {
				ele.StepFinished(soluv)
				ele.Update()
			}
		}
		soluv.VecX[0].Copy(soluv.VecX[2])
		soluv.Current.Update()
	} else {
		for soluv.ID = range m {
			if ele, ok := soluv.ElementList[soluv.ID]; ok {
				ele.StepFinished(soluv)
				ele.Rollback()
			}
		}
		soluv.VecX[2].Copy(soluv.VecX[0])
		soluv.Current.Rollback()
	}
}

// Solve 求解线性系统
func (soluv *Soluv) Solve() (ok bool, err error) {
	// 处理备份
	defer func() {
		// 调用结束
		soluv.MnaStepFinished(ok)
		// 检查矩阵
		if soluv.Debug != nil && soluv.Debug.IsDebug() && ok {
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
	soluv.Iter = 0             // 迭代次数
	soluv.OscillationCount = 0 // 重置震荡次数
	for ; soluv.Iter < soluv.MaxIter; soluv.Iter++ {
		// 重置
		soluv.Converged = true
		soluv.VecB.Rollback()
		soluv.MatJ.Rollback()
		soluv.VecX[0].Copy(soluv.VecX[1])
		// 非线性迭代
		soluv.MnaDoStep()
		// 求解
		if err := soluv.Lu.Decompose(soluv.MatJ); err != nil {
			return false, fmt.Errorf("矩阵分解失败: %v", err)
		}
		if err := soluv.Lu.SolveReuse(soluv.VecB, soluv.VecX[0]); err != nil {
			return false, fmt.Errorf("矩阵求解失败: %v", err)
		}
		// 处理收敛
		if soluv.Converged {
			return true, nil
		}
		// 更新阻尼状态
		if soluv.Iter > 0 {
			maxVoltageChange := 0.0
			for i := 0; i < soluv.VecX[1].Length(); i++ {
				change := math.Abs(soluv.VecX[0].Get(i) - soluv.VecX[1].Get(i))
				if change > maxVoltageChange {
					maxVoltageChange = change
				}
			}
			// 阻尼自适应调整
			switch {
			case maxVoltageChange > soluv.ConvergenceTol*10.0:
				soluv.DampingFactor = math.Max(soluv.MinDampingFactor, soluv.DampingFactor*0.1)
				soluv.OscillationCount++
			case maxVoltageChange > soluv.ConvergenceTol*2.0:
				soluv.DampingFactor = math.Max(soluv.MinDampingFactor, soluv.DampingFactor*0.5)
				soluv.OscillationCount++
			case maxVoltageChange > soluv.ConvergenceTol*1.5:
				soluv.OscillationCount++
				soluv.DampingFactor = math.Max(soluv.MinDampingFactor, soluv.DampingFactor*0.8)
			case maxVoltageChange > soluv.ConvergenceTol:
				soluv.OscillationCount++
				soluv.DampingFactor = math.Max(soluv.MinDampingFactor, soluv.DampingFactor*0.9)
			case maxVoltageChange < soluv.ConvergenceTol*0.5:
				soluv.OscillationCount = 0
				soluv.DampingFactor = math.Min(soluv.MmxDampingFactor, soluv.DampingFactor*1.2)
			case maxVoltageChange < soluv.ConvergenceTol:
				soluv.OscillationCount = 0
				soluv.DampingFactor = math.Min(soluv.MmxDampingFactor, soluv.DampingFactor*1.1)
			case soluv.OscillationCount > soluv.OscillationCountMax:
				return false, fmt.Errorf("发散振荡 at iter=%d, res=%.3e", soluv.Iter, maxVoltageChange)
			}
			// 收敛检查
			if maxVoltageChange < soluv.ConvergenceTol {
				return true, nil
			}
			// 计算阻尼
			for i := 0; i < soluv.VecX[1].Length(); i++ {
				orig := soluv.VecX[1].Get(i)
				delta := soluv.VecX[0].Get(i) - orig
				soluv.VecX[0].Set(i, orig+soluv.DampingFactor*delta)
			}
		}
	}
	return false, nil
}

/*-------------------------------------------------------------------------------------------------------------------------------------------------*/

func (soluv *Soluv) GetJ() []float64 { return soluv.MatJ.ToDense() }
func (soluv *Soluv) GetC() []float64 { return soluv.Current.ToDense() }
func (soluv *Soluv) GetX() []float64 { return soluv.VecX[0].ToDense() }
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
