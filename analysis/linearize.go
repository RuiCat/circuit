// DC 工作点求解与非线性元件小信号线性化。
// 工作点用 element 时域体系求解(成熟模型:二极管/逻辑门/运放),
// 仅支持纯 DC 电路(无储能元件 C/L)。
package analysis

import (
	_ "circuit/element/register" // 注册全部元件类型(load 依赖)
	etime "circuit/element/time"
	"circuit/load"
	"circuit/mna"
	"fmt"
	"math"
	"strings"
)

// diodeSmallSignalG 二极管工作点小信号电导:
// g_d = dI/dV = Is·e^(Vd/(N·Vt))/(N·Vt),Vt = kT/q(27°C 默认 0.025865V)。
// 反偏时趋近 0,下限 1e-12 保证矩阵良态(与 element Gmin 思路一致)。
func diodeSmallSignalG(c parsedComp, op map[int]float64) float64 {
	is := c.param(0, 1e-14)
	nFactor := c.param(2, 1)
	if nFactor <= 0 {
		nFactor = 1
	}
	const vT = 0.025865 // 27°C 热电压(与 element 二极管默认一致)
	vD := op[c.n1] - op[c.n2]
	expArg := vD / (nFactor * vT)
	var g float64
	switch {
	case expArg > 40: // 溢出上限保护
		g = is / (nFactor * vT) * math.Exp(40)
	case expArg < -40:
		g = is / (nFactor * vT)
	default:
		g = is / (nFactor * vT) * math.Exp(expArg)
	}
	if g < 1e-12 {
		g = 1e-12
	}
	return g
}

// operatingPoint 求解纯 DC 电路的稳态工作点(原始节点 ID → 电压)。
// 变换规则:AC 源(waveform=1)在工作点视为 0V 短路,DC 源保留。
// 调用方需保证电路不含 C/L(储能元件无 DC 稳态)。
func operatingPoint(comps []parsedComp) (map[int]float64, error) {
	netlist := dcNetlist(comps)
	con, err := load.LoadString(netlist)
	if err != nil {
		return nil, fmt.Errorf("工作点网表加载失败: %w", err)
	}
	if con.HasReactiveElements() {
		return nil, fmt.Errorf("工作点求解不支持储能元件(电路含 C/L)")
	}
	tm, err := etime.NewTimeMNA(1.0)
	if err != nil {
		return nil, fmt.Errorf("创建仿真时间失败: %w", err)
	}
	con.Time = tm
	if err := etime.TransientSimulation(con, func(v []float64) {}); err != nil {
		return nil, fmt.Errorf("工作点求解失败: %w", err)
	}
	op := make(map[int]float64)
	for raw, compactIdx := range con.CompactNodeID {
		op[int(raw)] = con.GetNodeVoltage(mna.NodeID(compactIdx))
	}
	if len(op) == 0 {
		return nil, fmt.Errorf("工作点求解未产生节点电压")
	}
	return op, nil
}

// dcNetlist 从解析结果重建 DC 工作点网表:
// AC 源(waveform=1)→ 0V DC 源;AC 电流源 → 0;其余元件保留参数。
func dcNetlist(comps []parsedComp) string {
	var b strings.Builder
	for _, c := range comps {
		switch c.kind {
		case kindResistor:
			fmt.Fprintf(&b, "%s [%d,%d] [%g]\n", c.name, c.n1, c.n2, c.param(0, 0))
		case kindVoltage:
			if c.param(0, 0) == 0 { // DC 源保留
				fmt.Fprintf(&b, "%s [%d,%d] [0,%g,0,0,%g]\n", c.name, c.n1, c.n2,
					c.param(1, 0), c.param(4, 0))
			} else { // AC 源在工作点短路(0V)
				fmt.Fprintf(&b, "%s [%d,%d] [0,0,0,0,0]\n", c.name, c.n1, c.n2)
			}
		case kindCurrent:
			if c.param(0, 0) == 0 {
				fmt.Fprintf(&b, "%s [%d,%d] [0]\n", c.name, c.n1, c.n2)
			} else { // AC 电流源工作点开路
				fmt.Fprintf(&b, "%s [%d,%d] [0]\n", c.name, c.n1, c.n2)
			}
		case kindDiode:
			fmt.Fprintf(&b, "%s [%d,%d] [%g,%g,%g,%g,%g]\n", c.name, c.n1, c.n2,
				c.param(0, 1e-14), c.param(1, 0.7), c.param(2, 1),
				c.param(3, 0.1), c.param(4, 300.15))
		case kindGate:
			pinStrs := make([]string, len(c.pins))
			for i, p := range c.pins {
				pinStrs[i] = fmt.Sprintf("%d", p)
			}
			fmt.Fprintf(&b, "%s [%s] [%g,%g]\n", c.name, strings.Join(pinStrs, ","),
				c.param(0, 0), c.param(1, 5))
		}
	}
	return b.String()
}
