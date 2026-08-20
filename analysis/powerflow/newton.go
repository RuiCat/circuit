package powerflow

import (
	"circuit/maths"
	"fmt"
	"math"
)

// Solve 求解潮流(极坐标牛顿-拉夫逊)。
// 未知量:非 Slack 母线 θ + PQ 母线 V;方程:非 Slack ΔP + PQ ΔQ。
// PV 母线无功越限时自动转 PQ(固定 Q 于边界),经典处理。
func Solve(in Input, opts *Options) (*Result, error) {
	s, err := prepare(in, opts)
	if err != nil {
		return nil, err
	}
	return s.solve()
}

// role 根据当前母线类型确定未知量参与
func (s *solver) role() {
	n := len(s.buses)
	s.thetaBus = make([]bool, n)
	s.vBus = make([]bool, n)
	for i, b := range s.buses {
		s.thetaBus[i] = b.Type != BusSlack
		s.vBus[i] = b.Type == BusPQ
	}
}

// solve 牛顿-拉夫逊主循环
func (s *solver) solve() (*Result, error) {
	s.role()
	res := &Result{
		BusV:         make(map[int]complex128),
		GenPower:     make(map[int]complex128),
		BranchFlow:   make(map[string]BranchFlow),
		IterationLog: make([]float64, 0, 16),
	}
	// 迭代
	for iter := 0; iter < s.opts.MaxIter; iter++ {
		p, q, maxMismatch := s.powerMismatch()
		res.IterationLog = append(res.IterationLog, maxMismatch)
		if maxMismatch < s.opts.Tol {
			res.Converged = true
			res.Iterations = iter
			break
		}
		if err := s.newtonStep(p, q); err != nil {
			return nil, fmt.Errorf("第 %d 轮迭代失败: %w", iter+1, err)
		}
		// PV 无功越限检查
		s.enforcePVLimits(q)
	}
	if !res.Converged {
		res.Iterations = s.opts.MaxIter
	}
	// 后处理:母线电压、注入功率、支路潮流、网损
	s.postProcess(res)
	return res, nil
}

// newtonStep 单轮牛顿迭代:构建雅可比,求解 Δθ/ΔV,更新状态
func (s *solver) newtonStep(p, q []float64) error {
	n := len(s.buses)
	// 未知量编号:θ(非 Slack)、V(PQ)
	thetaIdx := make([]int, n) // 母线 → θ 未知量序号(-1 = Slack)
	vIdx := make([]int, n)     // 母线 → V 未知量序号(-1 = 非 PQ)
	nTheta, nV := 0, 0
	for i := 0; i < n; i++ {
		thetaIdx[i] = -1
		vIdx[i] = -1
	}
	for i := 0; i < n; i++ {
		if s.thetaBus[i] {
			thetaIdx[i] = nTheta
			nTheta++
		}
		if s.vBus[i] {
			vIdx[i] = nV
			nV++
		}
	}
	size := nTheta + nV
	if size == 0 {
		return fmt.Errorf("没有未知量(电路全为 Slack 母线?)")
	}
	// 雅可比 J + 失配向量 m
	J := maths.NewDenseMatrix[float64](size, size)
	m := maths.NewDenseVector[float64](size)
	row := 0
	for i := 0; i < n; i++ {
		if !s.thetaBus[i] {
			continue
		}
		m.Set(row, s.buses[i].P-p[i])
		vi := s.buses[i].V
		thetai := s.buses[i].Theta
		gii := real(s.Y[i][i])
		bii := imag(s.Y[i][i])
		for k := 0; k < n; k++ {
			thetaDiff := thetai - s.buses[k].Theta
			g := real(s.Y[i][k])
			b := imag(s.Y[i][k])
			// H: ∂P_i/∂θ_k
			var h float64
			if k == i {
				h = -q[i] - bii*vi*vi
			} else {
				h = vi * s.buses[k].V * (g*math.Sin(thetaDiff) - b*math.Cos(thetaDiff))
			}
			if thetaIdx[k] >= 0 {
				J.Set(row, thetaIdx[k], J.Get(row, thetaIdx[k])+h)
			}
			// N: ∂P_i/∂V_k · V_k
			if vIdx[k] >= 0 {
				var nn float64
				if k == i {
					nn = p[i] + gii*vi*vi
				} else {
					nn = vi * s.buses[k].V * (g*math.Cos(thetaDiff) + b*math.Sin(thetaDiff))
				}
				J.Set(row, nTheta+vIdx[k], J.Get(row, nTheta+vIdx[k])+nn)
			}
		}
		row++
	}
	for i := 0; i < n; i++ {
		if !s.vBus[i] {
			continue
		}
		m.Set(row, s.buses[i].Q-q[i])
		vi := s.buses[i].V
		thetai := s.buses[i].Theta
		gii := real(s.Y[i][i])
		bii := imag(s.Y[i][i])
		for k := 0; k < n; k++ {
			thetaDiff := thetai - s.buses[k].Theta
			g := real(s.Y[i][k])
			b := imag(s.Y[i][k])
			// J: ∂Q_i/∂θ_k
			if thetaIdx[k] >= 0 {
				var jm float64
				if k == i {
					jm = p[i] - gii*vi*vi
				} else {
					jm = -vi * s.buses[k].V * (g*math.Cos(thetaDiff) + b*math.Sin(thetaDiff))
				}
				J.Set(row, thetaIdx[k], J.Get(row, thetaIdx[k])+jm)
			}
			// L: ∂Q_i/∂V_k · V_k
			if vIdx[k] >= 0 {
				var ll float64
				if k == i {
					ll = q[i] - bii*vi*vi
				} else {
					ll = vi * s.buses[k].V * (g*math.Sin(thetaDiff) - b*math.Cos(thetaDiff))
				}
				J.Set(row, nTheta+vIdx[k], J.Get(row, nTheta+vIdx[k])+ll)
			}
		}
		row++
	}
	// 求解 J·x = m(均衡化 LU)
	lu, err := maths.NewLU[float64](size)
	if err != nil {
		return err
	}
	rowScale, colScale, err := maths.EquilibrateAndDecompose(lu, J)
	if err != nil {
		return fmt.Errorf("雅可比分解失败: %w", err)
	}
	x := maths.NewDenseVector[float64](size)
	if err := maths.SolveEquilibrated(lu, m, x, rowScale, colScale); err != nil {
		return fmt.Errorf("雅可比求解失败: %w", err)
	}
	// 更新:θ += Δθ;V += ΔV(V = V·(1+ΔV/V))
	damp := s.opts.Damping
	for i := 0; i < n; i++ {
		if thetaIdx[i] >= 0 {
			s.buses[i].Theta += damp * x.Get(thetaIdx[i])
		}
		if vIdx[i] >= 0 {
			dv := damp * x.Get(nTheta+vIdx[i])
			s.buses[i].V *= 1 + dv
			if s.buses[i].V < 1e-6 {
				s.buses[i].V = 1e-6 // 数值保护
			}
		}
	}
	return nil
}

// enforcePVLimits PV 母线无功越限 → 转 PQ(固定 Q 于边界)
func (s *solver) enforcePVLimits(q []float64) {
	for i := range s.buses {
		b := &s.buses[i]
		if b.Type != BusPV {
			continue
		}
		qMaxSet := b.Qmax > b.Qmin // 有限幅区间
		if !qMaxSet {
			continue
		}
		if q[i] > b.Qmax {
			b.Type = BusPQ
			b.Q = b.Qmax
		} else if q[i] < b.Qmin {
			b.Type = BusPQ
			b.Q = b.Qmin
		}
	}
	s.role()
}

// postProcess 结果整理:母线电压、注入功率、支路潮流、网损
func (s *solver) postProcess(res *Result) {
	n := len(s.buses)
	// 母线电压相量
	for _, b := range s.buses {
		res.BusV[b.ID] = complex(b.V*math.Cos(b.Theta), b.V*math.Sin(b.Theta))
	}
	// 注入电流与功率:Slack/PV 的 P、Q(含 PV 越限转 PQ 者,按原类型输出)
	for _, b := range s.buses {
		if b.origType == BusSlack || b.origType == BusPV {
			v := complex(b.V*math.Cos(b.Theta), b.V*math.Sin(b.Theta))
			var iInj complex128 // 注入电流 = Σ_k Y_ik·V_k
			for k := 0; k < n; k++ {
				vk := complex(s.buses[k].V*math.Cos(s.buses[k].Theta), s.buses[k].V*math.Sin(s.buses[k].Theta))
				iInj += s.Y[b.idx][k] * vk
			}
			sInj := v * complex(real(iInj), -imag(iInj)) // S = V·conj(I)
			res.GenPower[b.ID] = sInj
		}
	}
	// 支路潮流:From→To 方向 S_ik = V_i·conj(I_ik)
	var totalLoss complex128
	for _, br := range s.branches {
		vi := res.BusV[br.From]
		vk := res.BusV[br.To]
		z := complex(br.R, br.X)
		y := 1 / z
		tap := br.Tap
		if tap <= 0 {
			tap = 1
		}
		bFrom := complex(0, br.B/(2*tap*tap))
		bTo := complex(0, br.B/2)
		iIk := y*(vi/complex(tap, 0)-vk) + vi*bFrom
		iKi := y*(vk-vi/complex(tap, 0)) + vk*bTo
		sIk := vi * complex(real(iIk), -imag(iIk))
		sKi := vk * complex(real(iKi), -imag(iKi))
		bf := BranchFlow{
			From:  br.From,
			To:    br.To,
			PFrom: real(sIk), QFrom: imag(sIk),
			PTo: real(sKi), QTo: imag(sKi),
		}
		key := fmt.Sprintf("%d->%d", br.From, br.To)
		res.BranchFlow[key] = bf
		totalLoss += sIk + sKi
	}
	res.TotalLoss = totalLoss
}
