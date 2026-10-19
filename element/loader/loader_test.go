// loader_test.go：LOADER 控制器的单元用例——帧解码/长度上限/释放顺序/同刻幂等/CpuMode 参数化。
//
// 用替身（loaderTimeStub/loaderMnaStub）直接驱动元件：替身按引脚节点电压模拟 UART 的
// 4 相字节握手（RDY 高 → 加载器 ACK 高 → RDY 低 → ACK 低），并按 K1 的
// 10 个比特周期/字节节奏提供下一字节（等价于 UART 的取字节时机）。
package loader

import (
	"testing"

	"circuit/element"
	"circuit/load/ast"
	"circuit/mna"
)

// loaderTimeBase 承载 mna.Time 其余（本包不调用的）方法；直接嵌入 mna.Time 会与
// loaderTimeStub 自己声明的 Time() 方法重名。
type loaderTimeBase struct{ mna.Time }

// loaderTimeStub 是只实现 Time/CurrentTime/CurrentStep/NoConverged 的 mna.Time 替身。
type loaderTimeStub struct {
	loaderTimeBase
	now    float64
	step   float64
	noConv int
}

// Time 返回替身当前仿真时间。
func (s *loaderTimeStub) Time() float64 { return s.now }

// CurrentTime 返回替身当前仿真时间。
func (s *loaderTimeStub) CurrentTime() float64 { return s.now }

// CurrentStep 返回替身步长（StepGuard 用它判定是否超界）。
func (s *loaderTimeStub) CurrentStep() float64 { return s.step }

// NoConverged 记录未收敛次数（替身不实际减半步长，保持测试时间轴可复算）。
func (s *loaderTimeStub) NoConverged() { s.noConv++ }

// loaderMnaStub 是只实现节点读数与电压源写入的 mna.Mna 替身。
// UpdateVoltageSource 把元件输出电平落回它持有的输出节点，模拟电压源对节点的钳位。
type loaderMnaStub struct {
	mna.Mna
	nodeV   map[mna.NodeID]float64
	vsNode  map[mna.VoltageID]mna.NodeID
	vsValue map[mna.VoltageID]float64
}

// GetNodeVoltage 返回测试设定的节点电压。
func (s *loaderMnaStub) GetNodeVoltage(id mna.NodeID) float64 { return s.nodeV[id] }

// UpdateVoltageSource 记录元件写入的电平，并把对应节点钳位到该电平。
func (s *loaderMnaStub) UpdateVoltageSource(id mna.VoltageID, v float64) {
	s.vsValue[id] = v
	if node, ok := s.vsNode[id]; ok {
		s.nodeV[node] = v
	}
}

// StampVoltageSource 记录输出引脚的电压源加盖（本替身不求解，仅记录）。
func (s *loaderMnaStub) StampVoltageSource(n1, n2 mna.NodeID, id mna.VoltageID, v float64) {
	s.vsValue[id] = v
}

// 替身电路的节点分配：D0..D7 = 0..7、RDY = 8，输出引脚 = 100 起（LA/LD/LOAD/LWE/ACK）。
const (
	testRDYNode  mna.NodeID = 8
	testOutBase  mna.NodeID = 100
	testLOADNode mna.NodeID = testOutBase + 16
	testLWENode  mna.NodeID = testOutBase + 17
	testACKNode  mna.NodeID = testOutBase + 18
)

// testLANode 返回 LA 第 i 位的输出节点。
func testLANode(i int) mna.NodeID { return testOutBase + mna.NodeID(i) }

// testLDNode 返回 LD 第 i 位的输出节点。
func testLDNode(i int) mna.NodeID { return testOutBase + 8 + mna.NodeID(i) }

// testStep 是测试用的仿真步长，等于 StepGuard 缺省上界（9600 baud 的 T/10）。
const testStep = 1.0 / (9600 * 10)

// newLoaderNode 走与 load 相同的实例化路径：Base → 槽位初始化 → OrigValue 登记 → 引脚/电压源分配。
func newLoaderNode(t *testing.T, instanceName string) *element.Node {
	t.Helper()
	face, ok := element.ElementList[LoaderType]
	if !ok {
		t.Fatalf("LOADER 未注册（NodeType %d）", LoaderType)
	}
	cfg := face.Base(ast.ElementNode{Type: "LOADER", ID: "1", InstanceName: instanceName})
	node := &element.Node{
		ConfigPtr:    cfg,
		NodeType:     LoaderType,
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
	for i := 0; i < cfg.PinNum(); i++ {
		if i <= pinRDY {
			node.Nodes[i] = mna.NodeID(i)
		} else {
			node.Nodes[i] = testOutBase + mna.NodeID(i-pinLA0)
		}
	}
	for v := 0; v < cfg.VoltageNum(); v++ {
		node.VoltSource[v] = mna.VoltageID(v)
	}
	return node
}

// newLoaderMnaStub 建替身，并把 LOADER 的每个输出电压源映射到对应输出节点。
func newLoaderMnaStub(node *element.Node) *loaderMnaStub {
	s := &loaderMnaStub{
		nodeV:   map[mna.NodeID]float64{},
		vsNode:  map[mna.VoltageID]mna.NodeID{},
		vsValue: map[mna.VoltageID]float64{},
	}
	for v := 0; v < node.Config().VoltageNum(); v++ {
		s.vsNode[node.VoltSource[v]] = node.Nodes[loaderVoltPin(v)]
	}
	return s
}

// loaderByteSource 模拟 K1 的字节口：按 10 个比特周期/字节的节奏给出字节，等 ACK 高后撤 RDY。
type loaderByteSource struct {
	bytes       []byte
	next        int
	framePeriod float64
	available   float64
	pending     bool
}

// update 在时间 now 驱动 D0..D7 与 RDY。
func (s *loaderByteSource) update(m *loaderMnaStub, now float64) {
	if s.next >= len(s.bytes) {
		m.nodeV[testRDYNode] = 0
		return
	}
	if !s.pending {
		if now < s.available {
			return
		}
		b := s.bytes[s.next]
		for i := 0; i < 8; i++ {
			m.nodeV[mna.NodeID(i)] = loaderBitLevel(int(b), i, defaultVHigh)
		}
		m.nodeV[testRDYNode] = defaultVHigh
		s.pending = true
		return
	}
	if m.nodeV[testACKNode] > defaultVHigh/2 {
		m.nodeV[testRDYNode] = 0
		s.pending = false
		s.next++
		s.available = now + s.framePeriod
	}
}

// loaderSample 是一个被接受仿真时刻的输出快照（用于断言写时序）。
type loaderSample struct {
	now  float64
	lwe  bool
	la   int
	ld   int
	load bool
}

// loaderSampleAt 读取当前时刻的 LA/LD/LWE/LOAD 输出节点电平。
func loaderSampleAt(m *loaderMnaStub, now float64) loaderSample {
	s := loaderSample{now: now, lwe: m.nodeV[testLWENode] > defaultVHigh/2, load: m.nodeV[testLOADNode] > defaultVHigh/2}
	for i := 0; i < 8; i++ {
		if m.nodeV[testLANode(i)] > defaultVHigh/2 {
			s.la |= 1 << uint(i)
		}
		if m.nodeV[testLDNode(i)] > defaultVHigh/2 {
			s.ld |= 1 << uint(i)
		}
	}
	return s
}

// loaderRun 是一次替身仿真的结果。
type loaderRun struct {
	node    *element.Node
	m       *loaderMnaStub
	steps   int
	samples []loaderSample
}

// runLoader 用固定步长与字节节奏驱动 LOADER（以及可选的 RAM），直到 done/error 或步数用尽。
func runLoader(t *testing.T, node, ram *element.Node, payload []byte, framePeriod, step float64, maxSteps int) *loaderRun {
	t.Helper()
	m := newLoaderMnaStub(node)
	if ram != nil {
		for v := 0; v < ram.Config().VoltageNum(); v++ {
			m.vsNode[ram.VoltSource[v]] = ram.Nodes[17+v]
		}
	}
	run := &loaderRun{node: node, m: m}
	tt := &loaderTimeStub{step: step}
	src := &loaderByteSource{bytes: payload, framePeriod: framePeriod}
	steps := 0
	for ; steps < maxSteps; steps++ {
		tt.now = float64(steps) * step
		src.update(m, tt.now)
		doLoaderStep(m, tt, node)
		if ram != nil {
			element.ElementList[ram.Type()].DoStep(m, tt, ram)
		}
		run.samples = append(run.samples, loaderSampleAt(m, tt.now))
		if node.GetFloat64(slotDone) != 0 || node.GetFloat64(slotError) != 0 {
			break
		}
	}
	run.steps = steps + 1
	return run
}

// doLoaderStep 经注册表分发 DoStep，与引擎 CallMark(MarkDoStep) 走同一条路径。
func doLoaderStep(m mna.Mna, t mna.Time, node element.NodeFace) {
	element.ElementList[LoaderType].DoStep(m, t, node)
}

// buildFrame 按 SYNC+LEN+DATA+CHK 组帧（CHK = (LEN + ΣDATA) & 0xFF）。
func buildFrame(payload []byte) []byte {
	frame := make([]byte, 0, len(payload)+3)
	frame = append(frame, frameSyncByte, byte(len(payload)))
	sum := len(payload)
	for _, b := range payload {
		frame = append(frame, b)
		sum += int(b)
	}
	return append(frame, byte(sum&checksumMask))
}

// assertWriteTiming 断言 C4 的写时序不变量：每个字节至少一个接受时刻 LWE=1 且 LA/LD 稳定；
// LWE=1 期间 LA/LD 不得变化；写出的数据必须与程序逐字节一致。
func assertWriteTiming(t *testing.T, samples []loaderSample, want []byte) {
	t.Helper()
	written := map[int]bool{}
	prev := loaderSample{}
	for i, s := range samples {
		if s.lwe && (s.la != prev.la || s.ld != prev.ld) {
			t.Fatalf("第 %d 个接受时刻 LWE=1 但 LA/LD 变化：(%d,%d)→(%d,%d)", i, prev.la, prev.ld, s.la, s.ld)
		}
		if s.lwe && s.la < len(want) {
			if s.ld != int(want[s.la]) {
				t.Fatalf("地址 %d 写入 %#02x，期望 %#02x", s.la, s.ld, want[s.la])
			}
			written[s.la] = true
		}
		prev = s
	}
	for addr := range want {
		if !written[addr] {
			t.Fatalf("地址 %d 没有「LWE=1 且 LA/LD 稳定」的接受时刻", addr)
		}
	}
}

// TestLoaderFrameDecode 覆盖 C2.1：合法帧逐字节写出；坏校验/非 SYNC 前导/超长/超短帧不进入加载。
func TestLoaderFrameDecode(t *testing.T) {
	prog := []byte{0x64, 0x2C, 0x09, 0x16}
	node := newLoaderNode(t, "LOADER1")
	run := runLoader(t, node, nil, buildFrame(prog), 5*testStep, testStep, 5000)
	if got := node.GetFloat64(slotError); got != 0 {
		t.Fatalf("合法帧不得置 error，实际 %v", got)
	}
	if got := node.GetFloat64(slotDone); got != 1 {
		t.Fatalf("合法帧应完成加载（done=1），实际 %v", got)
	}
	assertWriteTiming(t, run.samples, prog)

	// 坏校验：只置 error，不写 RAM、不释放 CPU。
	bad := buildFrame(prog)
	bad[len(bad)-1] ^= 0xFF
	node2 := newLoaderNode(t, "LOADER2")
	run2 := runLoader(t, node2, nil, bad, 5*testStep, testStep, 5000)
	if got := node2.GetFloat64(slotError); got != 1 {
		t.Fatalf("坏校验必须置 error=1，实际 %v", got)
	}
	if got := node2.GetFloat64(slotDone); got != 0 {
		t.Fatalf("坏校验不得置 done，实际 %v", got)
	}
	assertNoRamWrite(t, run2.samples)

	// 非 SYNC 前导字节应被静默忽略（流内重新同步），之后的合法帧照常加载。
	noisy := append([]byte{0x11, 0x22}, buildFrame(prog)...)
	node3 := newLoaderNode(t, "LOADER3")
	run3 := runLoader(t, node3, nil, noisy, 5*testStep, testStep, 5000)
	if got := node3.GetFloat64(slotDone); got != 1 {
		t.Fatalf("非 SYNC 前导后应能重新同步并完成加载，done=%v", got)
	}
	assertWriteTiming(t, run3.samples, prog)
}

// assertNoRamWrite 断言整段运行没有任何 LWE=1 的接受时刻（即没有写 RAM）。
func assertNoRamWrite(t *testing.T, samples []loaderSample) {
	t.Helper()
	for i, s := range samples {
		if s.lwe {
			t.Fatalf("第 %d 个接受时刻出现 LWE=1（失败帧不得写 RAM）", i)
		}
	}
}

// TestLoaderReleaseCpu 覆盖 C12：成功后 done=1、LOAD=0、LWE=0；释放顺序 LWE=0 → 至少一个
// 接受时刻 → LOAD=0；失败时 LOAD 保持高（CPU 不释放）。
func TestLoaderReleaseCpu(t *testing.T) {
	prog := []byte{0x01, 0x02}
	node := newLoaderNode(t, "LOADER1")
	run := runLoader(t, node, nil, buildFrame(prog), 5*testStep, testStep, 5000)
	if got := node.GetFloat64(slotDone); got != 1 {
		t.Fatalf("加载成功应置 done=1，实际 %v", got)
	}
	if got := run.m.nodeV[testLOADNode]; got != 0 {
		t.Fatalf("加载成功必须释放 CPU（LOAD=0），实际 %v V", got)
	}
	if got := run.m.nodeV[testLWENode]; got != 0 {
		t.Fatalf("加载成功后 LWE 必须为低，实际 %v V", got)
	}
	lastHigh, firstLow := -1, -1
	for i, s := range run.samples {
		if s.lwe {
			lastHigh = i
		}
		if !s.load && firstLow < 0 {
			firstLow = i
		}
	}
	if lastHigh < 0 || firstLow < 0 {
		t.Fatalf("未观察到 LWE 高窗口或 LOAD 释放：lastHigh=%d firstLow=%d", lastHigh, firstLow)
	}
	settled := false
	for i := lastHigh + 1; i < firstLow; i++ {
		if !run.samples[i].lwe && run.samples[i].load {
			settled = true
		}
	}
	if !settled {
		t.Fatal("释放顺序必须为 LWE=0 → 至少一个接受时刻 → LOAD=0")
	}

	// 失败帧：CPU 必须保持冻结。
	bad := buildFrame(prog)
	bad[len(bad)-1] ^= 0xFF
	node2 := newLoaderNode(t, "LOADER2")
	run2 := runLoader(t, node2, nil, bad, 5*testStep, testStep, 5000)
	if got := node2.GetFloat64(slotError); got != 1 {
		t.Fatalf("坏帧应置 error=1，实际 %v", got)
	}
	for i, s := range run2.samples {
		if !s.load {
			t.Fatalf("失败帧不得释放 CPU：第 %d 个接受时刻 LOAD=0", i)
		}
	}
}

// TestLoaderFrameLength 覆盖 C14：LENMax 是硬上限，超长/零长帧只置 error 且不写 RAM。
func TestLoaderFrameLength(t *testing.T) {
	prog := []byte{0xAA, 0xBB, 0xCC, 0xDD}
	node := newLoaderNode(t, "LOADER1")
	node.SetFloat64(slotLenMax, float64(len(prog)))
	ram := newRAMNode(t, "RAM1")
	run := runLoader(t, node, ram, buildFrame([]byte{0x11, 0x22, 0x33, 0x44, 0x55}), 5*testStep, testStep, 5000)
	if got := node.GetFloat64(slotError); got != 1 {
		t.Fatalf("超过 LenMax 的帧必须置 error=1，实际 %v", got)
	}
	if got := node.GetFloat64(slotDone); got != 0 {
		t.Fatalf("超长帧不得置 done，实际 %v", got)
	}
	assertNoRamWrite(t, run.samples)
	assertRAMZero(t, ram)

	// LEN=0 是超短帧。
	node2 := newLoaderNode(t, "LOADER2")
	run2 := runLoader(t, node2, nil, []byte{frameSyncByte, 0x00}, 5*testStep, testStep, 5000)
	if got := node2.GetFloat64(slotError); got != 1 {
		t.Fatalf("LEN=0 必须置 error=1，实际 %v", got)
	}
	assertNoRamWrite(t, run2.samples)

	// 恰好等于 LenMax 的帧应被接受。
	node3 := newLoaderNode(t, "LOADER3")
	node3.SetFloat64(slotLenMax, float64(len(prog)))
	run3 := runLoader(t, node3, nil, buildFrame(prog), 5*testStep, testStep, 5000)
	if got := node3.GetFloat64(slotDone); got != 1 {
		t.Fatalf("LEN == LenMax 应加载成功，done=%v error=%v", got, node3.GetFloat64(slotError))
	}
	assertWriteTiming(t, run3.samples, prog)
}

// TestLoaderIdempotent 覆盖 C17：同一 Time() 重复 DoStep 不得二次推进相位；推进一个步长
// 后只前进一步。
func TestLoaderIdempotent(t *testing.T) {
	node := newLoaderNode(t, "LOADER1")
	m := newLoaderMnaStub(node)
	m.nodeV[testRDYNode] = defaultVHigh
	for i := 0; i < 8; i++ {
		m.nodeV[mna.NodeID(i)] = loaderBitLevel(int(frameSyncByte), i, defaultVHigh)
	}
	tt := &loaderTimeStub{step: testStep}
	doLoaderStep(m, tt, node)
	if got := node.GetFloat64(slotPhase); got != float64(phaseHold) {
		t.Fatalf("收到 SYNC 应进入 hold 相位（ACK 高），实际相位 %v", got)
	}
	phase, ack := node.GetFloat64(slotPhase), m.nodeV[testACKNode]
	for i := 0; i < 5; i++ {
		doLoaderStep(m, tt, node)
	}
	if got := node.GetFloat64(slotPhase); got != phase {
		t.Fatalf("同一仿真时刻不得二次推进相位：%v → %v", phase, got)
	}
	if got := m.nodeV[testACKNode]; got != ack {
		t.Fatalf("同一仿真时刻输出必须稳定：ACK %v → %v", ack, got)
	}
	if tt.noConv != 0 {
		t.Fatalf("幂等短路不应触发 NoConverged，noConv=%d", tt.noConv)
	}
	m.nodeV[testRDYNode] = 0 // UART 看到 ACK 高后撤掉 RDY
	tt.now = testStep
	doLoaderStep(m, tt, node)
	if got := node.GetFloat64(slotPhase); got != float64(phaseRelease) {
		t.Fatalf("推进一个步长后应只前进到 release，实际相位 %v", got)
	}
}

// TestLoaderCpuModeParameter 覆盖 C2.5：同一个元件按 CpuMode 参数化两种时序。
// v2 的地址空间是 54 的 LA[0:3]（16 字节），写脉冲保持 2 个接受时刻；v3 是 8 位地址 + 1 个。
func TestLoaderCpuModeParameter(t *testing.T) {
	if got := loaderModeOf(newLoaderNode(t, "LOADER1")); got.name != modeV3 {
		t.Fatalf("缺省 CpuMode 应为 v3，实际 %q", got.name)
	}

	// v2：超过 16 字节的帧必须拒绝（地址空间不足）。
	node := newLoaderNode(t, "LOADER2")
	node.Base().NodeValue[slotCpuMode] = modeV2
	run := runLoader(t, node, nil, buildFrame(make([]byte, 17)), 5*testStep, testStep, 6000)
	if got := node.GetFloat64(slotError); got != 1 {
		t.Fatalf("v2 模式必须拒绝超过 16 字节的帧，error=%v", got)
	}
	assertNoRamWrite(t, run.samples)

	// v2：合法帧的 LWE 高窗口为 2 个连续接受时刻。
	node2 := newLoaderNode(t, "LOADER3")
	node2.Base().NodeValue[slotCpuMode] = modeV2
	prog := []byte{0x01, 0x02, 0x03, 0x04}
	run2 := runLoader(t, node2, nil, buildFrame(prog), 5*testStep, testStep, 6000)
	if got := node2.GetFloat64(slotDone); got != 1 {
		t.Fatalf("v2 模式合法帧应加载成功，done=%v error=%v", got, node2.GetFloat64(slotError))
	}
	assertWriteTiming(t, run2.samples, prog)
	if got := maxLWEHighRun(run2.samples); got != 2 {
		t.Fatalf("v2 的 LWE 高窗口应为 2 个接受时刻，实际 %d", got)
	}

	// v3：同规模帧的 LWE 高窗口为 1 个接受时刻。
	node3 := newLoaderNode(t, "LOADER4")
	run3 := runLoader(t, node3, nil, buildFrame(prog), 5*testStep, testStep, 6000)
	if got := node3.GetFloat64(slotDone); got != 1 {
		t.Fatalf("v3 模式合法帧应加载成功，done=%v", got)
	}
	if got := maxLWEHighRun(run3.samples); got != 1 {
		t.Fatalf("v3 的 LWE 高窗口应为 1 个接受时刻，实际 %d", got)
	}
}

// maxLWEHighRun 返回 LWE 连续为高的最长接受时刻数。
func maxLWEHighRun(samples []loaderSample) int {
	best, run := 0, 0
	for _, s := range samples {
		if s.lwe {
			run++
			if run > best {
				best = run
			}
		} else {
			run = 0
		}
	}
	return best
}
