// AC 相量分析:复数 MNA 一次求解,输出幅值/相位/复功率,支持频率扫描。
package analysis

import (
	"circuit/maths"
	"circuit/mna"
	"fmt"
	"math"
	"sort"
)

// ACResult 单频 AC 相量分析结果。
// 功率约定:NodePowers[k] = V_k·conj(I_net_into_k)(节点净注入);元件消耗
// (V1-V2)·conj(I);TotalPower = Σ 所有元件消耗(理论 0,用作守恒校验);
// SourcesPower = Σ 独立源消耗(负值 = 净输出功率)。
type ACResult struct {
	Frequency     float64
	NodeVoltages  map[int]complex128 // 原始节点ID → 相量电压 Vm∠φ
	BranchCurrent map[string]complex128 // 元件实例名 → 相量电流(方向 n1→n2)
	NodePowers    map[int]complex128 // 节点复功率(净注入)
	TotalPower    complex128         // 全网复功率守恒校验值(≈0)
	SourcesPower  complex128         // 独立源总消耗(负 = 净输出)
	Nodes         []int              // 原始节点ID,确定性顺序
	Branches      []string           // 元件实例名,确定性顺序
}

// SweepResult 频率扫描结果(幅频/相频曲线)
type SweepResult struct {
	Frequencies []float64
	Nodes       []int
	Magnitudes  map[int][]float64 // 节点ID → 幅值序列
	Phases      map[int][]float64 // 节点ID → 相位序列(度)
}

// AnalyzeAC 线性 AC 相量分析:构建复数 MNA 并一次求解。
//
//	netlist: R/C/L/V/I 网表(见 parse.go)
//	freq: 分析频率(Hz);<=0 时使用网表中第一个 V 源的 frequency 参数
func AnalyzeAC(netlist string, freq float64) (*ACResult, error) {
	comps, err := parseNetlist(netlist)
	if err != nil {
		return nil, err
	}
	for _, c := range comps {
		if c.kind == kindDiode || c.kind == kindGate {
			return nil, fmt.Errorf("行 %d: 元件 %s 为非线性元件,请使用 AnalyzeSmallSignal 进行小信号分析", c.line, c.name)
		}
	}
	return buildAndSolve(comps, freq, nil)
}

// buildAndSolve 构建复数 MNA 并求解(线性 AC 与工作点线性化共用)。
// op: DC 工作点电压(原始节点 ID → V),非线性元件(D/U)线性化依赖;nil = 纯线性 AC。
func buildAndSolve(comps []parsedComp, freq float64, op map[int]float64) (*ACResult, error) {
	// 频率确定:参数优先,其次网表 V 源 frequency
	if freq <= 0 {
		for _, c := range comps {
			if c.kind == kindVoltage {
				if f := c.param(2, 0); f > 0 {
					freq = f
					break
				}
			}
		}
	}
	if freq <= 0 {
		return nil, fmt.Errorf("AC 分析频率必须 > 0(可通过参数 freq 或网表 V 源 frequency 指定)")
	}
	omega := 2 * math.Pi * freq

	// 节点重编号:原始 ID → 紧凑索引(GND=-1 不参与)
	rawSet := make(map[int]bool)
	vsNum := 0
	hasGnd := false
	for _, c := range comps {
		for _, p := range c.pins {
			if p == -1 {
				hasGnd = true
			} else {
				rawSet[p] = true
			}
		}
		if c.kind == kindVoltage || c.kind == kindGate {
			vsNum++
		}
	}
	if !hasGnd {
		return nil, fmt.Errorf("电路缺少接地节点:AC 分析要求至少一个引脚接 GND(-1)(注意:本引擎节点 0 不是地,地是 -1)")
	}
	rawNodes := make([]int, 0, len(rawSet))
	for id := range rawSet {
		rawNodes = append(rawNodes, id)
	}
	sort.Ints(rawNodes)
	compact := make(map[int]int, len(rawNodes))
	for i, id := range rawNodes {
		compact[id] = i
	}
	nodeNum := len(rawNodes)

	m := mna.NewMnaUpdateComplex(nodeNum, vsNum)
	vsIdx := 0

	// cid 原始节点 ID → 紧凑矩阵索引(GND=-1 → mna.Gnd)
	cid := func(raw int) mna.NodeID {
		if raw == -1 {
			return mna.Gnd
		}
		return mna.NodeID(compact[raw])
	}

	// 导纳缓存:用于支路电流后处理
	type branchAdm struct {
		y      complex128 // 复数导纳
		phasor bool       // 电压源支路:电流来自解向量
	}
	adms := make([]branchAdm, len(comps))

	for i, c := range comps {
		n1, n2 := c.n1, c.n2
		switch c.kind {
		case kindResistor:
			r := c.param(0, 0)
			if r <= 0 {
				return nil, fmt.Errorf("行 %d: 电阻 %s 阻值必须 > 0(实际 %g)", c.line, c.name, r)
			}
			m.StampImpedance(cid(n1), cid(n2), complex(r, 0))
			adms[i] = branchAdm{y: complex(1/r, 0)}
		case kindCapacitor:
			capVal := c.param(0, 0)
			if capVal < 0 {
				return nil, fmt.Errorf("行 %d: 电容 %s 容值不能为负(实际 %g)", c.line, c.name, capVal)
			}
			if capVal > 0 {
				y := complex(0, omega*capVal) // jωC
				m.StampAdmittance(cid(n1), cid(n2), y)
				adms[i] = branchAdm{y: y}
			} else {
				adms[i] = branchAdm{y: 0} // 断路
			}
		case kindInductor:
			ind := c.param(0, 0)
			if ind < 0 {
				return nil, fmt.Errorf("行 %d: 电感 %s 感值不能为负(实际 %g)", c.line, c.name, ind)
			}
			if ind > 0 {
				y := complex(0, -1/(omega*ind)) // 1/(jωL) = -j/(ωL)
				m.StampAdmittance(cid(n1), cid(n2), y)
				adms[i] = branchAdm{y: y}
			} else {
				adms[i] = branchAdm{y: 0}
			}
		case kindVoltage:
			waveform := c.param(0, 0)
			vmax := c.param(4, 0)
			bias := c.param(1, 0)
			phaseDeg := c.param(3, 0)
			var phasor complex128
			if op != nil && waveform == 0 {
				// 小信号分析:DC 源在工作点处短路(SPICE .AC 语义)
				phasor = 0
			} else if waveform == 0 { // 线性 AC:DC 源幅值 = V_max + bias,相位 0
				phasor = complex(vmax+bias, 0)
			} else { // AC:幅值 = V_max,相位 phase°
				phase := phaseDeg * math.Pi / 180
				phasor = complex(vmax*math.Cos(phase), vmax*math.Sin(phase))
			}
			m.StampVoltageSource(cid(n1), cid(n2), mna.VoltageID(vsIdx), phasor)
			adms[i] = branchAdm{phasor: true}
			vsIdx++
		case kindCurrent:
			cur := c.param(0, 0)
			if op != nil {
				cur = 0 // 小信号分析:电流源开路(恒定 DC 电流无交流分量)
			}
			m.StampCurrentSource(cid(n1), cid(n2), complex(cur, 0))
			adms[i] = branchAdm{y: 0} // 电流源:电流恒定
		case kindDiode:
			if op == nil {
				return nil, fmt.Errorf("行 %d: 二极管 %s 需小信号分析(工作点线性化)", c.line, c.name)
			}
			g := diodeSmallSignalG(c, op)
			m.StampAdmittance(cid(n1), cid(n2), complex(g, 0))
			adms[i] = branchAdm{y: complex(g, 0)}
		case kindGate:
			// 逻辑门:输出引脚小信号 = 理想电压源短路(0V);输入引脚高阻到地防悬空
			outPin := c.pins[len(c.pins)-1]
			m.StampVoltageSource(cid(outPin), mna.Gnd, mna.VoltageID(vsIdx), 0)
			adms[i] = branchAdm{phasor: true}
			vsIdx++
			for _, in := range c.pins[:len(c.pins)-1] {
				if in != -1 {
					m.StampAdmittance(cid(in), mna.Gnd, 1e-12) // 1e12Ω 输入阻抗
				}
			}
		}
	}

	// 一次求解(均衡化复数 LU)
	systemSize := nodeNum + vsNum
	if systemSize == 0 {
		return nil, fmt.Errorf("电路没有未知量")
	}
	luSolver, err := maths.NewLU[complex128](systemSize)
	if err != nil {
		return nil, fmt.Errorf("LU 分解器初始化失败: %w", err)
	}
	rowScale, colScale, err := maths.EquilibrateAndDecompose(luSolver, m.A)
	if err != nil {
		return nil, fmt.Errorf("矩阵分解失败: %w", err)
	}
	if err := maths.SolveEquilibrated(luSolver, m.Z, m.X, rowScale, colScale); err != nil {
		return nil, fmt.Errorf("方程求解失败: %w", err)
	}

	// 解校验:节点电压 + 电压源电流
	nodeVolt := make(map[int]complex128, nodeNum)
	for i, raw := range rawNodes {
		v := m.GetNodeVoltage(mna.NodeID(i))
		if !maths.IsValidComplex128(v) {
			return nil, fmt.Errorf("节点 %d 电压无效(NaN/Inf)", raw)
		}
		nodeVolt[raw] = v
	}

	// 后处理:支路电流、节点复功率、守恒校验
	res := &ACResult{
		Frequency:     freq,
		NodeVoltages:  nodeVolt,
		BranchCurrent: make(map[string]complex128, len(comps)),
		NodePowers:    make(map[int]complex128),
		Nodes:         rawNodes,
		Branches:      make([]string, 0, len(comps)),
	}
	vsCurrent := make([]complex128, vsNum)
	for i := range vsCurrent {
		vsCurrent[i] = m.GetVoltageSourceCurrent(mna.VoltageID(i))
	}
	vsIdx = 0
	var total complex128
	var sources complex128
	for i, c := range comps {
		v1 := voltageAt(nodeVolt, c.n1)
		v2 := voltageAt(nodeVolt, c.n2)
		var cur complex128
		switch {
		case c.kind == kindVoltage || c.kind == kindGate:
			cur = vsCurrent[vsIdx]
			vsIdx++
		case c.kind == kindCurrent:
			cur = complex(c.param(0, 0), 0)
		default:
			cur = adms[i].y * (v1 - v2)
		}
		res.BranchCurrent[c.name] = cur
		res.Branches = append(res.Branches, c.name)
		// 元件消耗功率 (V1-V2)·conj(I);V 源为负 = 提供功率
		consumed := (v1 - v2) * complex(real(cur), -imag(cur))
		total += consumed
		if c.kind == kindVoltage || c.kind == kindCurrent {
			sources += consumed
		}
		// 节点注入聚合:流入 n1 = +I,流入 n2 = -I
		agg := res.NodePowers
		agg[c.n1] += v1 * complex(real(cur), -imag(cur))
		agg[c.n2] -= v2 * complex(real(cur), -imag(cur))
	}
	res.TotalPower = total
	res.SourcesPower = sources
	// 清除地节点(-1)的功率聚合项
	delete(res.NodePowers, -1)
	return res, nil
}

// voltageAt 读取节点电压,地(GND=-1)返回 0
func voltageAt(voltages map[int]complex128, node int) complex128 {
	if node == -1 {
		return 0
	}
	return voltages[node]
}

// SweepAC 频率扫描:对数或线性采样 fMin~fMax,逐点 AnalyzeAC。
func SweepAC(netlist string, fMin, fMax float64, points int, logScale bool) (*SweepResult, error) {
	if fMin <= 0 || fMax <= fMin {
		return nil, fmt.Errorf("扫描频率范围无效: fMin=%g fMax=%g(需 0 < fMin < fMax)", fMin, fMax)
	}
	if points < 2 {
		points = 2
	}
	if points > 10000 {
		points = 10000
	}
	// 先用一个频率跑通,确定节点集合
	probe, err := AnalyzeAC(netlist, fMin)
	if err != nil {
		return nil, err
	}
	res := &SweepResult{
		Frequencies: make([]float64, points),
		Nodes:       probe.Nodes,
		Magnitudes:  make(map[int][]float64),
		Phases:      make(map[int][]float64),
	}
	for _, n := range probe.Nodes {
		res.Magnitudes[n] = make([]float64, points)
		res.Phases[n] = make([]float64, points)
	}
	for i := 0; i < points; i++ {
		var f float64
		if logScale {
			f = fMin * math.Pow(fMax/fMin, float64(i)/float64(points-1))
		} else {
			f = fMin + (fMax-fMin)*float64(i)/float64(points-1)
		}
		res.Frequencies[i] = f
		r, err := AnalyzeAC(netlist, f)
		if err != nil {
			return nil, fmt.Errorf("频率 %g Hz 分析失败: %w", f, err)
		}
		for _, n := range probe.Nodes {
			v := r.NodeVoltages[n]
			res.Magnitudes[n][i] = math.Hypot(real(v), imag(v))
			res.Phases[n][i] = math.Atan2(imag(v), real(v)) * 180 / math.Pi
		}
	}
	return res, nil
}
