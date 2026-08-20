// 小信号混合分析:DC 工作点 + 工作点线性化 + AC 小信号。
// 对应 SPICE 的 .OP + .AC 最小闭环;逻辑门输出在工作点视为理想电压源(小信号短路),
// 实现逻辑电路与模拟网络在同一分析中的联动。
package analysis

import "fmt"

// SmallSignalResult DC 工作点 + 小信号 AC 响应。
// ACResult 为小信号响应(幅值/相位/复功率),OperatingPoint 为直流偏置。
type SmallSignalResult struct {
	Frequency      float64
	OperatingPoint map[int]float64 // DC 工作点电压(原始节点 ID → V)
	ACResult                       // 小信号响应(嵌入)
}

// AnalyzeSmallSignal 小信号混合分析:
//
//  1. DC 工作点:AC 源视为 0V,DC 源保留,用 element 时域体系收敛
//     (仅支持纯 DC 电路,含 C/L 储能元件时报错)
//  2. 小信号 AC:DC 源短路(0V 理想源)、AC 源注入相量、二极管在工作点
//     线性化(g_d = dI/dV)、逻辑门输出短路(理想电压源)、输入高阻
//
//	netlist: R/C/L/V/I/D/U 网表(见 parse.go)
//	freq: 分析频率(Hz);<=0 时使用网表中第一个 AC 源的 frequency 参数
func AnalyzeSmallSignal(netlist string, freq float64) (*SmallSignalResult, error) {
	comps, err := parseNetlist(netlist)
	if err != nil {
		return nil, err
	}
	for _, c := range comps {
		if c.kind == kindCapacitor || c.kind == kindInductor {
			return nil, fmt.Errorf("行 %d: 小信号分析暂不支持储能元件 %s(直流工作点要求电路无 C/L)", c.line, c.name)
		}
	}
	op, err := operatingPoint(comps)
	if err != nil {
		return nil, fmt.Errorf("DC 工作点求解失败: %w", err)
	}
	res, err := buildAndSolve(comps, freq, op)
	if err != nil {
		return nil, err
	}
	return &SmallSignalResult{
		Frequency:      res.Frequency,
		OperatingPoint: op,
		ACResult:       *res,
	}, nil
}
