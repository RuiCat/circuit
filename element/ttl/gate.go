// gate.go：TTL 电气级门模型——输出驱动（内部节点 + 理想电压源 + 串联电阻 Ron）、
// 输入负载（按 IIH/IIL 折线在输入节点盖章导纳）与惯性延时（DelayQueue.PlanReplace）。
//
// 职责边界（K3）：本文件只实现「门级电气行为」，**不含参数数值**（参数表在 spec.go，
// 经 gateSpecLookup 接线）、**不注册元件类型**（注册在 m4 的器件文件里），也不做逻辑
// 功能判决——目标逻辑电平 wantHigh 由器件层算好后传进 Step。
//
// 契约要点：
//   - 输出：内部节点 + 理想电压源（0/VCC）+ 串联电阻（StampImpedance），Ron 由
//     IOH/IOL 与 VOL/VOH 反推（见 GateParams.Ron）；负载把输出拉过 IOH/IOL 时电平可见下陷；
//   - 输入：按 IIH/IIL 折线盖章导纳（digital.InputLoad.ConductanceAt）；
//   - 延时：只经 digital.DelayQueue，惯性语义用 PlanReplace（元件侧零 SetTriggers）；
//   - 状态：全部落在 element.NodeFace 的 NodeValue 槽位，槽位布局由调用方用 GateSlots 给出。
package ttl

import (
	"fmt"
	"math"

	"circuit/element"
	"circuit/element/digital"
	"circuit/mna"
)

// GATE_QUEUE_ARMED 等四个常量是 digital.DelayQueue 的槽位布局（K1：每个 base 占
// [armed, at, value, now] 四个连续 float64 槽位）。本包按该文档化契约读取「是否已排程」
// 与「到点电平」，以便在翻转生效的那一步把驱动电平提交下来。
const (
	// GATE_QUEUE_ARMED 为 base+0：0 空闲 / 1 已排程。
	GATE_QUEUE_ARMED = iota
	// GATE_QUEUE_AT 为 base+1：到点时刻（仿真时间，秒）。
	GATE_QUEUE_AT
	// GATE_QUEUE_VALUE 为 base+2：到点后写入电压源的电平（V）。
	GATE_QUEUE_VALUE
	// GATE_QUEUE_NOW 为 base+3：最近一次 Apply 观察到的仿真时间（秒）。
	GATE_QUEUE_NOW
)

// GateParams 是电气级门模型消费的规格字段（K3 的 DeviceSpec 电气子集）。
//
// IIH/IIL 按 digital.InputLoad 的字段语义当作**导纳量级**（S）使用：输入高电平侧盖章
// IIH、低电平侧盖章 IIL（折算关系由参数表侧负责标定）。
type GateParams struct {
	Model string  // 型号名（查表键与错误信息用）。
	TpdLH float64 // 低→高传播延时（s）。
	TpdHL float64 // 高→低传播延时（s）。
	IIH   float64 // 输入高电平侧的等效导纳（S）。
	IIL   float64 // 输入低电平侧的等效导纳（S）。
	IOH   float64 // 输出高电平驱动上限（A）。
	IOL   float64 // 输出低电平灌电流上限（A）。
	VOH   float64 // 输出高电平（V）。
	VOL   float64 // 输出低电平（V）。
}

// Validate 校验电气参数自洽性：延时与驱动上限必须为正、电平必须落在 0 ≤ VOL < VOH < VCC。
// 非法参数必须报错而不是照常建模（否则会静默产出错误电平与错误延时）。
func (p GateParams) Validate(vcc float64) error {
	switch {
	case p.Model == "":
		return fmt.Errorf("ttl: 电气参数缺少型号名")
	case !(p.TpdLH > 0) || !(p.TpdHL > 0):
		return fmt.Errorf("ttl: 型号 %q 的传播延时必须为正（TpdLH=%g TpdHL=%g）", p.Model, p.TpdLH, p.TpdHL)
	case !(p.IOH > 0) || !(p.IOL > 0):
		return fmt.Errorf("ttl: 型号 %q 的输出驱动上限必须为正（IOH=%g IOL=%g）", p.Model, p.IOH, p.IOL)
	case !(p.VOL >= 0) || !(p.VOH > p.VOL) || !(p.VOH < vcc):
		return fmt.Errorf("ttl: 型号 %q 的电平不合法（需 0 ≤ VOL < VOH < VCC）：VOL=%g VOH=%g VCC=%g",
			p.Model, p.VOL, p.VOH, vcc)
	case p.IIH < 0 || p.IIL < 0:
		return fmt.Errorf("ttl: 型号 %q 的输入导纳不得为负（IIH=%g IIL=%g）", p.Model, p.IIH, p.IIL)
	}
	return nil
}

// RonHigh 反推**拉高方向**的串联电阻：输出把节点拉到 VOH 时，串联电阻上的压降是
// VCC-VOH（额定电流 IOH）⇒ (VCC-VOH)/IOH。
func (p GateParams) RonHigh(vcc float64) (float64, error) {
	if err := p.Validate(vcc); err != nil {
		return 0, err
	}
	return (vcc - p.VOH) / p.IOH, nil
}

// RonLow 反推**灌低方向**的串联电阻：输出把节点灌到 VOL 时，串联电阻上的压降是
// VOL（额定电流 IOL）⇒ VOL/IOL。
func (p GateParams) RonLow(vcc float64) (float64, error) {
	if err := p.Validate(vcc); err != nil {
		return 0, err
	}
	return p.VOL / p.IOL, nil
}

// Ron 返回两个驱动方向串联电阻中的**更大者**（整体更保守的那个），供报告/校验使用；
// 实际盖章按方向分别取 RonHigh/RonLow（上下拉晶体管的导通电阻不同，不能混用）。
//
// 两个方向各自都满足：负载不超过额定电流（IOH/IOL）时，输出电平仍在 VOL/VOH 以内；
// 一旦超过，串联电阻上的压降使电平**可见下陷**，不被静默当作正常。
func (p GateParams) Ron(vcc float64) (float64, error) {
	rHigh, err := p.RonHigh(vcc)
	if err != nil {
		return 0, err
	}
	rLow, err := p.RonLow(vcc)
	if err != nil {
		return 0, err
	}
	return math.Max(rHigh, rLow), nil
}

// gateSpecLookup 是「型号 → 电气参数」的唯一查表出口，默认**未接线**（返回 ok=false）。
//
// 参数表（element/ttl/spec.go 的 DeviceSpec/SpecOf）落盘后由器件层接线，例如：
//
//	func init() {
//		gateSpecLookup = func(model string) (GateParams, bool) {
//			s, ok := SpecOf(model)
//			if !ok {
//				return GateParams{}, false
//			}
//			return GateParams{Model: s.Model, TpdLH: s.TpdLH, TpdHL: s.TpdHL,
//				IIH: s.IIH, IIL: s.IIL, IOH: s.IOH, IOL: s.IOL,
//				VOH: s.VOH, VOL: s.VOL}, true
//		}
//	}
//
// 本文件不复制任何参数数值（C11：数值与来源只存在于参数表）。
var gateSpecLookup = func(model string) (GateParams, bool) {
	return GateParams{}, false
}

// ParamsFromModel 按型号取电气参数：型号为空、未接线或未知型号都必须报错（不得静默用零值建模）。
func ParamsFromModel(model string) (GateParams, error) {
	if model == "" {
		return GateParams{}, fmt.Errorf("ttl: 门模型型号为空")
	}
	params, ok := gateSpecLookup(model)
	if !ok {
		return GateParams{}, fmt.Errorf("ttl: 型号 %q 查不到电气参数（spec.go 未落盘，或 gateSpecLookup 尚未接线）", model)
	}
	return params, nil
}

// GateOutput 描述输出侧在元件实例里的位置：外部输出引脚、输出内部节点、理想电压源。
type GateOutput struct {
	Pin      int // 输出引脚索引（value.GetNodes 的下标）。
	Internal int // 输出内部节点索引（value.GetNodesInternal 的下标）。
	Source   int // 输出理想电压源索引（value.GetVoltSource 的下标）。
}

// GateSlots 描述模型私有状态在元件实例 NodeValue 里的槽位（由器件层分配并登记进
// Config.ValueInit 与 Config.OrigValue）。
type GateSlots struct {
	QueueBase int // DelayQueue 的 base（占 4 个连续槽位，见 GATE_QUEUE_*）。
	Goal      int // 目标电平槽：0 = 低、1 = 高（最近一次排程/输入判决的电平）。
	Drive     int // 驱动电平槽：0 = 低、1 = 高（已经生效、当前由电压源驱动的电平）。
	Stamp     int // 步进幂等槽，初值必须是 math.NaN()（K1 约定），否则 t=0 首步会被误判为已推进。
}

// GateModel 是一个电气级 TTL 门的运行态模型。它自身**不持有跨步状态**：全部状态都在
// 元件实例的 NodeValue 槽里，跨步可回滚；同一模型可服务任意多个实例。
type GateModel struct {
	params  GateParams
	level   digital.Level
	ronHigh float64
	ronLow  float64
	out     GateOutput
	slots   GateSlots
	inputs  []int
	load    digital.InputLoad
}

// NewGateModel 组装一个门模型：校验参数、按方向反推串联电阻，并把槽位/引脚映射固化下来。
// inputPins 是参与输入负载折线的引脚下标（可为零个）。
func NewGateModel(params GateParams, vcc float64, out GateOutput, slots GateSlots, inputPins ...int) (GateModel, error) {
	ronHigh, err := params.RonHigh(vcc)
	if err != nil {
		return GateModel{}, err
	}
	ronLow, err := params.RonLow(vcc)
	if err != nil {
		return GateModel{}, err
	}
	if slots.QueueBase < 0 || slots.Goal < 0 || slots.Drive < 0 || slots.Stamp < 0 {
		return GateModel{}, fmt.Errorf("ttl: 型号 %q 的槽位下标不得为负（%+v）", params.Model, slots)
	}
	return GateModel{
		params:  params,
		level:   digital.Level{VHigh: vcc, VLow: 0, Threshold: vcc / 2},
		ronHigh: ronHigh,
		ronLow:  ronLow,
		out:     out,
		slots:   slots,
		inputs:  append([]int(nil), inputPins...),
		load:    digital.InputLoad{IIH: params.IIH, IIL: params.IIL},
	}, nil
}

// StepLimit 返回本模型要求的步长上界：两个方向标称延时中较小者的 1/10（K1 的 StepGuard 口径）。
func (g GateModel) StepLimit() float64 {
	return math.Min(g.params.TpdLH, g.params.TpdHL) / 10
}

// RonHigh 返回拉高方向的串联电阻（供器件层/测试核对反推结果）。
func (g GateModel) RonHigh() float64 { return g.ronHigh }

// RonLow 返回灌低方向的串联电阻（供器件层/测试核对反推结果）。
func (g GateModel) RonLow() float64 { return g.ronLow }

// Ron 返回两个方向中更大的那个串联电阻（整体保守值）。
func (g GateModel) Ron() float64 { return math.Max(g.ronHigh, g.ronLow) }

// Step 推进一步：步长自检 → 惯性延时状态机（先 Apply 再排程）→ 盖章。
//
// 调用约定（K1）：
//   - 每步（每次元件迭代）都要调用；状态推进按仿真时刻幂等（同一时刻只推进一次）；
//   - 步长超界时**自己调 t.NoConverged()**（K1 硬约束②）并返回错误，由器件层决定是否记录；
//   - 本函数不发触发点（元件侧零 SetTriggers），时间语义只经 mna.Time。
//
// wantHigh 是器件层算出的目标逻辑电平（本模型不做逻辑判决）。
func (g GateModel) Step(m mna.Mna, t mna.Time, value element.NodeFace, wantHigh bool) error {
	if err := digital.StepGuard(t, g.StepLimit()); err != nil {
		t.NoConverged()
		return fmt.Errorf("ttl: 门模型 %s: %w", g.params.Model, err)
	}
	if value == nil {
		return fmt.Errorf("ttl: 门模型 %s: 元件节点为 nil", g.params.Model)
	}
	now := t.Time()
	if digital.SinceLastStamp(value, g.slots.Stamp, now) != 0 {
		if err := g.advance(m, t, value, wantHigh, now); err != nil {
			return err
		}
		digital.MarkStamp(value, g.slots.Stamp, now)
	}
	// 每轮迭代都要盖章：本模型的贡献（电压源、Ron、输入负载）不入线性基线，
	// 每次 doStep 前 A/Z 都会回滚，所以必须随迭代重盖（先例：Diode 在 DoStep 内盖章）。
	g.stamp(m, value)
	return nil
}

// advance 是本时刻的**唯一一次**状态推进：先 Apply 到期翻转，再按需要排新翻转（K1 顺序）。
func (g GateModel) advance(m mna.Mna, t mna.Time, value element.NodeFace, wantHigh bool, now float64) error {
	base := g.slots.QueueBase
	armedBefore := value.GetFloat64(base+GATE_QUEUE_ARMED) != 0
	pending := &digital.DelayQueue{}
	pending.Apply(m, t, value, base, value.GetVoltSource(g.out.Source))
	if armedBefore && value.GetFloat64(base+GATE_QUEUE_ARMED) == 0 {
		// 本步到点：排程时的目标电平已经写进电压源，把它提交为当前驱动电平。
		g.setDrive(value, g.goalHigh(value))
	}
	if wantHigh == g.goalHigh(value) {
		return nil
	}
	tpd := g.params.TpdHL
	if wantHigh {
		tpd = g.params.TpdLH
	}
	if err := g.scheduleEdge(m, value, now+tpd, g.driveVolt(wantHigh)); err != nil {
		return fmt.Errorf("ttl: 门模型 %s: %w", g.params.Model, err)
	}
	g.setGoal(value, wantHigh)
	return nil
}

// scheduleEdge 把一次目标电平翻转按**惯性延时**语义覆盖式排入延时队列（短于 tpd 的脉冲会被吞掉）。
// at 为生效时刻，levelVolt 为生效后写进输出理想电压源的电平。
func (g GateModel) scheduleEdge(m mna.Mna, value element.NodeFace, at, levelVolt float64) error {
	queue := &digital.DelayQueue{}
	return queue.PlanReplace(value, g.slots.QueueBase, at, levelVolt)
}

// stamp 加盖本模型在当前迭代下的全部电学贡献：
// 输出理想电压源（0/VCC，取当前驱动电平）、输出串联电阻 Ron、各输入引脚的 IIH/IIL 折线导纳。
func (g GateModel) stamp(m mna.Mna, value element.NodeFace) {
	internal := value.GetNodesInternal(g.out.Internal)
	pin := value.GetNodes(g.out.Pin)
	m.StampVoltageSource(internal, mna.Gnd, value.GetVoltSource(g.out.Source), g.driveVolt(g.driveHigh(value)))
	m.StampImpedance(internal, pin, g.ronForVoltage(g.driveHigh(value), m.GetNodeVoltage(pin)))
	for _, idx := range g.inputs {
		node := value.GetNodes(idx)
		m.StampAdmittance(node, mna.Gnd, g.load.ConductanceAt(m.GetNodeVoltage(node), g.level.Threshold))
	}
}

// driveVolt 把逻辑电平折算成理想电压源的电平（高 → VCC，低 → 0）。
func (g GateModel) driveVolt(high bool) float64 {
	if high {
		return g.level.VHigh
	}
	return g.level.VLow
}

// ronHighShort 是**拉高方向在低输出电压下的串联电阻下限**，取值依据是 LS TTL 图腾柱输出级的
// 结构（上管为达林顿、经约 120 Ω 电阻接 VCC）——**这是模型约定，不是数据手册参数**：
// 本机取源受限（TI PDF 无法解析、alldatasheet HTML 返回 403），故不把它写进 spec.go 的参数表
// （参数表只放手册数值，C11）。登记为未决项 V9，取到 iOS（短路输出电流）后改用 iOS 反推：
// ronShort = VCC / IOS。
//
// 为什么必须有它（电路级测试实测的缺陷 D28）：只有一个线性 RonHigh=(VCC-VOH)/IOH≈4 kΩ 时，
// 一个输出带 4 个 LS 输入会被拉到 0.293 V 并**自锁在低侧**（负载 4×IIL=4 mS 压过驱动器 1.2 mA），
// 而真实 LS 在低输出电压下能灌几十 mA —— 这正是「扇出 8」成立的原因。
const ronHighShort = 120.0

// ronForVoltage 按驱动方向与**当前输出电压**取串联电阻。
// 灌低方向恒为 RonLow（低侧本来就按 IOL=8 mA 标定，已经够硬）；拉高方向在输出电压接近 VOH 时
// 用 RonHigh（与手册的 VOH@IOH 一致），输出电压越低越接近饱和档 ronHighShort，按 Vout/VOH
// **连续**插值（分段跳变会让牛顿迭代在两点间来回跳，故必须连续）。
func (g GateModel) ronForVoltage(high bool, vOut float64) float64 {
	if !high {
		return g.ronLow
	}
	ratio := vOut / g.params.VOH
	switch {
	case !(ratio > 0): // 含 NaN 与 ≤0
		ratio = 0
	case ratio > 1:
		ratio = 1
	}
	ron := g.ronHigh * ratio
	if ron < ronHighShort {
		ron = ronHighShort
	}
	return ron
}

// ronFor 按驱动方向取串联电阻（忽略输出电压，供不读取节点电压的场景/测试使用）。
func (g GateModel) ronFor(high bool) float64 {
	if high {
		return g.ronHigh
	}
	return g.ronLow
}

// driveHigh 读当前驱动电平（0 = 低，其余 = 高）。
func (g GateModel) driveHigh(value element.NodeFace) bool {
	return value.GetFloat64(g.slots.Drive) != 0
}

// goalHigh 读目标电平（最近一次排程/判决的电平）。
func (g GateModel) goalHigh(value element.NodeFace) bool {
	return value.GetFloat64(g.slots.Goal) != 0
}

// setDrive 写当前驱动电平。
func (g GateModel) setDrive(value element.NodeFace, high bool) {
	value.SetFloat64(g.slots.Drive, boolToLevel(high))
}

// setGoal 写目标电平。
func (g GateModel) setGoal(value element.NodeFace, high bool) {
	value.SetFloat64(g.slots.Goal, boolToLevel(high))
}

// boolToLevel 把逻辑电平折算成槽位值（1 = 高，0 = 低）。
func boolToLevel(high bool) float64 {
	if high {
		return 1
	}
	return 0
}
