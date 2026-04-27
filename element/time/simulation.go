package time

import (
	"circuit/element"
	"circuit/maths"
	"circuit/mna"
	"fmt"
	"math"
)

// TransientSimulation 执行瞬态仿真，使用元件回调函数和LU求解器实现迭代计算。
// 参数：
//
//	con: 包含电路所有信息的上下文
//	call: 每步成功后的回调函数，接收节点电压数组
//
// 仿真流程：
//  1. 初始化阶段：创建LU求解器，重置所有元件
//  2. 主时间迭代循环（在每个时间步内）：
//     a. 对纯DC电路（无储能元件），用3阶Adams-Bashford预测提供Newton初始猜测
//     b. 线性元件加盖：Stamp线性贡献到MNA矩阵
//     c. 非线性迭代（Newton-Raphson法）：
//     - DoStep计算元件非线性贡献
//     - LU分解 + 前代/回代求解
//     - 残差计算与收敛检查
//     - 如不收敛进入元件次级迭代循环
//     d. 后处理：计算电流、更新元件状态、提取节点电压
//     e. 纯DC电路：估计局部截断误差(LTE)并自适应调整步长
//     f. 推进仿真时间，调用用户回调
func TransientSimulation(con *element.Context, call func([]float64)) error {
	// 初始化阶段：获取电路规模并创建求解器
	nodesNum, voltageSourcesNum := con.GetNodeNum(), con.GetVoltageSourcesNum()
	// 创建LU分解器
	systemSize := nodesNum + voltageSourcesNum

	var luSolver maths.LU[float64]
	var err error
	if con.ParallelOpts != nil && con.ParallelOpts.StampWorkers > 1 {
		luSolver, err = maths.NewParallelLU[float64](systemSize, con.ParallelOpts.StampWorkers)
	} else {
		luSolver, err = maths.NewLU[float64](systemSize)
	}
	if err != nil {
		return fmt.Errorf("LU分解器初始化失败: %v", err)
	}
	// 电压数组用于存储每步的节点电压结果
	voltages := make([]float64, nodesNum)
	// 标记是否需要重新加盖线性元件（步长变化或首次迭代）
	needLinearStamp := true
	// 初始化所有元件状态
	con.CallMark(element.MarkReset)

	tm := con.Time.(*TimeMNA)
	successfulSteps := 0

	con.ResetTimeStepCount()
	for !con.IsSimulationFinished() {
		// 检查是否超过最大时间步数
		if !con.IncrementTimeStepCount() {
			return fmt.Errorf("达到最大时间步数限制 %f，仿真可能陷入无限循环", con.Time.MaxTimeStep())
		}

		// 预测阶段：对纯 DC 电路，用 Adams-Bashford 预测提供 Newton 初始猜测
		// 并通过 LTE 估计增长步长；含储能元件的电路使用固定小步长。
		if !con.HasReactiveElements() && successfulSteps >= 3 {
			if err := tm.Predict(); err == nil {
				for i := range systemSize {
					con.GetX().Set(i, tm.predState[i])
				}
			}
		}

		con.ResetNonlinearIter()
		// 回滚到线性状态（丢弃非线性迭代的修改）
		con.A.Rollback()
		con.Z.Rollback()
		// 还原解向量X和元件内部状态到上一次收敛结果
		con.CallMark(element.MarkRollbackElements)
		// 触发点步长预调整：如果步长会越过最近的触发点，截断步长
		for _, tr := range con.Triggers() {
			if !tr.Triggered && tr.Time > con.CurrentTime() {
				remaining := tr.Time - con.CurrentTime()
				if remaining > 0 && remaining < con.CurrentStep()*0.999 {
					con.SetTimeStep(remaining)
					needLinearStamp = true
					break
				}
			}
		}
		// 线性元件处理
		if needLinearStamp {
			// 需要重新加盖线性元件
			needLinearStamp = false
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
		newtonIterCount := 0
		for con.NextNonlinearIter() {
			newtonIterCount++
			// 回滚到线性基准状态（MarkStamp），避免上一轮DoStep的累积
			con.A.Rollback()
			con.Z.Rollback()
			// 遍历所有非线性元件，计算其贡献
			if err := doStep(con); err != nil {
				return err
			}
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
			// 最少2轮迭代：防止第1轮线性精确求解后残差为0导致的假收敛
			con.CheckResidualConvergence()
			if con.IsConverged() && newtonIterCount >= 2 {
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
				if err := doStep(con); err != nil {
					return err
				}
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
					newtonConverged = true
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

		// 积分误差估计与步长自适应
		// 仅对纯 DC 电路（无储能元件）执行步长调整，快速跳过稳定状态。
		// 含储能元件的电路使用固定步长以保证数值稳定性。
		if !con.HasReactiveElements() {
			if successfulSteps >= 3 {
				tm.EstimateLTE()
				if err := tm.AdjustStepSize(); err != nil {
					return fmt.Errorf("步长调整失败: %v", err)
				}
			} else {
				deriv := con.ComputeStateDerivative()
				convergedState := make([]float64, systemSize)
				for i := range systemSize {
					convergedState[i] = con.GetX().Get(i)
				}
				_ = bootstrapHistory(tm, convergedState, deriv, successfulSteps)
			}
		}

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
			if !con.HasReactiveElements() && successfulSteps >= 3 {
				tm.UpdateHistory()
			}
			successfulSteps++

			call(voltages)
		} else {
			// 残差不可接受，减小步长并重新计算当前步
			needLinearStamp = true
			continue
		}
	}

	// 将X恢复到指向最后一次收敛解。
	// 循环内的最后一次MarkUpdateElements调用了UpdateX，它会交换
	// X和LastX，使得MnaType.X指向"待求解"缓冲区。再次交换
	// 使MnaType.X指向收敛解缓冲区。
	con.CallMark(element.MarkUpdateElements)
	return nil
}

// bootstrapHistory 在前 3 个成功步累积历史数据，为 3 阶 Adams 方法初始化。
// stepIdx: 0, 1, 2 分别对应 historyStates[2], [1], [0]。
// 仅在纯 DC 电路中使用，含储能元件的电路不启用历史记录。
func bootstrapHistory(tm *TimeMNA, state, deriv []float64, stepIdx int) error {
	if tm.historyInited {
		return nil
	}

	if tm.predState == nil {
		tm.predState = make([]float64, len(state))
		tm.predDer = make([]float64, len(state))
		tm.corrState = make([]float64, len(state))
		tm.corrDer = make([]float64, len(state))
	}

	switch stepIdx {
	case 0:
		tm.historyStates[2] = make([]float64, len(state))
		tm.historyDers[2] = make([]float64, len(state))
		copy(tm.historyStates[2], state)
		copy(tm.historyDers[2], deriv)
	case 1:
		tm.historyStates[1] = make([]float64, len(state))
		tm.historyDers[1] = make([]float64, len(state))
		copy(tm.historyStates[1], state)
		copy(tm.historyDers[1], deriv)
	case 2:
		tm.historyStates[0] = make([]float64, len(state))
		tm.historyDers[0] = make([]float64, len(state))
		copy(tm.historyStates[0], state)
		copy(tm.historyDers[0], deriv)
		tm.historyInited = true
	}

	return nil
}

// doStep 执行一步 DoStep，根据 ParallelOpts 选择串行或并行
func doStep(con *element.Context) error {
	if con.ParallelOpts != nil {
		return con.ParallelCallMark(element.MarkDoStep)
	}
	con.CallMark(element.MarkDoStep)
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
// 步长已在别处确定（DC电路由AdjustStepSize调整，含储能元件电路使用固定步长），
// 此函数仅负责：currentTime += currentStep、触发点截断、目标时间截断
func advanceTimeSimple(con *element.Context) error {
	if con.IsSimulationFinished() {
		return nil
	}
	// 获取当前时间和步长
	currentTime := con.CurrentTime()
	timeStep := con.CurrentStep()
	targetTime := con.TargetTime()
	// 计算下一个时间
	nextTime := currentTime + timeStep
	// 触发点截断：确保不会越过未触发的触发点
	triggers := con.Triggers()
	for i := range triggers {
		if !triggers[i].Triggered && triggers[i].Time > currentTime && nextTime > triggers[i].Time {
			nextTime = triggers[i].Time
		}
	}
	if nextTime > targetTime {
		nextTime = targetTime
	}
	// 更新仿真时间
	con.SetTime(nextTime)
	// 标记已到达的触发点
	for i := range triggers {
		if !triggers[i].Triggered && nextTime >= triggers[i].Time {
			triggers[i].Triggered = true
		}
	}
	return nil
}
