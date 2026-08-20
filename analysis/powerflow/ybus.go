package powerflow

import (
	"math"
)

// solver 潮流求解器内部状态
type solver struct {
	buses    []solveBus
	branches []Branch
	idToIdx  map[int]int
	opts     *Options
	// Ybus:复数稠密矩阵(标幺),自导纳 Y[i][i],互导纳 Y[i][k]
	Y [][]complex128
	// 母线类型推导后的角色:idx → 是否参与 θ / V 未知量
	thetaBus []bool // 参与 ΔP 方程(非 Slack)
	vBus     []bool // 参与 ΔQ 方程(PQ 且未转 PV)
}

// buildYbus 构建导纳矩阵(标幺):
//
//	普通线路(Tap=1): y = 1/(R+jX),对地 B/2 各半
//	变压器(Tap≠1):   From 端等效导纳 y/t²,互导纳 -y/t
func (s *solver) buildYbus() {
	n := len(s.buses)
	s.Y = make([][]complex128, n)
	for i := range s.Y {
		s.Y[i] = make([]complex128, n)
	}
	for _, br := range s.branches {
		i := s.idToIdx[br.From]
		k := s.idToIdx[br.To]
		z := complex(br.R, br.X)
		y := 1 / z // 串联导纳
		tap := br.Tap
		if tap <= 0 {
			tap = 1
		}
		// 变压器模型:From 端 y/t²,互导纳 -y/t
		yFrom := y / complex(tap*tap, 0)
		yMut := y / complex(tap, 0)
		// 对地电纳:B/2 各端(变压器修正:From 端 B/(2t²))
		bFrom := complex(0, br.B/(2*tap*tap))
		bTo := complex(0, br.B/2)
		s.Y[i][i] += yFrom + bFrom
		s.Y[k][k] += y + bTo
		s.Y[i][k] -= yMut
		s.Y[k][i] -= yMut
	}
}

// powerMismatch 计算各母线注入功率与失配量(标幺)。
// 返回 P、Q 数组与每轮最大失配量。
func (s *solver) powerMismatch() (p, q []float64, maxMismatch float64) {
	n := len(s.buses)
	p = make([]float64, n)
	q = make([]float64, n)
	for i := 0; i < n; i++ {
		vi := s.buses[i].V
		thetai := s.buses[i].Theta
		var pi, qi float64
		for k := 0; k < n; k++ {
			vk := s.buses[k].V
			thetaDiff := thetai - s.buses[k].Theta
			g := real(s.Y[i][k])
			b := imag(s.Y[i][k])
			pi += vi * vk * (g*math.Cos(thetaDiff) + b*math.Sin(thetaDiff))
			qi += vi * vk * (g*math.Sin(thetaDiff) - b*math.Cos(thetaDiff))
		}
		p[i] = pi
		q[i] = qi
	}
	// 失配量:非 Slack 母线 ΔP;PQ 母线 ΔQ
	maxMismatch = 0
	for i := range s.buses {
		if s.thetaBus[i] {
			d := s.buses[i].P - p[i]
			if math.Abs(d) > maxMismatch {
				maxMismatch = math.Abs(d)
			}
		}
		if s.vBus[i] {
			d := s.buses[i].Q - q[i]
			if math.Abs(d) > maxMismatch {
				maxMismatch = math.Abs(d)
			}
		}
	}
	return p, q, maxMismatch
}
