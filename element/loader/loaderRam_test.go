// loaderRam_test.go：LOADER → RAM 的端到端单元用例——12 字节加载后逐字节读 RAM 的 mem_hex 槽 2。
package loader

import (
	"encoding/hex"
	"testing"

	"circuit/element"
	"circuit/element/logic"
	"circuit/load/ast"
	"circuit/mna"
)

// newRAMNode 走与 load 相同的实例化路径构造一个 256 字节 RAM 实例，
// 并把 A/D/WE 引脚接到 LOADER 的 LA/LD/LWE 输出节点上（Q 引脚接空闲节点）。
func newRAMNode(t *testing.T, instanceName string) *element.Node {
	t.Helper()
	face, ok := element.ElementList[logic.RAMType]
	if !ok {
		t.Fatalf("RAM 未注册（NodeType %d）", logic.RAMType)
	}
	cfg := face.Base(ast.ElementNode{
		Type: "RAM", ID: "1", InstanceName: instanceName,
		Values: []ast.Value{{Value: "256"}},
	})
	node := &element.Node{
		ConfigPtr:    cfg,
		NodeType:     logic.RAMType,
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
	for i := 0; i < 8; i++ {
		node.Nodes[i] = testLANode(i)
		node.Nodes[8+i] = testLDNode(i)
	}
	node.Nodes[16] = testLWENode
	for i := 0; i < 8; i++ {
		node.Nodes[17+i] = 300 + mna.NodeID(i)
	}
	for v := 0; v < cfg.VoltageNum(); v++ {
		node.VoltSource[v] = mna.VoltageID(500 + v)
	}
	return node
}

// ramBytes 读 RAM 的 mem_hex 槽 2 并解码为固定长度字节切片（空槽按全 0 处理）。
func ramBytes(t *testing.T, ram *element.Node, size int) []byte {
	t.Helper()
	out := make([]byte, size)
	hexStr := ram.GetString(2)
	if hexStr == "" {
		return out
	}
	raw, err := hex.DecodeString(hexStr)
	if err != nil {
		t.Fatalf("RAM 的 mem_hex 槽 2 不是合法十六进制：%v（%q）", err, hexStr)
	}
	copy(out, raw)
	return out
}

// assertRAMZero 断言 RAM 的 mem_hex 槽 2 为空或全 0（失败帧不得写 RAM）。
func assertRAMZero(t *testing.T, ram *element.Node) {
	t.Helper()
	for addr, b := range ramBytes(t, ram, 256) {
		if b != 0 {
			t.Fatalf("失败帧不得写 RAM，但 RAM[%d]=%#02x", addr, b)
		}
	}
}

// TestLoaderWritesRam 覆盖 C4/C5：12 字节程序经 SYNC+LEN+DATA+CHK 帧逐字节写入 RAM，
// 读回 mem_hex 槽 2 逐字节比对；同时打印加载段被接受的仿真步数（预算 ≤2000）。
func TestLoaderWritesRam(t *testing.T) {
	prog := []byte{
		0x64, 0x2C, 0x09, 0x16, 0x36, 0x46,
		0x56, 0x74, 0x84, 0xA4, 0x94, 0x02,
	}
	node := newLoaderNode(t, "LOADER1")
	ram := newRAMNode(t, "RAM1")
	// 每字节按 9600 baud 的 10 个比特周期到达；仿真步长取 StepGuard 上界 T/10。
	run := runLoader(t, node, ram, buildFrame(prog), 10.0/9600, testStep, 4000)
	t.Logf("steps=%d", run.steps)

	if got := node.GetFloat64(slotError); got != 0 {
		t.Fatalf("合法帧不得置 error，实际 %v", got)
	}
	if got := node.GetFloat64(slotDone); got != 1 {
		t.Fatalf("12 字节加载完成应置 done=1，实际 %v（steps=%d）", got, run.steps)
	}
	if got := run.m.nodeV[testLOADNode]; got != 0 {
		t.Fatalf("加载成功后必须释放 CPU（LOAD=0），实际 %v V", got)
	}
	if got := run.m.nodeV[testLWENode]; got != 0 {
		t.Fatalf("加载成功后 LWE 必须为低，实际 %v V", got)
	}
	if run.steps > 2000 {
		t.Fatalf("12B 加载步数 %d 超过 C14 预算 2000", run.steps)
	}

	mem := ramBytes(t, ram, 256)
	for addr, want := range prog {
		if mem[addr] != want {
			t.Fatalf("RAM[%d]=%#02x，期望 %#02x（mem_hex 槽 2）", addr, mem[addr], want)
		}
	}
	for addr := len(prog); addr < len(mem); addr++ {
		if mem[addr] != 0 {
			t.Fatalf("越界写入：RAM[%d]=%#02x，期望 0", addr, mem[addr])
		}
	}
	assertWriteTiming(t, run.samples, prog)
}
