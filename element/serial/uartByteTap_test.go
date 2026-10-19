// uartByteTap_test.go：K1 双形态（3/13 针）的配置、D/RDY/ACK 4 相握手与 3 针不变性用例。
//
// 与 uart_test.go 共用 stubTime/stubMna；这里只补 13 针形态需要的多引脚实例化路径。
package serial

import (
	"os"
	"testing"

	"circuit/element"
	_ "circuit/element/passive" // 68 网表的 R1 需要 Resistor 已注册
	"circuit/load"
	"circuit/load/ast"
	"circuit/mna"
)

// newUartPinsNode 按引脚数走与 load 相同的实例化路径：Base → 槽位初始化 → OrigValue
// 登记 → 引脚/电压源分配（引脚 i 连节点 i、电压源 i 用 ID i，便于断言）。
func newUartPinsNode(t *testing.T, pinCount int) *element.Node {
	t.Helper()
	face, ok := element.ElementList[UartType]
	if !ok {
		t.Fatalf("UART 未注册（NodeType %d）", UartType)
	}
	elem := ast.ElementNode{
		Type:         "UART",
		ID:           "1",
		InstanceName: "UART1",
		Pins:         make([]ast.Value, pinCount),
	}
	cfg := face.Base(elem)
	node := &element.Node{
		ConfigPtr:    cfg,
		NodeType:     UartType,
		InstanceName: "UART1",
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
	for i := range node.Nodes {
		node.Nodes[i] = mna.NodeID(i)
	}
	for i := range node.VoltSource {
		node.VoltSource[i] = mna.VoltageID(i)
	}
	return node
}

// uartInstanceByName 在加载后的上下文中按实例名找 UART 节点。
func uartInstanceByName(t *testing.T, con *element.Context, name string) element.NodeFace {
	t.Helper()
	for _, n := range con.Nodelist {
		if n.Base() != nil && n.Base().InstanceName == name {
			return n
		}
	}
	t.Fatalf("网表里找不到实例 %q", name)
	return nil
}

// TestUartConfigByPinCount 覆盖 C2：3 针 = 3 引脚/2 电压源、13 针 = 13 引脚/11 电压源，
// 两形态都能被 load 实例化；3 针实例不得出现 D/RDY 电压源。
func TestUartConfigByPinCount(t *testing.T) {
	cases := []struct {
		name       string
		pins       int
		wantPin    int
		wantVolt   int
		wantStamps int
	}{
		{name: "threePins", pins: 3, wantPin: 3, wantVolt: 2, wantStamps: 2},
		{name: "thirteenPins", pins: 13, wantPin: 13, wantVolt: 11, wantStamps: 11},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			node := newUartPinsNode(t, c.pins)
			if got := node.Config().PinNum(); got != c.wantPin {
				t.Fatalf("PinNum = %d，想要 %d", got, c.wantPin)
			}
			if got := node.Config().VoltageNum(); got != c.wantVolt {
				t.Fatalf("VoltageNum = %d，想要 %d", got, c.wantVolt)
			}
			if len(node.Nodes) != c.wantPin || len(node.VoltSource) != c.wantVolt {
				t.Fatalf("实例分配与 Config 不成对：Nodes=%d VoltSource=%d",
					len(node.Nodes), len(node.VoltSource))
			}
			m := newStubMna()
			element.ElementList[UartType].Stamp(m, &stubTime{}, node)
			if len(m.stamped) != c.wantStamps {
				t.Fatalf("Stamp 加盖 %d 个电压源，想要 %d（3 针只盖 tx/intr）", len(m.stamped), c.wantStamps)
			}
			if _, ok := m.stamped[mna.VoltageID(volRdy)]; ok != (c.pins == 13) {
				t.Fatalf("RDY 电压源加盖状态与形态不符：pins=%d stamped=%v", c.pins, ok)
			}
		})
	}

	// 成对切换的反证面：3 针形态的 Voltage 表里不得出现 D0/RDY。
	for _, name := range []string{"D0", "D7", "RDY"} {
		for _, v := range newUartPinsNode(t, 3).Config().Voltage {
			if v == name {
				t.Fatalf("3 针形态不得分配 %s 电压源（会把未定位脚钉到 0 节点）", name)
			}
		}
	}
}

// TestUartConfigByPinCountLoadsNetlists 验证 68 三针网表与新 13 针网表都能被 load 实例化，
// 且加载后的 PinNum/VoltageNum 与 K1 一致（load.go 的引脚数校验与 VoltSource 分配路径）。
func TestUartConfigByPinCountLoadsNetlists(t *testing.T) {
	raw, err := os.ReadFile("../../cmd/circuits/68_ttl_uart_loopback.net")
	if err != nil {
		t.Fatalf("读取 68 网表失败: %v", err)
	}
	three, err := load.LoadString(string(raw))
	if err != nil {
		t.Fatalf("加载 68 网表失败: %v", err)
	}
	node3 := uartInstanceByName(t, three, "UART1")
	if node3.Config().PinNum() != 3 || node3.Config().VoltageNum() != 2 {
		t.Fatalf("68 的 UART1 应为 3 针/2 源，实际 %d/%d",
			node3.Config().PinNum(), node3.Config().VoltageNum())
	}

	const tapNetlist = `
UART1 [1,1,2,3,4,5,6,7,8,9,10,11,12] [9600]
R1 [12,-1] [100000]
`
	thirteen, err := load.LoadString(tapNetlist)
	if err != nil {
		t.Fatalf("加载 13 针网表失败: %v", err)
	}
	node13 := uartInstanceByName(t, thirteen, "UART1")
	if node13.Config().PinNum() != 13 || node13.Config().VoltageNum() != 11 {
		t.Fatalf("13 针网表的 UART1 应为 13 针/11 源，实际 %d/%d",
			node13.Config().PinNum(), node13.Config().VoltageNum())
	}
}

// uartDriveD 返回 D(i) 电压源当前电平（stub 只记录 UpdateVoltageSource 写入值）。
func uartDriveD(m *stubMna, i int) float64 {
	return m.vs[mna.VoltageID(volD0+i)]
}

// TestUartByteTap 覆盖 K1 的 13 针不变式：取字节 ⇒ D 锁存 + RDY 高；
// ACK 高 ⇒ RDY 回低且置「已 ACK」；ACK 高期间不得取下一字节；ACK 回低后才取。
func TestUartByteTap(t *testing.T) {
	const step = uartPeriod / 10
	node := newUartPinsNode(t, 13)
	m := newStubMna()
	m.nodeV[node.GetNodes(pinRx)] = 5 // 空闲线为高，避免误起帧
	ackNode := node.GetNodes(pinAck)
	tt := &stubTime{step: step}
	node.SetFloat64(slotTxByte, 0xA5)

	doUartStep(m, tt, node) // t=0：取字节 → 起始位、D 锁存、RDY 高
	if got := node.GetFloat64(slotTxByte); got != txByteNone {
		t.Fatalf("t=0 应取走待发字节，槽 3 = %v", got)
	}
	if got := node.GetFloat64(slotByteLatch); got != 0xA5 {
		t.Fatalf("D 锁存字节应为 0xA5，实际 %#02x", int(got))
	}
	if got := m.vs[mna.VoltageID(volRdy)]; got != 5 {
		t.Fatalf("取字节后 RDY 必须为 VHigh，实际 %v V", got)
	}
	for i := 0; i < 8; i++ {
		want := 0.0
		if (0xA5>>uint(i))&1 == 1 {
			want = 5
		}
		if got := uartDriveD(m, i); got != want {
			t.Fatalf("D%d = %v V，想要 %v V（字节 0xA5）", i, got, want)
		}
	}

	// 加载器看到 RDY 高后拉 ACK：下一步 UART 必须拉低 RDY，并记录槽 22 = 1。
	m.nodeV[ackNode] = 5
	tt.now += step
	doUartStep(m, tt, node)
	if got := node.GetFloat64(slotByteRdy); got != 0 {
		t.Fatalf("ACK 为高时 RDY 内部状态应回低，实际 %v", got)
	}
	if got := node.GetFloat64(slotByteAck); got != 1 {
		t.Fatalf("ACK 为高时必须记录「加载器已 ACK」（槽 22 = 1），实际 %v", got)
	}
	if got := m.vs[mna.VoltageID(volRdy)]; got != 0 {
		t.Fatalf("ACK 为高时 RDY 引脚必须回低，实际 %v V", got)
	}
	if got := node.GetFloat64(slotByteLatch); got != 0xA5 {
		t.Fatalf("RDY 高到 ACK 低期间 D 不得变化，实际 %#02x", int(got))
	}

	// ACK 仍为高：送入下一字节，帧发完后也不得取走（D 保持 0xA5）。
	node.SetFloat64(slotTxByte, 0x3C)
	for i := 0; i < 200; i++ {
		tt.now += step
		doUartStep(m, tt, node)
	}
	if got := node.GetFloat64(slotTxByte); got != 0x3C {
		t.Fatalf("ACK 回低前不得从槽 3 取下一字节，槽 3 = %v", got)
	}
	if got := node.GetFloat64(slotByteLatch); got != 0xA5 {
		t.Fatalf("ACK 回低前 D 必须保持 0xA5，实际 %#02x", int(got))
	}
	for i := 0; i < 8; i++ {
		want := 0.0
		if (0xA5>>uint(i))&1 == 1 {
			want = 5
		}
		if got := uartDriveD(m, i); got != want {
			t.Fatalf("ACK 回低前 D%d 不得变化：%v V，想要 %v V", i, got, want)
		}
	}

	// ACK 回低：下一步即可取下一字节，D 换成 0x3C、RDY 重新拉高。
	m.nodeV[ackNode] = 0
	tt.now += step
	doUartStep(m, tt, node)
	if got := node.GetFloat64(slotTxByte); got != txByteNone {
		t.Fatalf("ACK 回低后应取走下一字节，槽 3 = %v", got)
	}
	if got := node.GetFloat64(slotByteLatch); got != 0x3C {
		t.Fatalf("下一字节应锁存到 D，锁存值 %#02x", int(got))
	}
	if got := node.GetFloat64(slotByteRdy); got != 1 {
		t.Fatalf("取下一字节后 RDY 必须重新拉高，内部状态 %v", got)
	}
	if got := m.vs[mna.VoltageID(volRdy)]; got != 5 {
		t.Fatalf("取下一字节后 RDY 引脚必须为 VHigh，实际 %v V", got)
	}
}

// TestUartAckGate 覆盖 C1 的另一半：ACK 门控只作用于 13 针形态；3 针实例既不读 ACK 脚，
// 也不分配/驱动 D、RDY 电压源。
func TestUartAckGate(t *testing.T) {
	const step = uartPeriod / 10

	// 3 针：没有 ACK 脚可读，取字节不得被任何门控阻塞。
	node3 := newUartPinsNode(t, 3)
	if got := node3.Config().VoltageNum(); got != 2 {
		t.Fatalf("3 针形态 VoltageNum = %d，想要 2", got)
	}
	m3 := newStubMna()
	m3.nodeV[node3.GetNodes(pinRx)] = 5
	tt3 := &stubTime{step: step}
	node3.SetFloat64(slotTxByte, 0x11)
	doUartStep(m3, tt3, node3)
	if got := node3.GetFloat64(slotTxByte); got != txByteNone {
		t.Fatalf("3 针实例不得因 ACK 门控拒绝取字节，槽 3 = %v", got)
	}
	if got := node3.GetFloat64(slotByteRdy); got != 0 {
		t.Fatalf("3 针实例不得推进 13 针握手，slotByteRdy = %v", got)
	}
	if _, ok := m3.vs[mna.VoltageID(volD0)]; ok {
		t.Fatal("3 针实例不得驱动 D0 电压源")
	}
	if _, ok := m3.vs[mna.VoltageID(volRdy)]; ok {
		t.Fatal("3 针实例不得驱动 RDY 电压源")
	}

	// 13 针：ACK 初始为高（加载器未就绪）⇒ 不得取字节。
	node13 := newUartPinsNode(t, 13)
	m13 := newStubMna()
	m13.nodeV[node13.GetNodes(pinRx)] = 5
	ackNode := node13.GetNodes(pinAck)
	m13.nodeV[ackNode] = 5
	tt13 := &stubTime{step: step}
	node13.SetFloat64(slotTxByte, 0x22)
	doUartStep(m13, tt13, node13)
	if got := node13.GetFloat64(slotTxByte); got != 0x22 {
		t.Fatalf("13 针实例在 ACK 为高时不得取字节，槽 3 = %v", got)
	}
	if got := node13.GetFloat64(slotTxBit); got != txBitIdle {
		t.Fatalf("被 ACK 门控阻塞时发送侧应保持空闲，txBit = %v", got)
	}

	// ACK 回低后才取字节并拉高 RDY。
	m13.nodeV[ackNode] = 0
	tt13.now = step
	doUartStep(m13, tt13, node13)
	if got := node13.GetFloat64(slotTxByte); got != txByteNone {
		t.Fatalf("ACK 回低后应取字节，槽 3 = %v", got)
	}
	if got := node13.GetFloat64(slotByteRdy); got != 1 {
		t.Fatalf("取字节后 RDY 内部状态应为 1，实际 %v", got)
	}
}
