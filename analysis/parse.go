// Package analysis 提供稳态分析能力:线性 AC 相量分析(L1)、DC 工作点 + 小信号 AC(L2)。
// 独立于 element 时域体系,只依赖 load/ast 语法解析 + maths 复数线性代数 + mna 复数 MNA。
// 网表语法与 element 体系一致:R/C/L/V/I,节点 0 为普通节点,GND = -1。
package analysis

import (
	"circuit/load/ast"
	"fmt"
	"strings"
)

// compKind 元件类别(AC/小信号分析支持的子集)
type compKind int

const (
	kindResistor compKind = iota
	kindCapacitor
	kindInductor
	kindVoltage
	kindCurrent
	kindDiode
	kindGate
)

// parsedComp 解析后的元件
type parsedComp struct {
	kind   compKind
	name   string // 实例名(如 "R1")
	pins   []int  // 全部引脚原始 ID(多引脚元件如逻辑门;GND = -1)
	n1, n2 int    // 前两个引脚(双引脚元件的端口)
	params []float64
	line   int
}

// param 读取第 i 个参数,缺失或非法时返回默认值
func (c *parsedComp) param(i int, def float64) float64 {
	if i < 0 || i >= len(c.params) {
		return def
	}
	return c.params[i]
}

// parseNetlist 解析网表(R/C/L/V/I/D/U),参数格式与 element 体系一致:
//
//	R1 [n1,n2] [R]           — 电阻
//	C1 [n1,n2] [C]           — 电容(AC 导纳 jωC)
//	L1 [n1,n2] [L]           — 电感(AC 导纳 1/(jωL))
//	V1 [n+,n-] [waveform,bias,frequency,phase,V_max] — 电压源
//	  waveform=0(DC):相量 (V_max+bias)∠0°;waveform=1(AC):相量 V_max∠phase°
//	I1 [n+,n-] [I]           — 电流源:相量 I∠0°
//	D1 [a,k] [Is,Vz,N,Rs,T]  — 二极管(小信号分析线性化用)
//	U1 [in1,...,out] [type,V_high] — 逻辑门(小信号:输出短路、输入高阻)
func parseNetlist(netlist string) ([]parsedComp, error) {
	tree, err := ast.NewParseTree(strings.NewReader(netlist))
	if err != nil {
		return nil, fmt.Errorf("网表解析失败: %w", err)
	}
	if len(tree.SubCircuitDefs) > 0 {
		return nil, fmt.Errorf("分析暂不支持子电路(.subckt)")
	}
	comps := make([]parsedComp, 0, len(tree.ElementNodes))
	for _, el := range tree.ElementNodes {
		var comp parsedComp
		comp.line = el.Line
		comp.name = strings.ToUpper(el.Type) + el.ID
		// 引脚:至少 2 个
		if len(el.Pins) < 2 {
			return nil, fmt.Errorf("行 %d: 元件 %s 引脚不足 2 个", el.Line, comp.name)
		}
		for _, p := range el.Pins {
			comp.pins = append(comp.pins, parseNodeID(p, el.Line, comp.name))
		}
		comp.n1 = comp.pins[0]
		comp.n2 = comp.pins[1]
		// 参数:全部按 float64 读取(缺失补 0)
		for _, v := range el.Values {
			comp.params = append(comp.params, v.ParseFloat64(0))
		}
		switch strings.ToLower(el.Type) {
		case "r":
			comp.kind = kindResistor
		case "c":
			comp.kind = kindCapacitor
		case "l":
			comp.kind = kindInductor
		case "v":
			comp.kind = kindVoltage
		case "i":
			comp.kind = kindCurrent
		case "d":
			comp.kind = kindDiode
		case "u":
			comp.kind = kindGate
		default:
			return nil, fmt.Errorf("行 %d: 分析不支持的元件类型 %q(支持 R/C/L/V/I/D/U)", el.Line, el.Type)
		}
		comps = append(comps, comp)
	}
	if len(comps) == 0 {
		return nil, fmt.Errorf("网表中没有元件")
	}
	return comps, nil
}

// parseNodeID 解析引脚节点 ID(整数;GND = -1)
func parseNodeID(v ast.Value, line int, name string) int {
	id := v.ParseInt(-1)
	if id < -1 {
		return -1 // 非法节点视为地(与 element 体系一致,保守处理)
	}
	return id
}
