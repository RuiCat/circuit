package time

import (
	"circuit/element"
	"circuit/maths"
	"circuit/mna"
	"fmt"
	"math"
)

// TransientSimulation 执行瞬态仿真，使用元件回调函数和LU求解器实现迭代计算
// 参数：
//
//	con: 包含电路所有信息的上下文
//	call: 每步成功后的回调函数，接收节点电压数组
func TransientSimulation(con *element.Context, call func([]float64)) error {
	// 初始化阶段：获取电路规模并创建求解器
	nodesNum, voltageSourcesNum := con.GetNodeNum(), con.GetVoltageSourcesNum()
	// 创建LU分解器
	luSolver, err := maths.NewLU[float64](nodesNum + voltageSourcesNum)
	if err != nil {
		return fmt.Errorf("LU分解器初始化失败: %v", err)
	}
	// 电压数组用于存储每步的节点电压结果
	voltages := make([]float64, nodesNum)
	// 标记是否需要重新加盖线性元件（步长变化或首次迭代）
	needLinearStamp := true
	// 初始化所有元件状态
	con.CallMark(element.MarkReset)
	// 主时间迭代循环
	con.ResetTimeStepCount()
	for !con.IsSimulationFinished() {
		// 检查是否超过最大时间步数
		if !con.IncrementTimeStepCount() {
			return fmt.Errorf("达到最大时间步数限制 %f，仿真可能陷入无限循环", con.Time.MaxTimeStep())
		}
		// 重置当前时间步的非线性迭代状态
		con.ResetNonlinearIter()
		// 回滚到线性状态（丢弃非线性迭代的修改）
		con.A.Rollback()
		con.Z.Rollback()
		// 线性元件处理
		if needLinearStamp {
			// 需要重新加盖线性元件
			needLinearStamp = false
			// 还原求解状态为上一次收敛结果
			con.CallMark(element.MarkRollbackElements)
			// 清空矩阵和向量
			con.GetA().Base().Zero()
			con.GetZ().Base().Zero()
			// 通知元件开始新迭代
			con.CallMark(element.MarkStartIteration)
			// 加盖线性元件贡献
			con.CallMark(element.MarkStamp)
			// 保存线性状态（用于后续回滚）
			con.Update()
		} else {
			// 重用已有的线性贡献，仅通知元件开始新迭代
			con.CallMark(element.MarkStartIteration)
		}
		// 非线性迭代（牛顿-拉夫逊法）
		newtonConverged := false
		for con.NextNonlinearIter() {
			// 遍历所有非线性元件，计算其贡献
			con.CallMark(element.MarkDoStep)
			// 求解MNA方程
			if err := luSolver.Decompose(con.GetA()); err != nil {
				return fmt.Errorf("矩阵分解失败（时间=%.6e，步长=%.6e）: %v", con.CurrentTime(), con.CurrentStep(), err)
			}
			// 执行前向替换和后向替换
			if err := luSolver.SolveReuse(con.GetZ(), con.GetX()); err != nil {
				return fmt.Errorf("方程求解失败（时间=%.6e）: %v", con.CurrentTime(), err)
			}
			// 计算残差并检查收敛
			if err := con.CalculateMNAResidual(con); err != nil {
				return fmt.Errorf("残差计算失败: %v", err)
			}
			// 检查全局收敛条件
			con.CheckResidualConvergence()
			if con.IsConverged() {
				newtonConverged = true
				break // 牛顿迭代收敛，退出内层循环
			}
			// 重置元件迭代计数器
			con.ResetElemIter()
			// 开始次级迭代循环
			for con.NextElemIter() {
				// 将矩阵回滚到加盖线性元件之后的状态
				con.A.Rollback()
				con.Z.Rollback()
				// 重新计算所有非线性元件的贡献
				con.CallMark(element.MarkDoStep)
				// 重新求解MNA方程
				if err := luSolver.Decompose(con.GetA()); err != nil {
					return fmt.Errorf("元件迭代中矩阵分解失败: %v", err)
				}
				if err := luSolver.SolveReuse(con.GetZ(), con.GetX()); err != nil {
					return fmt.Errorf("元件迭代中方程求解失败: %v", err)
				}
				// 重新计算残差并检查收敛
				if err := con.CalculateMNAResidual(con); err != nil {
					return fmt.Errorf("残差计算失败: %v", err)
				}
				con.CheckResidualConvergence()
				// 如果整个系统现在已经收敛，则更新状态并返回
				if con.IsConverged() {
					con.CallMark(element.MarkUpdateElements)
					break
				}
			}
			// 如果循环结束，意味着即使经过额外的迭代也未能收敛
			if con.IsElemIterExhausted() {
				return fmt.Errorf("经过 %d 次元件迭代后仍未收敛", con.MaxElemIter())
			}
		}
		// 检查牛顿迭代是否成功收敛
		if !newtonConverged {
			return fmt.Errorf("牛顿迭代在时间 %.6e 未收敛（达到最大迭代次数 %d）", con.CurrentTime(), con.MaxNonlinearIter())
		}
		// 后处理：计算电流和更新元件状态
		con.CallMark(element.MarkCalculateCurrent)
		con.CallMark(element.MarkStepFinished)
		// 提取并验证节点电压
		if !extractAndValidateVoltages(con, nodesNum, voltages) {
			return fmt.Errorf("检测到无效电压值（NaN/Inf）在时间 %.6e，停止仿真", con.CurrentTime())
		}
		// 更新残差历史并调整步长
		con.UpdateResidualHistory()
		if con.ShouldAdjustStepSize() {
			needLinearStamp = true // 步长变化较大，需要重新加盖线性元件
		}
		// 检查残差是否可接受并推进时间
		if con.IsResidualConverged() {
			// 残差可接受，推进时间
			if err := advanceTimeSimple(con); err != nil {
				return fmt.Errorf("时间推进失败: %v", err)
			}
			// 接受求解状态
			con.CallMark(element.MarkUpdateElements)
			// 重置计数
			con.ResetTimeStepCount()
			// 调用用户回调函数
			call(voltages)
		} else {
			// 残差不可接受，减小步长并重新计算当前步
			needLinearStamp = true
			continue
		}
	}
	return nil
}

// extractAndValidateVoltages 从MNA求解器提取节点电压并验证有效性
func extractAndValidateVoltages(mnaSolver mna.Mna, nodesNum int, voltages []float64) bool {
	allValid := true
	for i := mna.NodeID(0); i < mna.NodeID(nodesNum); i++ {
		voltages[i] = mnaSolver.GetNodeVoltage(i)
		if math.IsNaN(voltages[i]) || math.IsInf(voltages[i], 0) {
			allValid = false
			break
		}
	}
	return allValid
}

// advanceTimeSimple 简单时间推进方法
func advanceTimeSimple(con *element.Context) error {
	// 检查仿真状态
	if con.IsSimulationFinished() {
		return nil
	}
	// 获取当前时间和步长
	currentTime := con.CurrentTime()
	timeStep := con.CurrentStep()
	targetTime := con.TargetTime()
	// 计算下一个时间
	nextTime := currentTime + timeStep
	if nextTime > targetTime {
		nextTime = targetTime
	}
	// 更新仿真时间
	con.SetTime(nextTime)
	return nil
}
