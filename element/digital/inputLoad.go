// inputLoad.go：数字输入引脚的负载折线（供 TTL 电气模型复用）。
package digital

import "math"

// InputLoad 是输入引脚的折线负载：低段 IIL、高段 IIH（数据手册的输入电流量级，单位 A），
// 之间按对数内插过渡。
type InputLoad struct {
	IIH float64 // 输入高电平时的负载量级。
	IIL float64 // 输入低电平时的负载量级。
}

// 过渡带由判决阈值缩放而来：knee = 0.32·threshold ~ 0.8·threshold。
// 以 TTL 的 2.5 V 阈值算即 [0.8 V, 2.0 V]。
//
// 为什么必须有过渡带（本轮电路级测试实测的缺陷）：两段阶跃折线（低于阈值一律 IIL）会让
// 「一个输出带多个输入」的电路**自锁在错误稳态**——4 个 LS 输入各按 IIL=1 mS 盖章 = 4 mS，
// 与驱动器 RonHigh=4 kΩ 分压得 0.293 V，而 0.293 V 仍低于阈值 ⇒ 负载继续按低段算 ⇒ 永远到不了高电平。
// 真实 LS 输入电流在阈值附近就塌缩（这正是"扇出 8"能成立的原因），故按对数内插把中间段压下去。
const (
	inputLoadKneeLowScale  = 0.32
	inputLoadKneeHighScale = 0.8
)

// ConductanceAt 返回输入电压 v 处的等效电导（调用方据此在输入节点盖章导纳）。
//
// 三段：v ≤ 0.32·threshold 走高段? 否——走**低段** IIL；v ≥ 0.8·threshold 走高段 IIH；
// 中间按对数内插（电导跨数量级，线性内插会高估中间段负载）。
// threshold 仅用于缩放过渡带；NaN 与退化参数（非正、或高低段相等）按下述防御处理。
func (l InputLoad) ConductanceAt(v, threshold float64) float64 {
	kneeLow, kneeHigh := threshold*inputLoadKneeLowScale, threshold*inputLoadKneeHighScale
	// 退化输入：非正负载、阈值非正、NaN 一律退回旧的阶跃口径（保持既有行为可预期）。
	if !(l.IIL > 0) || !(l.IIH > 0) || !(kneeHigh > kneeLow) || math.IsNaN(v) {
		if v > threshold {
			return l.IIH
		}
		return l.IIL
	}
	switch {
	case v <= kneeLow:
		return l.IIL
	case v >= kneeHigh:
		return l.IIH
	default:
		t := (v - kneeLow) / (kneeHigh - kneeLow)
		return math.Exp(math.Log(l.IIL)*(1-t) + math.Log(l.IIH)*t)
	}
}
