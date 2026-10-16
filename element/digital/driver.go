// driver.go：数字元件的唯一输出方式——把电平写进元件持有的电压源。
package digital

import "circuit/mna"

// DriveNow 把电压 v 写进元件持有的电压源 vs（mna.UpdateVoltageSource）。
// 元件不得直接写节点解向量，输出只能经电压源钳位。
//
// K1 固定了本函数没有 error 通道：m 为 nil（无输出目标）时直接返回；
// vs 越界由 mna.UpdateVoltageSource 按既有约定忽略（见 mna/mna.go 的范围检查）。
func DriveNow(m mna.Mna, vs mna.VoltageID, v float64) {
	if m == nil {
		return
	}
	m.UpdateVoltageSource(vs, v)
}
