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
//	timeMNA: 时间管理和自适应步长控制器
//	mnaSolver: MNA矩阵求解器（支持更新和回滚）
//	circuitElements: 电路元件列表
//	call: 每步成功后的回调函数，接收节点电压数组
func TransientSimulation(timeMNA *TimeMNA, mnaSolver mna.UpdateMNA, circuitElements []element.NodeFace, call func([]float64)) error {
	// 初始化阶段：获取电路规模并创建求解器
	nodesNum, voltageSourcesNum := mnaSolver.GetNodeNum(), mnaSolver.GetVoltageSourcesNum()
	// 创建LU分解器（根据矩阵稀疏性选择稠密或稀疏实现）
	luSolver, err := maths.NewLU(nodesNum + voltageSourcesNum)
	if err != nil {
		return fmt.Errorf("LU分解器初始化失败: %v", err)
	}
	// 电压数组用于存储每步的节点电压结果
	voltages := make([]float64, nodesNum)
	// 标记是否需要重新加盖线性元件（步长变化或首次迭代）
	needLinearStamp := true
	// 初始化所有元件状态
	element.CallMark(element.MarkReset, mnaSolver, timeMNA, circuitElements)
	updateElements(mnaSolver, circuitElements)
	// 主时间迭代循环
	timeMNA.ResetTimeStepCount()
	for !timeMNA.IsSimulationFinished() {
		// 检查是否超过最大时间步数
		if !timeMNA.IncrementTimeStepCount() {
			return fmt.Errorf("达到最大时间步数限制 %d，仿真可能陷入无限循环", timeMNA.maxTimeSteps)
		}
		// 重置当前时间步的非线性迭代状态
		timeMNA.ResetNonlinearIter()
		// 回滚到线性状态（丢弃非线性迭代的修改）
		mnaSolver.Rollback()
		// 线性元件处理
		if needLinearStamp {
			// 需要重新加盖线性元件
			needLinearStamp = false
			// 还原求解状态为上一次收敛结果
			rollbackElements(mnaSolver, circuitElements)
			// 清空矩阵和向量
			mnaSolver.GetA().Base().Zero()
			mnaSolver.GetZ().Base().Zero()
			// 通知元件开始新迭代
			element.CallMark(element.MarkStartIteration, mnaSolver, timeMNA, circuitElements)
			// 加盖线性元件贡献
			element.CallMark(element.MarkStamp, mnaSolver, timeMNA, circuitElements)
			// 保存线性状态（用于后续回滚）
			mnaSolver.Update()
		} else {
			// 重用已有的线性贡献，仅通知元件开始新迭代
			element.CallMark(element.MarkStartIteration, mnaSolver, timeMNA, circuitElements)
		}
		// 非线性迭代（牛顿-拉夫逊法）
		newtonConverged := false
		for timeMNA.NextNonlinearIter() {
			// 清空未收敛元件列表
			timeMNA.ResetUnconvergedList()
			// 遍历所有非线性元件，计算其贡献
			for i := range circuitElements {
				// 设置元件收敛状态为真（假设收敛）
				timeMNA.SetElementConverged(true)
				// 执行元件计算
				element.CallMark(element.MarkDoStep, mnaSolver, timeMNA, []element.NodeFace{circuitElements[i]})
				// 检查元件是否收敛
				if !timeMNA.IsElementConverged() {
					// 记录未收敛元件
					timeMNA.AddUnconvergedElem(i)
				}
			}
			// 求解MNA方程
			if err := luSolver.Decompose(mnaSolver.GetA()); err != nil {
				return fmt.Errorf("矩阵分解失败（时间=%.6e，步长=%.6e）: %v",
					timeMNA.CurrentTime(), timeMNA.CurrentStep(), err)
			}
			// 执行前向替换和后向替换
			if err := luSolver.SolveReuse(mnaSolver.GetZ(), mnaSolver.GetX()); err != nil {
				return fmt.Errorf("方程求解失败（时间=%.6e）: %v", timeMNA.CurrentTime(), err)
			}
			// 计算残差并检查收敛
			if err := timeMNA.CalculateMNAResidual(mnaSolver); err != nil {
				return fmt.Errorf("残差计算失败: %v", err)
			}
			timeMNA.CheckResidualConvergence()
			// 检查全局收敛条件
			if timeMNA.IsConverged() {
				newtonConverged = true
				break // 牛顿迭代收敛，退出内层循环
			}
			// 处理未收敛的元件（单独迭代）
			if len(timeMNA.UnconvergedElems()) > 0 {
				if err := handleUnconvergedElements(timeMNA, mnaSolver, luSolver,
					circuitElements, timeMNA.UnconvergedElems()); err != nil {
					return err
				}
			}
			// 检查元件级收敛
			if timeMNA.IsElementConverged() {
				newtonConverged = true
				break
			}
		}
		// 检查牛顿迭代是否成功收敛
		if !newtonConverged {
			return fmt.Errorf("牛顿迭代在时间 %.6e 未收敛（达到最大迭代次数 %d）",
				timeMNA.CurrentTime(), timeMNA.MaxNonlinearIter())
		}
		// 后处理：计算电流和更新元件状态
		element.CallMark(element.MarkCalculateCurrent, mnaSolver, timeMNA, circuitElements)
		element.CallMark(element.MarkStepFinished, mnaSolver, timeMNA, circuitElements)
		// 提取并验证节点电压
		if !extractAndValidateVoltages(mnaSolver, nodesNum, voltages) {
			return fmt.Errorf("检测到无效电压值（NaN/Inf）在时间 %.6e，停止仿真", timeMNA.CurrentTime())
		}
		// 更新残差历史并调整步长
		timeMNA.UpdateResidualHistory()
		if timeMNA.ShouldAdjustStepSize() {
			needLinearStamp = true // 步长变化较大，需要重新加盖线性元件
		}
		// 检查残差是否可接受并推进时间
		if timeMNA.IsResidualConverged() {
			// 残差可接受，推进时间
			if err := advanceTimeSimple(timeMNA); err != nil {
				return fmt.Errorf("时间推进失败: %v", err)
			}
			// 接受求解状态
			updateElements(mnaSolver, circuitElements)
			// 重置计数
			timeMNA.ResetTimeStepCount()
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

// handleUnconvergedElements 处理未收敛的元件，进行单独迭代
func handleUnconvergedElements(timeMNA *TimeMNA, mnaSolver mna.UpdateMNA,
	luSolver maths.LU, circuitElements []element.NodeFace, unconvergedIndices []int) error {
	// 对每个未收敛元件进行单独迭代
	for _, elemIdx := range unconvergedIndices {
		timeMNA.ResetElemIter()
		// 还原求解状态
		rollbackElements(mnaSolver, circuitElements)
		// 元件级迭代循环
		for timeMNA.NextElemIter() {
			// 设置元件收敛状态为真
			timeMNA.SetElementConverged(true)
			// 执行元件计算
			element.CallMark(element.MarkDoStep, mnaSolver, timeMNA, []element.NodeFace{circuitElements[elemIdx]})
			// 检查元件是否收敛
			if timeMNA.IsElementConverged() {
				// 当元件收敛就记录求解状态
				updateElements(mnaSolver, circuitElements)
				break // 元件收敛，退出迭代
			}
			// 重新求解MNA方程（元件状态变化可能影响矩阵）
			if err := luSolver.Decompose(mnaSolver.GetA()); err != nil {
				return fmt.Errorf("矩阵分解失败（元件 %d）: %v", elemIdx, err)
			}
			if err := luSolver.SolveReuse(mnaSolver.GetZ(), mnaSolver.GetX()); err != nil {
				return fmt.Errorf("方程求解失败（元件 %d）: %v", elemIdx, err)
			}
		}
		// 检查元件是否达到最大迭代次数
		if timeMNA.IsElemIterExhausted() {
			return fmt.Errorf("元件 %d 迭代达到最大次数 %d 仍未收敛",
				elemIdx, timeMNA.MaxElemIter())
		}
	}
	return nil
}

// extractAndValidateVoltages 从MNA求解器提取节点电压并验证有效性
func extractAndValidateVoltages(mnaSolver mna.MNA, nodesNum int, voltages []float64) bool {
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
func advanceTimeSimple(timeMNA *TimeMNA) error {
	// 检查仿真状态
	if timeMNA.IsSimulationFinished() {
		return nil
	}
	// 获取当前时间和步长
	currentTime := timeMNA.CurrentTime()
	timeStep := timeMNA.CurrentStep()
	targetTime := timeMNA.TargetTime()
	// 计算下一个时间
	nextTime := currentTime + timeStep
	if nextTime > targetTime {
		nextTime = targetTime
	}
	// 更新仿真时间
	timeMNA.SetTime(nextTime)
	return nil
}

// GetNum 计算节点和电压源数量
func GetNum(circuitElements []element.NodeFace) (nodesNum, voltageSourcesNum int) {
	nodeSet := make(map[mna.NodeID]struct{})
	voltageSourceSet := 0

	for _, elem := range circuitElements {
		base := elem.Base()
		// 收集外部节点
		for i := 0; i < len(base.Nodes); i++ {
			nodeID := elem.GetNodes(i)
			if nodeID != mna.Gnd {
				nodeSet[nodeID] = struct{}{}
			}
		}
		// 收集内部节点
		for i := 0; i < len(base.NodesInternal); i++ {
			nodeID := elem.GetNodesInternal(i)
			if nodeID != mna.Gnd {
				nodeSet[nodeID] = struct{}{}
			}
		}
		// 收集电压源
		voltageSourceSet += len(base.VoltSource)
	}

	return len(nodeSet), voltageSourceSet
}

// updateElements 更新元件状态
func updateElements(mnaSolver mna.UpdateMNA, circuitElements []element.NodeFace) {
	mnaSolver.UpdateX()
	for _, elem := range circuitElements {
		elem.Base().Update()
	}
}

// rollbackElements 回滚元件状态
func rollbackElements(mnaSolver mna.UpdateMNA, circuitElements []element.NodeFace) {
	mnaSolver.RollbackX()
	for _, elem := range circuitElements {
		elem.Base().Rollback()
	}
}
