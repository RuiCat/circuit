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
	// 稀疏模式（con.EngineCfg.SparseLU，load 从 CIRCUIT_LUSPARSE 解析）优先使用稀疏 LU；
	// 否则并行稠密 / 串行稠密。
	sparseLU := con.EngineCfg.SparseLU
	var luSolver maths.LU[float64]
	var err error
	switch {
	case sparseLU:
		luSolver, err = maths.NewLUSparse[float64](systemSize)
	case con.ParallelOpts != nil && con.ParallelOpts.StampWorkers > 1:
		luSolver, err = maths.NewParallelLU[float64](systemSize, con.ParallelOpts.StampWorkers)
	default:
		luSolver, err = maths.NewLU[float64](systemSize)
	}
	if err != nil {
		return fmt.Errorf("LU分解器初始化失败: %v", err)
	}
	// 电压数组用于存储每步的节点电压结果
	voltages := make([]float64, nodesNum+voltageSourcesNum)
	// 标记是否需要重新加盖线性元件（步长变化或首次迭代）
	needLinearStamp := true
	// 初始化所有元件状态
	if err = con.CallMark(element.MarkReset); err != nil {
		return fmt.Errorf("元件状态重置失败: %v", err)
	}
	con.ResetTimeStepCount()
	// Gmin 延续（标准 SPICE Gmin stepping）：t=0 直流求解启动延续步进。
	// 大 Gmin 把交叉耦合双稳态阻尼成单稳态，随迭代收敛逐级下调至自然 gmin，
	// 解决锁存器/触发器在 t=0 的对称振荡不收敛问题。默认关闭（con.EngineCfg.GminCont，
	// load 从 CIRCUIT_GMCONT 解析），不影响普通电路的既有行为。
	gminContEnabled := con.EngineCfg.GminCont
	naturalGmin := con.EngineCfg.NaturalGmin
	if con.EngineCfg.GminFinal > 0 {
		con.Time.SetGminFinal(con.EngineCfg.GminFinal)
	}
	// 全局 gmin（con.EngineCfg.GlobalGmin，load 解析 CIRCUIT_GLOBALGMIN）：
	// 到地电导，仅给弱连接节点（如只接截止晶体管基极的控制线）接地。
	// 值必须远小于 RTL 上拉电导（1kΩ=1e-3 S），否则会把逻辑节点拉到中间电平、
	// 门失去增益、锁存器焊死中间态；不叠加 continuationGmin（延续阻尼只由
	// PN 结元件自身读取，不进入矩阵对角）。
	globalGmin := con.EngineCfg.GlobalGmin
	for !con.IsSimulationFinished() {
		// 将事件值同步到元件 NodeValue
		if !con.PushEvents() {
			return nil // 优雅停止
		}
		// Gmin 延续调度：启用时每个时间步都从大 Gmin 开始步进。
		// t=0 直流求解与后续步的锁存器（双稳态）都需要阻尼收敛；
		// 步进完成（gmin 降到终值）后本步解即真实工作点，下一步重新步进。
		if gminContEnabled {
			con.Time.BeginGminStepping()
		}
		// 重置X更新状态，允许本时间步内重新调用UpdateX/RollbackX
		con.MnaUpdateType.ResetXUpdate()
		// 检查是否超过最大时间步数
		if !con.IncrementTimeStepCount() {
			return fmt.Errorf("达到最大时间步数限制，仿真可能陷入无限循环（已执行 %d 步后超限）", con.Time.GoodIterations())
		}
		// 预测阶段：对纯 DC 电路，用 Adams-Bashford 预测提供 Newton 初始猜测
		// 并通过 LTE 估计增长步长；含储能元件的电路使用固定小步长。
		if !con.HasReactiveElements() && con.Time.GoodIterations() >= 3 {
			if err := con.Time.Predict(); err == nil {
				con.Time.CopyPredStateToX(con.GetX())
			}
		}
		con.ResetNonlinearIter()
		// 回滚到线性状态（丢弃非线性迭代的修改）
		con.A.Rollback()
		con.Z.Rollback()
		// 还原解向量X和元件内部状态到上一次收敛结果
		if err = con.CallMark(element.MarkRollbackElements); err != nil {
			return fmt.Errorf("元件状态回滚失败: %v", err)
		}
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
		// 外部强制重盖请求（暂停中修改元件参数后）：置位 needLinearStamp，
		// 恢复仿真时线性元件按新参数重新加盖。
		if tm, ok := con.Time.(*TimeMNA); ok && tm.ConsumeForceStamp() {
			needLinearStamp = true
		}
		// 线性元件处理
		if needLinearStamp {
			// 需要重新加盖线性元件
			needLinearStamp = false
			// 清空矩阵和向量
			con.GetA().Base().Zero()
			con.GetZ().Base().Zero()
			// 通知元件开始新迭代
			if err = con.CallMark(element.MarkStartIteration); err != nil {
				return fmt.Errorf("元件开始新迭代失败: %v", err)
			}
			// 加盖线性元件贡献
			if err = con.CallMark(element.MarkStamp); err != nil {
				return fmt.Errorf("元件加盖失败: %v", err)
			}
			// 保存线性状态（用于后续回滚）
			con.Update()
		} else {
			// 重用已有的线性贡献，仅通知元件开始新迭代
			if err = con.CallMark(element.MarkStartIteration); err != nil {
				return fmt.Errorf("元件开始新迭代失败: %v", err)
			}
		}
		// 非线性迭代（牛顿-拉夫逊法）
		newtonConverged := false
		newtonIterCount := 0
		refillConverged := false // 显式补轮后是否收敛（P3：收敛则外层立即终止，避免重复触发采样）
		var luRetry bool
		for con.NextNonlinearIter() {
			newtonIterCount++
			// 回滚到线性基准状态（MarkStamp），避免上一轮DoStep的累积
			con.A.Rollback()
			con.Z.Rollback()
			// 遍历所有非线性元件，计算其贡献
			if err := doStep(con); err != nil {
				return err
			}
			// 求解MNA方程（使用均衡化LU分解以处理混合域）
			rowScale, colScale, err := equilibrateDecompose(luSolver, con.GetA(), sparseLU, gminForLU(naturalGmin, globalGmin), con.EngineCfg.NoEq)
			if err != nil {
				newStep := con.CurrentStep() / 2
				if newStep < con.MinTimeStep() {
					return fmt.Errorf("矩阵分解失败且步长已最小（时间=%.6e）: %v", con.CurrentTime(), err)
				}
				con.SetTimeStep(newStep)
				needLinearStamp = true
				luRetry = true
				break
			}
			// 执行前向替换和后向替换
			if err := maths.SolveEquilibrated(luSolver, con.GetZ(), con.GetX(), rowScale, colScale); err != nil {
				newStep := con.CurrentStep() / 2
				if newStep < con.MinTimeStep() {
					return fmt.Errorf("方程求解失败且步长已最小（时间=%.6e）: %v", con.CurrentTime(), err)
				}
				con.SetTimeStep(newStep)
				needLinearStamp = true
				luRetry = true
				break
			}
			// 计算残差并检查收敛
			if err := con.CalculateMNAResidual(con); err != nil {
				return fmt.Errorf("残差计算失败: %v", err)
			}
			// 检查全局收敛条件
			// 最少2轮迭代：防止第1轮线性精确求解后残差为0导致的假收敛
			con.CheckResidualConvergence()
			if con.IsConverged() && newtonIterCount >= 2 {
				// 标准 Gmin 步进：收敛于当前阻尼水平，但延续尚未降到终值 →
				// 下调一档并以当前解为热启动继续迭代，直到在自然 gmin 下收敛。
				if con.Time.GminSteppingActive() && con.Time.StepGminDown() {
					if con.EngineCfg.GminDbg {
						fmt.Printf("GMIN_DBG step=%d iter=%d -> gmin=%g\n", con.Time.GoodIterations(), newtonIterCount, con.Time.GetContinuationGmin())
					}
					continue
				}
				newtonConverged = true
				break // 牛顿迭代收敛，退出内层循环
			}
			// 重置元件迭代计数器
			con.ResetElemIter()
			// 开始次级迭代循环
			gminContinue := false
			for con.NextElemIter() {
				// 将矩阵回滚到加盖线性元件之后的状态
				con.A.Rollback()
				con.Z.Rollback()
				// 重新计算所有非线性元件的贡献
				if err := doStep(con); err != nil {
					return err
				}
				// 重新求解MNA方程（使用均衡化LU分解以处理混合域）
				rowScale, colScale, err := equilibrateDecompose(luSolver, con.GetA(), sparseLU, gminForLU(naturalGmin, globalGmin), con.EngineCfg.NoEq)
				if err != nil {
					newStep := con.CurrentStep() / 2
					if newStep < con.MinTimeStep() {
						return fmt.Errorf("元件迭代中矩阵分解失败且步长已最小（时间=%.6e）: %v", con.CurrentTime(), err)
					}
					con.SetTimeStep(newStep)
					needLinearStamp = true
					luRetry = true
					break
				}
				if err := maths.SolveEquilibrated(luSolver, con.GetZ(), con.GetX(), rowScale, colScale); err != nil {
					newStep := con.CurrentStep() / 2
					if newStep < con.MinTimeStep() {
						return fmt.Errorf("元件迭代中方程求解失败且步长已最小（时间=%.6e）: %v", con.CurrentTime(), err)
					}
					con.SetTimeStep(newStep)
					needLinearStamp = true
					luRetry = true
					break
				}
				// 重新计算残差并检查收敛
				if err := con.CalculateMNAResidual(con); err != nil {
					return fmt.Errorf("残差计算失败: %v", err)
				}
				con.CheckResidualConvergence()
				// 如果整个系统现在已经收敛，则更新状态并返回
				if con.IsConverged() {
					// 标准 Gmin 步进：中间阻尼解不提交（提交会交换 X，破坏热启动），
					// 下调一档后继续外层牛顿循环，直至在自然 gmin 下收敛。
					if con.Time.GminSteppingActive() && con.Time.StepGminDown() {
						gminContinue = true
						break
					}
					if err = con.CallMark(element.MarkUpdateElements); err != nil {
						return fmt.Errorf("元件状态更新失败: %v", err)
					}
					// P3 修复：显式补轮——Rollback A/Z 缓存 → doStep（元件读已提交
					// 状态）→ 求解写 X → 残差检查。旧实现依赖"外层必然再迭代一轮"
					// 的隐性补轮把解写回 UpdateX 交换后的缓冲区：若 elem 恰好在外层
					// 最后一次迭代收敛（额度耗尽），补轮不执行，电压提取会静默读到
					// 旧解。显式补轮使 X 始终为本步解，与迭代额度无关。
					// 补轮后若收敛（refillConverged）则本步解即最终解，外层立即终止；
					// 若未收敛（如 HCV 单向阀/蓄能器写回历史电流后元件仍抖动），
					// X 已更新为补轮解，外层继续迭代直至稳定。
					con.A.Rollback()
					con.Z.Rollback()
					if err := doStep(con); err != nil {
						return err
					}
					rowScale, colScale, err := equilibrateDecompose(luSolver, con.GetA(), sparseLU, gminForLU(naturalGmin, globalGmin), con.EngineCfg.NoEq)
					if err != nil {
						newStep := con.CurrentStep() / 2
						if newStep < con.MinTimeStep() {
							return fmt.Errorf("补轮矩阵分解失败且步长已最小（时间=%.6e）: %v", con.CurrentTime(), err)
						}
						con.SetTimeStep(newStep)
						needLinearStamp = true
						luRetry = true
						break
					}
					if err := maths.SolveEquilibrated(luSolver, con.GetZ(), con.GetX(), rowScale, colScale); err != nil {
						newStep := con.CurrentStep() / 2
						if newStep < con.MinTimeStep() {
							return fmt.Errorf("补轮方程求解失败且步长已最小（时间=%.6e）: %v", con.CurrentTime(), err)
						}
						con.SetTimeStep(newStep)
						needLinearStamp = true
						luRetry = true
						break
					}
					if err := con.CalculateMNAResidual(con); err != nil {
						return fmt.Errorf("残差计算失败: %v", err)
					}
					con.CheckResidualConvergence()
					if con.IsConverged() {
						newtonConverged = true
						refillConverged = true
					}
					break
				}
			}
			if gminContinue {
				continue
			}
			if luRetry {
				break
			}
			// P3 修复：显式补轮收敛（refillConverged）后立即终止外层牛顿循环。
			// 若不终止，外层再迭代一轮会基于补轮后的新解重新执行元件 doStep
			// （如 DFF 上升沿检测读到补轮写入的旧缓冲 prevClk 而重复触发采样、
			// 把状态改回），破坏"补轮即最终解"的语义并浪费一次 LU。
			if refillConverged {
				break
			}
			// 如果循环结束，意味着即使经过额外的迭代也未能收敛
			// 交叉耦合门可能永远无法收敛；继续牛顿外循环而非失败
			if con.IsElemIterExhausted() {
				// 标准 Gmin 步进：当前阻尼水平下元件子迭代耗尽（振荡）→
				// 增大阻尼一档重试；若已到起始阻尼或恢复超限，回退原有强制推进。
				if con.Time.GminSteppingActive() {
					if con.Time.RecoverGmin() {
						continue
					}
				}
				// 残差已收敛（MNA 方程满足）但元件级电压仍在临界点附近
				// 来回抖动（|ΔV|>0.01 触发 NoConverged）→ 接受当前解。
				// 锁存器/RAM 单元在双稳态临界电压上会持续抖动，等待元件级
				// 收敛会耗尽迭代后强制推进（结果相同且浪费 20 次 LU 分解）。
				if con.IsResidualConverged() {
					newtonConverged = true
					break
				}
				// P8 修复：元件迭代耗尽且残差未收敛 → 真正的数值困难，减半步长重试
				// （原实现无条件强制推进，会静默接受不收敛的解；log 警告易被忽略）。
				// 已到最小步长则报错终止，不再产生错误结果。
				newStep := con.CurrentStep() / 2
				if newStep < con.MinTimeStep() {
					return fmt.Errorf("元件次级迭代耗尽且残差未收敛（时间=%.6e，步长已最小 %.6e）", con.CurrentTime(), con.MinTimeStep())
				}
				con.SetTimeStep(newStep)
				needLinearStamp = true
				luRetry = true
				break // 复用 luRetry 路径：break 外层 → 时间循环减半步长重算
			}
		}
		if luRetry {
			continue
		}
		// 检查牛顿迭代是否成功收敛
		if !newtonConverged {
			return fmt.Errorf("牛顿迭代在时间 %.6e 未收敛（达到最大迭代次数 %d）", con.CurrentTime(), con.MaxNonlinearIter())
		}
		// 后处理：计算电流和更新元件状态
		if err = con.CallMark(element.MarkCalculateCurrent); err != nil {
			return fmt.Errorf("电流计算失败: %v", err)
		}
		if err = con.CallMark(element.MarkStepFinished); err != nil {
			return fmt.Errorf("步骤完成失败: %v", err)
		}
		// 提取并验证节点电压
		if !extractAndValidateVoltages(con, nodesNum, voltageSourcesNum, voltages) {
			return fmt.Errorf("检测到无效电压值（NaN/Inf）在时间 %.6e，停止仿真", con.CurrentTime())
		}
		// 积分误差估计与步长自适应
		// 仅对纯 DC 电路（无储能元件）执行步长调整，快速跳过稳定状态。
		// 含储能元件的电路使用固定步长以保证数值稳定性。
		if !con.HasReactiveElements() {
			if con.Time.GoodIterations() >= 3 {
				con.Time.SetCorrStateFromX(con.GetX())
				con.Time.EstimateLTE()
				if err := con.Time.AdjustStepSize(); err != nil {
					return fmt.Errorf("步长调整失败: %v", err)
				}
			} else {
				deriv := con.ComputeStateDerivative()
				convergedState := make([]float64, systemSize)
				for i := range systemSize {
					convergedState[i] = con.GetX().Get(i)
				}
				con.Time.BootstrapHistory(convergedState, deriv, con.Time.GoodIterations())
			}
		}
		// 注意：残差历史已在 CalculateMNAResidual 中更新，此处不再重复
		if con.ShouldAdjustStepSize() {
			needLinearStamp = true // 步长变化较大，需要重新加盖线性元件
		}
		// 检查残差是否可接受并推进时间。
		// Gmin 延续"步进中"（尚未降到终值）跳过残差检查：中间阻尼解的残差
		// 含 gmin 泄漏电流分量（PN 结元件按延续值计算 I-V，与自然系统不符），
		// 若按常规判据会导致步长反复减半死循环。延续降到终值（gminStepDone）后
		// GminSteppingActive 返回 false，本步解即真实工作点，恢复残差验收
		// （P1 修复：不再永久旁路 Gmin 电路的残差检查）。
		if con.IsResidualConverged() || con.Time.GminSteppingActive() {
			// 残差可接受，推进时间
			if err := con.Time.AdvanceTimeSimple(); err != nil {
				return fmt.Errorf("时间推进失败: %v", err)
			}
			// 在 MarkUpdateElements 之前保存校正状态（此时 X 是收敛解）
			if !con.HasReactiveElements() && con.Time.GoodIterations() >= 3 {
				con.Time.SetCorrStateFromX(con.GetX())
			}
			// 接受求解状态
			if err = con.CallMark(element.MarkUpdateElements); err != nil {
				return fmt.Errorf("元件状态更新失败: %v", err)
			}
			// 重置计数
			con.ResetTimeStepCount()
			// 调用用户回调函数
			if !con.HasReactiveElements() && con.Time.GoodIterations() >= 3 {
				deriv := con.ComputeStateDerivative()
				corrDerPtr := con.Time.CorrDer()
				if corrDerPtr != nil && len(deriv) == len(*corrDerPtr) {
					copy(*corrDerPtr, deriv)
				}
				con.Time.UpdateHistory()
			}
			con.Time.IncrementGoodSteps()
			call(voltages)
			con.PullEvents() // 将生产者元件的状态回写到事件系统
		} else {
			// 残差不可接受，减小步长并重新计算当前步
			newStep := con.CurrentStep() / 2
			if newStep < con.MinTimeStep() {
				return fmt.Errorf("步长已降至最小限制 %.6e，仿真在时间 %.6e 无法收敛", con.MinTimeStep(), con.CurrentTime())
			}
			con.SetTimeStep(newStep)
			needLinearStamp = true
			continue
		}
	}

	// 将X恢复到指向最后一次收敛解。
	// 循环内的最后一次MarkUpdateElements调用了UpdateX，它会交换
	// X和LastX，使得MnaType.X指向"待求解"缓冲区。再次交换
	// 使MnaType.X指向收敛解缓冲区。
	con.MnaUpdateType.ResetXUpdate()
	if err = con.CallMark(element.MarkUpdateElements); err != nil {
		return fmt.Errorf("元件状态更新失败: %v", err)
	}
	return nil
}

// doStep 执行一步 DoStep，根据 ParallelOpts 选择串行或并行
func doStep(con *element.Context) error {
	if con.ParallelOpts != nil {
		return con.ParallelCallMark(element.MarkDoStep)
	}
	return con.CallMark(element.MarkDoStep)
}

// gminForLU 计算本次 LU 分解的对角 gmin：
// 自然 gmin（CIRCUIT_NATGMIN）始终叠加；全局 gmin（CIRCUIT_GLOBALGMIN，
// 默认 1e-9 S=1GΩ，可显式设数值如 1e-7）仅当启用时叠加到所有节点对角
// （标准 SPICE 做法，为弱连接节点提供接地路径）。
// 注意：不再叠加 continuationGmin——Gmin stepping 的阻尼（起始 1e-3 S=1kΩ）
// 若叠加到所有节点对角，会把 RTL 逻辑门（1kΩ 上拉）拉向中间电平，
// 门失去增益、锁存器焊死中间态（见 2026-08-22 诊断）。延续阻尼只由
// PN 结元件自身读取，不进入矩阵对角。
func gminForLU(naturalGmin, globalGmin float64) float64 {
	g := naturalGmin
	if globalGmin > g {
		g = globalGmin
	}
	return g
}

// equilibrateDecompose 根据稀疏开关选择均衡化 LU 分解路径：
// 稀疏模式用只遍历非零元的稀疏版本，否则用稠密版本。两者数值等价。
// gmin>0 时（Gmin stepping 激活）给所有行对角并联 gmin 到地（标准 SPICE 做法的
// 工程变体）：为无直流通路的弱连接节点提供接地路径避免近奇异。节点 KCL 行加 gmin
// 是标准做法；电压源约束行也加（软电压源效应 gmin·I(vs)<<容差，数值可忽略）是
// CPU8 这类大规模 RTL 电路的数值需要——电压源行对角保持非零占位，避免稀疏 LU
// 静态主元序在病态矩阵上假奇异（实测去掉后 CPU8 t=0 分解奇异 k=291）。审查结论
// （P2 候选）曾计划只加节点行，因破坏 CPU8 回归而放弃，注释留档。
// noEq=true 时跳过行/列均衡化（con.EngineCfg.NoEq，load 从 CIRCUIT_NOEQ 解析）：
// 均衡化的缩放会破坏近奇异矩阵（如含大量电压源支路的 MNA 矩阵）的部分主元选择，
// 导致假奇异→步长减半卡死。
func equilibrateDecompose(luSolver maths.LU[float64], A maths.Matrix[float64], sparse bool, gmin float64, noEq bool) (rowScale, colScale []float64, err error) {
	if gmin > 0 {
		n := A.Rows()
		for i := 0; i < n; i++ {
			A.Increment(i, i, gmin)
		}
	}
	if noEq {
		n := A.Rows()
		rowScale = make([]float64, n)
		colScale = make([]float64, n)
		for i := 0; i < n; i++ {
			rowScale[i] = 1
			colScale[i] = 1
		}
		return rowScale, colScale, luSolver.Decompose(A)
	}
	if sparse {
		rowScale, colScale, err = maths.EquilibrateAndDecomposeSparse(luSolver, A)
	} else {
		rowScale, colScale, err = maths.EquilibrateAndDecompose(luSolver, A)
	}
	return rowScale, colScale, err
}

// extractAndValidateVoltages 从MNA求解器提取节点电压和电压源电流并验证有效性
func extractAndValidateVoltages(mnaSolver mna.Mna, nodesNum int, voltageSourcesNum int, voltages []float64) bool {
	allValid := true
	// 验证范围扩展到电压源电流分量，防止 NaN/Inf 静默传播
	totalVars := nodesNum + voltageSourcesNum
	if len(voltages) < totalVars {
		return false
	}
	for i := 0; i < totalVars; i++ {
		if i < nodesNum {
			voltages[i] = mnaSolver.GetNodeVoltage(mna.NodeID(i))
		} else {
			// 电压源电流分量：GetNodeVoltage 只覆盖节点区（对 i>=NodesNum 恒返回 0），
			// 需改用 GetVoltageSourceCurrent 读取真实电流并检测 NaN/Inf。
			voltages[i] = mnaSolver.GetVoltageSourceCurrent(mna.VoltageID(i - nodesNum))
		}
		if math.IsNaN(voltages[i]) || math.IsInf(voltages[i], 0) {
			allValid = false
			break
		}
	}
	return allValid
}

// TransientSimulationContinuous 运行连续仿真（永不停止，直到外部调用 Stop()）。
// 包装 TransientSimulation，自动设置连续模式。
func TransientSimulationContinuous(con *element.Context, call func([]float64)) error {
	tm, ok := con.Time.(*TimeMNA)
	if !ok {
		return fmt.Errorf("TransientSimulationContinuous: Time 不是 *TimeMNA 类型")
	}
	tm.SetContinuousMode()
	return TransientSimulation(con, call)
}
