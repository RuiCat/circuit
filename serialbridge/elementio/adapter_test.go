// adapter_test.go 用真实 element.Context（load + element/register 注册路径）验证 K5 适配器：
// 读槽（数值 / bool 归一 / 越界 / nil / 类型不符）、写事件后被引擎消费、推进确实驱动了引擎、
// 以及错误路径可判定且有界。反证夹具在 testdata/，用 go test -overlay 替换 adapter.go 复算。
package elementio_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"circuit/element"
	_ "circuit/element/register"
	etime "circuit/element/time"
	"circuit/load"
	"circuit/mna"
	"circuit/serialbridge"
	"circuit/serialbridge/elementio"
)

// loadNetlist 读取仓库内既有网表并加载成真实 Context（走 element/register 的注册路径）。
func loadNetlist(t *testing.T, name string) *element.Context {
	t.Helper()
	path := filepath.Join("..", "..", "cmd", "circuits", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取网表 %s 失败: %v", path, err)
	}
	con, err := load.LoadString(string(raw))
	if err != nil {
		t.Fatalf("加载网表 %s 失败: %v", name, err)
	}
	return con
}

// newAdapter 构造适配器并在用例结束时关闭（关闭超时算用例失败）。
func newAdapter(t *testing.T, con *element.Context, instance string, opts elementio.Options) *elementio.Adapter {
	t.Helper()
	a, err := elementio.NewAdapter(con, instance, opts)
	if err != nil {
		t.Fatalf("NewAdapter(%s) 失败: %v", instance, err)
	}
	t.Cleanup(func() {
		if err := a.Close(); err != nil {
			t.Errorf("Close 失败: %v", err)
		}
	})
	return a
}

// findNode 按实例名取元件 Node（用例内直接改槽值用）。
func findNode(t *testing.T, con *element.Context, instance string) *element.Node {
	t.Helper()
	for _, face := range con.Nodelist {
		if node := face.Base(); node != nil && node.InstanceName == instance {
			return node
		}
	}
	t.Fatalf("网表中找不到实例 %s", instance)
	return nil
}

// TestReadSlotRealContext 用真实分压网表读数值槽，并逐个反证三类非数据错误：
// 越界、nil、非数值（string 槽）都必须报错，不许返回 0 冒充数据。
func TestReadSlotRealContext(t *testing.T) {
	con := loadNetlist(t, "01_resistor_divider.net")
	a := newAdapter(t, con, "R1", elementio.Options{})

	got, err := a.ReadSlot(0) // R1 NodeValue[0] = R = 1000Ω
	if err != nil {
		t.Fatalf("ReadSlot(0) 读取真实数值槽失败: %v", err)
	}
	if got != 1000 {
		t.Errorf("槽 0 期望 1000（R1 阻值），实际 %v", got)
	}

	for _, idx := range []int{-1, 2, 99} { // R1 只有 2 个槽（0 阻值、1 电流）
		if _, err := a.ReadSlot(idx); !errors.Is(err, serialbridge.ErrSlotValue) {
			t.Errorf("槽 %d 期望越界错误（errors.Is ErrSlotValue），实际 %v", idx, err)
		}
	}

	node := findNode(t, con, "R1")
	node.NodeValue[1] = nil // 真实槽位置成 nil
	if _, err := a.ReadSlot(1); !errors.Is(err, serialbridge.ErrSlotValue) {
		t.Errorf("nil 槽期望错误（errors.Is ErrSlotValue），实际 %v", err)
	}
}

// TestReadSlotBoolAndTypeMismatch 覆盖 bool 槽归一（true=1/false=0）与非数值槽报错：
// 晶体管 NodeValue[0] 是 bool（PNP 标志），NodeValue[2] 是 string（模型名）。
func TestReadSlotBoolAndTypeMismatch(t *testing.T) {
	con := loadNetlist(t, "04_transistor_switch.net")
	a := newAdapter(t, con, "Q1", elementio.Options{})

	falsy, err := a.ReadSlot(0)
	if err != nil || falsy != 0 {
		t.Fatalf("bool(false) 槽期望 0/nil，实际 %v/%v", falsy, err)
	}
	findNode(t, con, "Q1").SetBool(0, true)
	truthy, err := a.ReadSlot(0)
	if err != nil || truthy != 1 {
		t.Fatalf("bool(true) 槽期望 1/nil，实际 %v/%v", truthy, err)
	}

	if _, err := a.ReadSlot(2); !errors.Is(err, serialbridge.ErrSlotValue) {
		t.Errorf("string 槽期望类型错误（errors.Is ErrSlotValue），实际 %v", err)
	}
}

// TestWriteEventConsumedByAdvance 用真实事件开关（B1，EventSlots: "A_IN"→槽 6）验证：
// WriteEvent 后 Advance 报告该事件被消费、事件真的进了元件槽、且节点电压随之变化
// （证明驱动的是真引擎而非空转）；没有消费者的名字不算被消费（不误报）。
func TestWriteEventConsumedByAdvance(t *testing.T) {
	con := loadNetlist(t, "30_logic_gates_interactive.net")
	a := newAdapter(t, con, "B1", elementio.Options{})

	if _, err := a.Advance(); err != nil { // 等价 CLI 的"先 step 一次初始化"
		t.Fatalf("初始化步失败: %v", err)
	}
	if v := con.GetRawNodeVoltage(mna.NodeID(1)); v >= 0.5 {
		t.Fatalf("前置条件不成立：开关断开时节点 1 期望低电平，实际 %v", v)
	}

	if err := a.WriteEvent("A_IN", 1); err != nil {
		t.Fatalf("WriteEvent 失败: %v", err)
	}
	consumed, err := a.Advance()
	if err != nil {
		t.Fatalf("Advance 失败: %v", err)
	}
	if !slices.Contains(consumed, "A_IN") {
		t.Fatalf("本步期望报告消费 [A_IN]，实际 %v", consumed)
	}
	if v, err := a.ReadSlot(6); err != nil || v != 1 { // 事件真的被 PushEvents 写进元件槽
		t.Fatalf("消费后元件槽 6（eventValue）期望 1/nil，实际 %v/%v", v, err)
	}
	if _, err := a.Advance(); err != nil {
		t.Fatalf("第二步失败: %v", err)
	}
	if v := con.GetRawNodeVoltage(mna.NodeID(1)); v < 4.5 { // 开关闭合 ⇒ 节点 1 接 5V 电源轨
		t.Errorf("推进后节点 1 期望 ≈5V，实际 %v（引擎没有被真正驱动）", v)
	}

	// 无新写入时不得报告任何消费（反证：不产生幽灵消费）
	consumed, err = a.Advance()
	if err != nil {
		t.Fatalf("第三步失败: %v", err)
	}
	if len(consumed) != 0 {
		t.Errorf("没有新写入的一步期望 0 个被消费事件，实际 %v", consumed)
	}

	// 没有消费者的名字：引擎不会置 nil ⇒ 不得报告为已消费（桥据此继续背压）
	if err := a.WriteEvent("NO_CONSUMER", 1); err != nil {
		t.Fatalf("WriteEvent(NO_CONSUMER) 失败: %v", err)
	}
	consumed, err = a.Advance()
	if err != nil {
		t.Fatalf("第四步失败: %v", err)
	}
	if slices.Contains(consumed, "NO_CONSUMER") {
		t.Errorf("无消费者的事件被误报为已消费: %v", consumed)
	}
	if con.GetEvent("NO_CONSUMER") == nil {
		t.Errorf("无消费者的事件应仍在事件表里（未被置 nil）")
	}
}

// TestAdvanceDrivesRealEngine 反证"Advance 真的驱动了引擎"：仿真前节点电压为 0，
// 推进后等于分压理论值 5V×2k/(1k+2k)。
func TestAdvanceDrivesRealEngine(t *testing.T) {
	con := loadNetlist(t, "01_resistor_divider.net")
	if before := con.GetRawNodeVoltage(mna.NodeID(2)); before >= 0.5 {
		t.Fatalf("前置条件不成立：未推进时节点 2 期望低电平，实际 %v", before)
	}
	a := newAdapter(t, con, "R1", elementio.Options{})
	if _, err := a.Advance(); err != nil {
		t.Fatalf("Advance 失败: %v", err)
	}
	after := con.GetRawNodeVoltage(mna.NodeID(2))
	want := 5.0 * 2000 / (1000 + 2000)
	if diff := after - want; diff > 1e-3 || diff < -1e-3 {
		t.Fatalf("推进后节点 2 期望 ≈%.4fV，实际 %.6fV（引擎没有被真正驱动）", want, after)
	}
}

// TestErrorsAreJudgeableAndBounded 覆盖构造与运行期错误路径：可判定、不 panic、不无界等待。
func TestErrorsAreJudgeableAndBounded(t *testing.T) {
	con := loadNetlist(t, "01_resistor_divider.net")

	if _, err := elementio.NewAdapter(nil, "R1", elementio.Options{}); !errors.Is(err, elementio.ErrNilContext) {
		t.Errorf("nil Context 期望 ErrNilContext，实际 %v", err)
	}
	if _, err := elementio.NewAdapter(con, "", elementio.Options{}); !errors.Is(err, elementio.ErrEmptyInstance) {
		t.Errorf("空实例名期望 ErrEmptyInstance，实际 %v", err)
	}
	if _, err := elementio.NewAdapter(con, "NOPE", elementio.Options{}); !errors.Is(err, elementio.ErrInstanceMissing) {
		t.Errorf("不存在实例期望 ErrInstanceMissing，实际 %v", err)
	}

	a := newAdapter(t, con, "R1", elementio.Options{})
	if err := a.WriteEvent("", 1); !errors.Is(err, elementio.ErrEmptyEventName) {
		t.Errorf("空事件名期望 ErrEmptyEventName，实际 %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("Close 失败: %v", err)
	}
	start := time.Now()
	if _, err := a.Advance(); !errors.Is(err, elementio.ErrAdapterClosed) {
		t.Errorf("关闭后 Advance 期望 ErrAdapterClosed，实际 %v", err)
	}
	if err := a.WriteEvent("A_IN", 1); !errors.Is(err, elementio.ErrAdapterClosed) {
		t.Errorf("关闭后 WriteEvent 期望 ErrAdapterClosed，实际 %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("关闭后调用期望立即返回，实际耗时 %v", elapsed)
	}
	if _, err := a.ReadSlot(0); err != nil { // 纯读在关闭后仍可用（已文档化）
		t.Errorf("关闭后 ReadSlot 期望仍可读，实际 %v", err)
	}

	// 引擎被外部停止 ⇒ Advance 报 ErrEngineStopped（有界，不等满 StepTimeout）
	con2 := loadNetlist(t, "01_resistor_divider.net")
	b := newAdapter(t, con2, "R1", elementio.Options{})
	engine, ok := con2.Time.(*etime.TimeMNA)
	if !ok {
		t.Fatalf("con.Time 期望 *etime.TimeMNA，实际 %T", con2.Time)
	}
	engine.Stop()
	start = time.Now()
	if _, err := b.Advance(); !errors.Is(err, elementio.ErrEngineStopped) {
		t.Errorf("引擎停止后期望 ErrEngineStopped，实际 %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("引擎停止后 Advance 期望立即返回，实际耗时 %v", elapsed)
	}
}

// TestAdvanceStepTimeoutIsBounded 反证"不无界等待"：1ns 的步超时下，Advance 必须返回
// ErrStepTimeout，而不是等真实一步跑完（真实一步至少是微秒量级）。
func TestAdvanceStepTimeoutIsBounded(t *testing.T) {
	con := loadNetlist(t, "01_resistor_divider.net")
	a := newAdapter(t, con, "R1", elementio.Options{StepTimeout: time.Nanosecond})
	if _, err := a.Advance(); !errors.Is(err, elementio.ErrStepTimeout) {
		t.Fatalf("1ns 步超时期望 ErrStepTimeout，实际 %v", err)
	}
}

// TestBridgeConsumesSlotIoContract 用 m5 的真实桥核对 K5 契约方向：
// 桥写事件后只有适配器报告"该事件被消费"才解除背压（不是与 UART 元件的端到端往返）。
func TestBridgeConsumesSlotIoContract(t *testing.T) {
	con := loadNetlist(t, "30_logic_gates_interactive.net")
	a := newAdapter(t, con, "B1", elementio.Options{})
	if _, err := a.Advance(); err != nil { // 初始化步
		t.Fatalf("初始化步失败: %v", err)
	}
	bridge, err := serialbridge.NewBridge(a, serialbridge.Config{TxEvent: "A_IN", AckEvent: "A_IN"})
	if err != nil {
		t.Fatalf("NewBridge 失败: %v", err)
	}
	if err := bridge.SendByte(1); err != nil {
		t.Fatalf("SendByte 失败: %v", err)
	}
	if err := bridge.WaitTxDrained(); err != nil {
		t.Fatalf("WaitTxDrained 失败: %v（适配器没有报告事件被消费）", err)
	}
	if bridge.Pending() {
		t.Errorf("事件被报告消费后桥不应仍处于在途状态")
	}
}
