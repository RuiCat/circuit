// 稳态分析工具:AC 相量分析、频率扫描、潮流计算。
// 复用 analysis 包;AC/潮流为秒级稳态瞬算,不走 jobs 异步框架。
package mcp

import (
	"circuit/analysis"
	"circuit/analysis/powerflow"
	"fmt"
	"math"
	"sort"
)

// ---------- circuit_run_ac ----------

// handleRunAC circuit_run_ac:对会话网表做线性 AC 相量分析(一次复数 MNA 求解)。
// 返回各节点幅值/相位、支路电流、节点复功率与守恒校验。
func (h *Handler) handleRunAC(args map[string]any) (any, error) {
	sess, err := h.store.Get(argStr(args, "sessionId"))
	if err != nil {
		return nil, err
	}
	if sess.Netlist == "" {
		return nil, fmt.Errorf("会话没有网表,请先 circuit_load_netlist")
	}
	freq := 0.0
	if v, ok := argFloat(args, "frequency"); ok {
		freq = v
	}
	res, err := analysis.AnalyzeAC(sess.Netlist, freq)
	if err != nil {
		return nil, err
	}
	filter := make(map[int]bool)
	for _, n := range argIntSlice(args, "nodes") {
		filter[n] = true
	}
	// 节点电压(幅值/相位/实部/虚部)
	nodeVoltages := make([]map[string]any, 0, len(res.Nodes))
	for _, n := range res.Nodes {
		if len(filter) > 0 && !filter[n] {
			continue
		}
		v := res.NodeVoltages[n]
		nodeVoltages = append(nodeVoltages, map[string]any{
			"node":      n,
			"mag":       math.Hypot(real(v), imag(v)),
			"phaseDeg":  math.Atan2(imag(v), real(v)) * 180 / math.Pi,
			"real":      real(v),
			"imag":      imag(v),
		})
	}
	branchCurrent := make([]map[string]any, 0, len(res.Branches))
	for _, name := range res.Branches {
		i := res.BranchCurrent[name]
		branchCurrent = append(branchCurrent, map[string]any{
			"element":  name,
			"mag":      math.Hypot(real(i), imag(i)),
			"phaseDeg": math.Atan2(imag(i), real(i)) * 180 / math.Pi,
			"real":     real(i),
			"imag":     imag(i),
		})
	}
	nodePowers := make([]map[string]any, 0, len(res.NodePowers))
	for _, n := range sortedIntKeys(res.NodePowers) {
		s := res.NodePowers[n]
		nodePowers = append(nodePowers, map[string]any{
			"node":   n,
			"p":      real(s),
			"q":      imag(s),
		})
	}
	return map[string]any{
		"frequency":     res.Frequency,
		"nodeVoltages":  nodeVoltages,
		"branchCurrent": branchCurrent,
		"nodePowers":    nodePowers,
		"sourcesPower":  map[string]float64{"p": real(res.SourcesPower), "q": imag(res.SourcesPower)},
		"totalPower":    map[string]float64{"p": real(res.TotalPower), "q": imag(res.TotalPower)},
	}, nil
}

// ---------- circuit_sweep_ac ----------

// handleSweepAC circuit_sweep_ac:频率扫描,输出幅频/相频曲线数据(供 export_plot 复用)。
func (h *Handler) handleSweepAC(args map[string]any) (any, error) {
	sess, err := h.store.Get(argStr(args, "sessionId"))
	if err != nil {
		return nil, err
	}
	if sess.Netlist == "" {
		return nil, fmt.Errorf("会话没有网表,请先 circuit_load_netlist")
	}
	fMin, ok1 := argFloat(args, "fMin")
	fMax, ok2 := argFloat(args, "fMax")
	if !ok1 || !ok2 {
		return nil, errMissing("fMin/fMax")
	}
	points := 100
	if v, ok := argInt(args, "points"); ok && v >= 2 {
		points = v
	}
	if points > 10000 {
		points = 10000
	}
	logScale := false
	if v, ok := argBool(args, "logScale"); ok {
		logScale = v
	}
	res, err := analysis.SweepAC(sess.Netlist, fMin, fMax, points, logScale)
	if err != nil {
		return nil, err
	}
	filter := make(map[int]bool)
	for _, n := range argIntSlice(args, "nodes") {
		filter[n] = true
	}
	curves := make([]map[string]any, 0, len(res.Nodes))
	for _, n := range res.Nodes {
		if len(filter) > 0 && !filter[n] {
			continue
		}
		mag := make([]float64, len(res.Frequencies))
		phase := make([]float64, len(res.Frequencies))
		for i := range res.Frequencies {
			mag[i] = res.Magnitudes[n][i]
			phase[i] = res.Phases[n][i]
		}
		curves = append(curves, map[string]any{
			"node":   n,
			"mag":    mag,
			"phase":  phase,
		})
	}
	return map[string]any{
		"frequencies": res.Frequencies,
		"logScale":    logScale,
		"curves":      curves,
	}, nil
}

// ---------- circuit_run_powerflow ----------

// handleRunPowerFlow circuit_run_powerflow:牛顿-拉夫逊潮流计算。
// 参数:buses(母线数组)、branches(支路数组)、可选 baseMva/frequency/tol/maxIter/flatStart/damping。
// 母线: {id, type: slack|pv|pq, v?, theta?, p?, q?, qmin?, qmax?}
// 支路: {from, to, r, x, b?, tap?}
func (h *Handler) handleRunPowerFlow(args map[string]any) (any, error) {
	in := powerflow.Input{}
	if v, ok := argFloat(args, "baseMva"); ok {
		in.BaseMVA = v
	}
	if v, ok := argFloat(args, "frequency"); ok {
		in.Frequency = v
	}
	busesRaw, ok := args["buses"].([]any)
	if !ok || len(busesRaw) == 0 {
		return nil, errMissing("buses")
	}
	for i, raw := range busesRaw {
		m, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("buses[%d] 必须是对象", i)
		}
		id, ok := argInt(m, "id")
		if !ok {
			return nil, fmt.Errorf("buses[%d] 缺少 id", i)
		}
		b := powerflow.Bus{ID: id}
		switch argStr(m, "type") {
		case "slack":
			b.Type = powerflow.BusSlack
		case "pv":
			b.Type = powerflow.BusPV
		case "pq", "":
			b.Type = powerflow.BusPQ
		default:
			return nil, fmt.Errorf("buses[%d] type 必须是 slack/pv/pq, 实际 %q", i, argStr(m, "type"))
		}
		if v, ok := argFloat(m, "v"); ok {
			b.V = v
		}
		if v, ok := argFloat(m, "theta"); ok {
			b.Theta = v * math.Pi / 180 // 用户友好:度
		}
		if v, ok := argFloat(m, "p"); ok {
			b.P = v
		}
		if v, ok := argFloat(m, "q"); ok {
			b.Q = v
		}
		if v, ok := argFloat(m, "qmin"); ok {
			b.Qmin = v
		}
		if v, ok := argFloat(m, "qmax"); ok {
			b.Qmax = v
		}
		in.Buses = append(in.Buses, b)
	}
	branchesRaw, ok := args["branches"].([]any)
	if !ok || len(branchesRaw) == 0 {
		return nil, errMissing("branches")
	}
	for i, raw := range branchesRaw {
		m, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("branches[%d] 必须是对象", i)
		}
		from, ok1 := argInt(m, "from")
		to, ok2 := argInt(m, "to")
		r, ok3 := argFloat(m, "r")
		x, ok4 := argFloat(m, "x")
		if !ok1 || !ok2 || !ok3 || !ok4 {
			return nil, fmt.Errorf("branches[%d] 需要 from/to/r/x", i)
		}
		br := powerflow.Branch{From: from, To: to, R: r, X: x}
		if v, ok := argFloat(m, "b"); ok {
			br.B = v
		}
		if v, ok := argFloat(m, "tap"); ok {
			br.Tap = v
		}
		in.Branches = append(in.Branches, br)
	}
	opts := &powerflow.Options{}
	if v, ok := argFloat(args, "tol"); ok {
		opts.Tol = v
	}
	if v, ok := argInt(args, "maxIter"); ok {
		opts.MaxIter = v
	}
	if v, ok := argFloat(args, "damping"); ok {
		opts.Damping = v
	}
	if v, ok := argBool(args, "flatStart"); ok {
		opts.FlatStart = v
	}
	res, err := powerflow.Solve(in, opts)
	if err != nil {
		return nil, err
	}
	busVoltages := make([]map[string]any, 0, len(res.BusV))
	for _, id := range sortedIntKeys(res.BusV) {
		v := res.BusV[id]
		busVoltages = append(busVoltages, map[string]any{
			"bus":      id,
			"mag":      math.Hypot(real(v), imag(v)),
			"phaseDeg": math.Atan2(imag(v), real(v)) * 180 / math.Pi,
		})
	}
	genPower := make([]map[string]any, 0, len(res.GenPower))
	for _, id := range sortedIntKeys(res.GenPower) {
		s := res.GenPower[id]
		genPower = append(genPower, map[string]any{
			"bus": id,
			"p":   real(s),
			"q":   imag(s),
		})
	}
	branchFlows := make([]map[string]any, 0, len(res.BranchFlow))
	for _, key := range sortedKeys(res.BranchFlow) {
		bf := res.BranchFlow[key]
		branchFlows = append(branchFlows, map[string]any{
			"branch": key,
			"pFrom":  bf.PFrom, "qFrom": bf.QFrom,
			"pTo": bf.PTo, "qTo": bf.QTo,
		})
	}
	return map[string]any{
		"converged":   res.Converged,
		"iterations":  res.Iterations,
		"busVoltages": busVoltages,
		"genPower":    genPower,
		"branchFlows": branchFlows,
		"totalLoss":   map[string]float64{"p": real(res.TotalLoss), "q": imag(res.TotalLoss)},
		"mismatchLog": res.IterationLog,
	}, nil
}

// sortedIntKeys 整数键排序
func sortedIntKeys[V any](m map[int]V) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}
