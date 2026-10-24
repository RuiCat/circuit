// dcSweep.go：DC 电压源扫描（m3，c3）。
package analysis

import (
	"fmt"
	"strings"
)

// DCSweepPoint 是扫描的一个点：源值 + 各节点 DC 电压。
type DCSweepPoint struct {
	SourceValue  float64
	NodeVoltages map[int]float64
}

// RunDCSweep 对指定电压源做 DC 扫描（source 如 "V1"；step 非 0）。
// 每个点把源设为 waveform=0、bias=值、V_max=0，用 freq=0 的复 MNA 求 DC 工作点。
func RunDCSweep(netlist, source string, start, stop, step float64) ([]DCSweepPoint, error) {
	if step == 0 {
		return nil, fmt.Errorf("DC sweep step 不能为 0")
	}
	comps, err := parseNetlist(netlist)
	if err != nil {
		return nil, err
	}
	var src *parsedComp
	want := strings.ToUpper(source)
	for i := range comps {
		if comps[i].name == want {
			src = &comps[i]
			break
		}
	}
	if src == nil || src.kind != kindVoltage {
		return nil, fmt.Errorf("DC sweep 源 %q 不是电压源", source)
	}
	points := make([]DCSweepPoint, 0)
	for v := start; ; v += step {
		if step > 0 && v > stop+1e-12 {
			break
		}
		if step < 0 && v < stop-1e-12 {
			break
		}
		src.params = padParams(src.params, 5)
		src.params[0] = 0 // waveform=DC
		src.params[1] = v // bias
		src.params[4] = 0 // V_max
		res, err := buildAndSolve(comps, 1e-12, nil) // ω≈0 近似 DC
		if err != nil {
			return nil, fmt.Errorf("DC sweep @ %g: %w", v, err)
		}
		point := DCSweepPoint{SourceValue: v, NodeVoltages: make(map[int]float64, len(res.NodeVoltages))}
		for node, phasor := range res.NodeVoltages {
			point.NodeVoltages[node] = real(phasor)
		}
		points = append(points, point)
		if len(points) > 100000 {
			return nil, fmt.Errorf("DC sweep 点数过多")
		}
	}
	return points, nil
}

// padParams 把参数切片补齐到 n 个（补 0）。
func padParams(params []float64, n int) []float64 {
	for len(params) < n {
		params = append(params, 0)
	}
	return params
}
