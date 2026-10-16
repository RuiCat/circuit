// uart_test.go：UART 元件的单元用例——步长自检/幂等门/ACK 清除/接收解码与帧错误/Reset。
//
// 这里用替身（stubTime/stubMna）直接驱动元件，覆盖引擎不便构造的边界：
// 超界步长、同一仿真时刻的重复 DoStep、被强行拉低的停止位。
package serial

import (
	"math"
	"testing"

	"circuit/element"
	"circuit/load/ast"
	"circuit/mna"
)

// timeStubBase 承载 mna.Time 其余（本包不调用的）方法；直接嵌入 mna.Time 会与 stubTime
// 自己声明的 Time() 方法重名。
type timeStubBase struct{ mna.Time }

// stubTime 是只实现 Time/CurrentTime/CurrentStep/NoConverged 的 mna.Time 替身。
// NoConverged 按引擎语义把步长减半，用于验证元件确实参与了「减半步长」这条回路。
type stubTime struct {
	timeStubBase
	now    float64
	step   float64
	noConv int
}

// Time 返回替身当前仿真时间。
func (s *stubTime) Time() float64 { return s.now }

// CurrentTime 返回替身当前仿真时间（DelayQueue 用它记录基准时刻）。
func (s *stubTime) CurrentTime() float64 { return s.now }

// CurrentStep 返回替身步长（StepGuard 用它判定是否超界）。
func (s *stubTime) CurrentStep() float64 { return s.step }

// NoConverged 记录未收敛次数并按引擎语义把步长减半。
func (s *stubTime) NoConverged() {
	s.noConv++
	s.step /= 2
}

// stubMna 是只实现节点读取与电压源写入的 mna.Mna 替身。
type stubMna struct {
	mna.Mna
	nodeV   map[mna.NodeID]float64    // 节点电压（rx 波形由测试直接给）。
	vs      map[mna.VoltageID]float64 // UpdateVoltageSource 写入值（元件输出）。
	stamped map[mna.VoltageID]float64 // StampVoltageSource 记录。
}

// newStubMna 建一个空替身。
func newStubMna() *stubMna {
	return &stubMna{
		nodeV:   map[mna.NodeID]float64{},
		vs:      map[mna.VoltageID]float64{},
		stamped: map[mna.VoltageID]float64{},
	}
}

// GetNodeVoltage 返回测试设定的节点电压。
func (s *stubMna) GetNodeVoltage(id mna.NodeID) float64 { return s.nodeV[id] }

// UpdateVoltageSource 记录元件写入的电平（DriveNow 的唯一出口）。
func (s *stubMna) UpdateVoltageSource(id mna.VoltageID, v float64) { s.vs[id] = v }

// StampVoltageSource 记录输出引脚的电压源加盖。
func (s *stubMna) StampVoltageSource(n1, n2 mna.NodeID, id mna.VoltageID, v float64) {
	s.stamped[id] = v
}

// uartTestPins 是替身电路里的引脚节点：tx=0、rx=1、intr=2。
var uartTestPins = []mna.NodeID{0, 1, 2}

// newUartNode 走与 load 相同的实例化路径：Base → 槽位初始化 → OrigValue 登记 → 电压源分配。
func newUartNode(t *testing.T, instanceName string, values []ast.Value) *element.Node {
	t.Helper()
	face, ok := element.ElementList[UartType]
	if !ok {
		t.Fatalf("UART 未注册（NodeType %d）", UartType)
	}
	cfg := face.Base(ast.ElementNode{Type: "UART", ID: "1", InstanceName: instanceName, Values: values})
	node := &element.Node{
		ConfigPtr:    cfg,
		NodeType:     UartType,
		InstanceName: instanceName,
		NodeValue:    make([]any, cfg.ValueNum()),
		OrigValue:    make(map[int]any),
		Nodes:        make([]mna.NodeID, cfg.PinNum()),
		VoltSource:   make([]mna.VoltageID, cfg.VoltageNum()),
		NodeInternal: make([]mna.NodeID, cfg.InternalNum()),
	}
	copy(node.NodeValue, cfg.ValueInit)
	for _, i := range cfg.OrigValue {
		node.OrigValue[i] = cfg.ValueInit[i]
	}
	copy(node.Nodes, uartTestPins)
	for i := range node.VoltSource {
		node.VoltSource[i] = mna.VoltageID(i)
	}
	return node
}

// doUartStep 经注册表分发 DoStep，与引擎 CallMark(MarkDoStep) 走同一条路径。
func doUartStep(m mna.Mna, t mna.Time, node element.NodeFace) {
	element.ElementList[UartType].DoStep(m, t, node)
}

// uartPeriod 是测试用的比特周期（9600 baud）。
const uartPeriod = 1.0 / 9600

// TestUartStepGuardNoConvergedHalvesStep 覆盖判据⑦：步长超界时元件必须自己调 NoConverged
// 并让引擎把步长减半；超界时不推进状态，步长合规后才推进。
func TestUartStepGuardNoConvergedHalvesStep(t *testing.T) {
	node := newUartNode(t, "UART1", nil)
	m := newStubMna()
	tt := &stubTime{step: 1.0} // 远大于 T/10 ≈ 1.04e-4
	node.SetFloat64(slotTxByte, 0x5A)

	doUartStep(m, tt, node)
	if tt.noConv == 0 {
		t.Fatal("步长超界必须调用 NoConverged()，否则引擎不会减半步长（K1 约定②）")
	}
	if tt.step != 0.5 {
		t.Fatalf("替身应因 NoConverged 把步长减半，实际 %v", tt.step)
	}
	if got := node.GetFloat64(slotTxBit); got != txBitIdle {
		t.Fatalf("步长超界时不得推进发送状态，txBit = %v", got)
	}

	tt.step = 1e-5 // 落在 T/10 内
	doUartStep(m, tt, node)
	if tt.noConv != 1 {
		t.Fatalf("步长合规后不应再报未收敛，noConv = %d", tt.noConv)
	}
	if got := node.GetFloat64(slotTxBit); got != 1 {
		t.Fatalf("步长合规后应发出起始位并进入位 1，txBit = %v", got)
	}
	if got := m.vs[mna.VoltageID(volTx)]; got != 0 {
		t.Fatalf("起始位必须把 tx 拉低，实际 %v V", got)
	}
}

// TestUartIdempotentStep 覆盖判据④：同一仿真时刻多次 DoStep 不得二次推进，
// 也不得因重复排程触发 NoConverged；推进一个比特周期后必须前进一位。
func TestUartIdempotentStep(t *testing.T) {
	node := newUartNode(t, "UART1", nil)
	m := newStubMna()
	tt := &stubTime{step: 1e-5}
	node.SetFloat64(slotTxByte, 0x5A)

	doUartStep(m, tt, node)
	first := node.GetFloat64(slotTxBit)
	doUartStep(m, tt, node)
	doUartStep(m, tt, node)
	if got := node.GetFloat64(slotTxBit); got != first {
		t.Fatalf("同一仿真时刻不得二次推进：txBit %v → %v", first, got)
	}
	if tt.noConv != 0 {
		t.Fatalf("幂等短路不应触发 NoConverged（重复排程被忽略即失败），noConv = %d", tt.noConv)
	}

	tt.now = uartPeriod
	doUartStep(m, tt, node)
	if got := node.GetFloat64(slotTxBit); got != 2 {
		t.Fatalf("推进一个比特周期后应进入位 2，txBit = %v", got)
	}
	if tt.noConv != 0 {
		t.Fatalf("位边沿排程不应报错，noConv = %d", tt.noConv)
	}
}

// TestUartAckClearsDataReady 覆盖判据②的 ACK 分支：槽 15 非 0 ⇒ 本步内 dataReady=false、
// ackFlag=0，且 intr 回落到 0V。
func TestUartAckClearsDataReady(t *testing.T) {
	node := newUartNode(t, "UART1", nil)
	m := newStubMna()
	m.nodeV[uartTestPins[pinRx]] = 5 // 空闲线为高，避免误起帧
	tt := &stubTime{step: 1e-5}
	node.SetBool(slotDataReady, true)
	node.SetFloat64(slotAckFlag, 1)

	doUartStep(m, tt, node)
	if node.GetBool(slotDataReady) {
		t.Fatal("槽 15 非 0 时本步内 dataReady 必须清零")
	}
	if got := node.GetFloat64(slotAckFlag); got != 0 {
		t.Fatalf("ACK 处理后 ackFlag 必须清零，实际 %v", got)
	}
	if got := m.vs[mna.VoltageID(volIntr)]; got != 0 {
		t.Fatalf("ACK 后 intr 必须回落到 0V，实际 %v V", got)
	}
}

// uartFrameBits 生成 8N1 帧的位序列：起始位低、8 数据位 LSB 先、停止位由 stopHigh 决定。
func uartFrameBits(b byte, stopHigh bool) []float64 {
	bits := make([]float64, 0, 10)
	bits = append(bits, 0)
	for i := 0; i < 8; i++ {
		bits = append(bits, float64((int(b)>>uint(i))&1))
	}
	stop := 0.0
	if stopHigh {
		stop = 1
	}
	return append(bits, stop)
}

// feedUartWave 以 T/10 的步长把位序列喂给元件（upto 为本次喂到的仿真时刻上界），
// 返回最后一次 DoStep 的替身时间。
func feedUartWave(m *stubMna, tt *stubTime, node *element.Node, bits []float64, upto float64) {
	step := uartPeriod / 10
	for i := 0; float64(i)*step <= upto; i++ {
		tt.step = step
		tt.now = float64(i) * step
		bit := int(tt.now / uartPeriod)
		if bit >= len(bits) {
			bit = len(bits) - 1
		}
		m.nodeV[uartTestPins[pinRx]] = bits[bit] * 5
		doUartStep(m, tt, node)
	}
}

// TestUartRxDecodesFrame 覆盖判据①的元件级最小版：tx 位流（此处直接喂 rx）解出同一字节。
func TestUartRxDecodesFrame(t *testing.T) {
	for _, want := range []byte{0x00, 0x5A, 0xFF} {
		node := newUartNode(t, "UART1", nil)
		m := newStubMna()
		tt := &stubTime{}
		feedUartWave(m, tt, node, uartFrameBits(want, true), rxStopOffset*uartPeriod)

		if !node.GetBool(slotDataReady) {
			t.Fatalf("字节 %#02x 收完应置 dataReady", want)
		}
		if got := node.GetFloat64(slotRxByte); got != float64(want) {
			t.Fatalf("接收字节不符：想要 %#02x，得到 %#02x", want, int(got))
		}
		if node.GetBool(slotFrameError) {
			t.Fatalf("停止位为高时不得置 frameError（字节 %#02x）", want)
		}
		if got := m.vs[mna.VoltageID(volIntr)]; got != 5 {
			t.Fatalf("dataReady=true 时 intr 必须钳位 VHigh，实际 %v V", got)
		}
	}
}

// TestUartRxFrameErrorOnLowStop 覆盖判据③：停止位被采样为低 ⇒ frameError=true，
// 数据位仍照常解出（不丢字节、不 panic）。
func TestUartRxFrameErrorOnLowStop(t *testing.T) {
	node := newUartNode(t, "UART1", nil)
	m := newStubMna()
	tt := &stubTime{}
	feedUartWave(m, tt, node, uartFrameBits(0xFF, false), rxStopOffset*uartPeriod)

	if !node.GetBool(slotFrameError) {
		t.Fatal("停止位采样为低必须置 frameError=true")
	}
	if !node.GetBool(slotDataReady) {
		t.Fatal("帧错误不应吞掉字节：dataReady 仍应置位")
	}
	if got := node.GetFloat64(slotRxByte); got != 0xFF {
		t.Fatalf("帧错误下数据位仍应解出 0xFF，实际 %#02x", int(got))
	}
}

// TestUartResetClearsRuntimeSlots 覆盖判据⑤的元件级最小版：Reset 必须复位全部运行态槽
// （含事件落点 3/15 与幂等戳），并同步刷新 OrigValue；复位后重放同一波形得到同一结果。
func TestUartResetClearsRuntimeSlots(t *testing.T) {
	node := newUartNode(t, "UART1", nil)
	m := newStubMna()
	tt := &stubTime{}
	feedUartWave(m, tt, node, uartFrameBits(0xFF, false), rxStopOffset*uartPeriod)

	element.ElementList[UartType].Reset(node)
	if got := node.GetFloat64(slotTxBit); got != txBitIdle {
		t.Fatalf("Reset 后 txBit 应回到空闲 %d，实际 %v", txBitIdle, got)
	}
	if got := node.GetFloat64(slotTxByte); got != txByteNone {
		t.Fatalf("Reset 后 txByte 应回到 %v，实际 %v", txByteNone, got)
	}
	if got := node.GetFloat64(slotRxPhase); got != 0 {
		t.Fatalf("Reset 后 rxPhase 应回到 0，实际 %v", got)
	}
	if node.GetBool(slotDataReady) || node.GetBool(slotFrameError) {
		t.Fatal("Reset 后 dataReady/frameError 必须清零")
	}
	if got := node.GetFloat64(slotAckFlag); got != 0 {
		t.Fatalf("Reset 后 ackFlag 必须清零，实际 %v", got)
	}
	if got := node.GetFloat64(slotLastStepTime); !math.IsNaN(got) {
		t.Fatalf("Reset 后幂等戳必须回到 NaN，实际 %v", got)
	}
	if got, ok := node.OrigValue[slotLastStepTime].(float64); !ok || !math.IsNaN(got) {
		t.Fatalf("Reset 必须同步刷新 OrigValue 的幂等戳，实际 %v", node.OrigValue[slotLastStepTime])
	}
	if got := node.GetFloat64(slotLastTxTime); !math.IsInf(got, -1) {
		t.Fatalf("Reset 后 lastTxTime 应回到 -Inf，实际 %v", got)
	}

	// 复位后重放同一波形：必须得到同一结果（否则同一 Context 第二次运行会不一致）。
	tt2 := &stubTime{}
	feedUartWave(m, tt2, node, uartFrameBits(0xFF, false), rxStopOffset*uartPeriod)
	if !node.GetBool(slotFrameError) {
		t.Fatal("Reset 后重放同一波形必须复现 frameError")
	}
	if got := node.GetFloat64(slotRxByte); got != 0xFF {
		t.Fatalf("Reset 后重放应解出 0xFF，实际 %#02x", int(got))
	}
}
