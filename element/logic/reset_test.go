// reset_test.go：RAM.Reset / DFlipFlop.Reset 的 K5 验收用例（含 Lead D4 语义修订）。
package logic

import (
	"math"
	"strings"
	"testing"

	"circuit/element"
	etime "circuit/element/time"
	"circuit/load"
)

// findElementByType 在已加载的上下文中按元件类型取第一个实例。
func findElementByType(t *testing.T, con *element.Context, elemType element.NodeType) element.NodeFace {
	t.Helper()
	for _, elem := range con.Nodelist {
		if elem.Type() == elemType {
			return elem
		}
	}
	t.Fatalf("网表中未找到类型 %d 的元件", elemType)
	return nil
}

// ramResetNetlist 返回只含一个 RAM 实例的网表（引脚接共用节点，不做求解），
// values 为 RAM 参数（size 与可选 init_hex）。RAM 共 25 引脚：A0..A7、D0..D7、WE、Q0..Q7。
func ramResetNetlist(values string) string {
	return `
V1 [1,-1] [0,0,0,0,5]
RAM1 [1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,4,5,6,7,8,9,10,11] [` + values + `]
`
}

// TestRamReset 验证 RAM.Reset（K5/D4）：
// init_hex 非空时恢复 init_hex；init_hex 为空时首次 Reset 快照当前 mem_hex 作为初值；
// 重复 Reset 幂等（第二次运行从同一初值起步）。
func TestRamReset(t *testing.T) {
	// 场景 1：init_hex 非空 → Reset 恢复 init_hex（去空白，按 size 补零）。
	con, err := load.LoadString(ramResetNetlist(`256, "08 19 2A"`))
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	ram := findElementByType(t, con, RAMType)

	// 模拟运行期/外部写入，确认会被 Reset 抹掉。
	(&RAM{}).SetByte(ram, 0x00, 0xDE)
	if err := con.CallMark(element.MarkReset); err != nil {
		t.Fatalf("MarkReset 失败: %v", err)
	}
	want := "08192A" + strings.Repeat("0", 512-6)
	if got := ram.Base().GetString(2); got != want {
		t.Fatalf("init_hex 非空时 mem_hex 未恢复: got %q want %q", got, want)
	}

	// 幂等：重复 Reset 结果不变。
	if err := con.CallMark(element.MarkReset); err != nil {
		t.Fatalf("重复 MarkReset 失败: %v", err)
	}
	if got := ram.Base().GetString(2); got != want {
		t.Fatalf("重复 Reset 不幂等: got %q want %q", got, want)
	}

	// 场景 2：init_hex 为空 + 仿真前 InitFromHex 预载。
	// 首次 Reset（引擎在 TransientSimulation 启动时自动调用）必须保留预载，
	// 并把该内容快照为初值；之后再 Reset 恢复快照、抹掉运行期写入。
	con2, err := load.LoadString(ramResetNetlist(`256`))
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	ram2 := findElementByType(t, con2, RAMType)
	if err := (&RAM{}).InitFromHex(ram2, "6456"); err != nil {
		t.Fatalf("InitFromHex 失败: %v", err)
	}
	if err := con2.CallMark(element.MarkReset); err != nil {
		t.Fatalf("首次 MarkReset 失败: %v", err)
	}
	snapshot := "6456" + strings.Repeat("0", 512-4)
	if got := ram2.Base().GetString(2); got != snapshot {
		t.Fatalf("首次 Reset 未保留预载并快照: got %q want %q", got, snapshot)
	}

	// 模拟运行期写入后再次 Reset：应回到首次快照，而非保留运行期内容。
	(&RAM{}).SetByte(ram2, 0x00, 0xEE)
	if err := con2.CallMark(element.MarkReset); err != nil {
		t.Fatalf("第二次 MarkReset 失败: %v", err)
	}
	if got := ram2.Base().GetString(2); got != snapshot {
		t.Fatalf("重复 Reset 未恢复首次快照: got %q want %q", got, snapshot)
	}
}

// TestDffReset 验证 DFlipFlop.Reset（K5/D4）：运行态槽回初值、重复 Reset 幂等，
// 且复位后仿真输出回到初值 Q=0V、NQ=5V。
func TestDffReset(t *testing.T) {
	con, err := load.LoadString(`
V1 [1,-1] [0,0,0,0,0]
V2 [2,-1] [0,0,0,0,0]
DFF1 [1,2,3,4] [5]
R1 [3,-1] [10000]
R2 [4,-1] [10000]
`)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	dff := findElementByType(t, con, DFlipFlopType)

	// 伪造上一轮运行态：q_state=1、prev_clk=5V、initialized=true、prev_d=5V。
	base := dff.Base()
	base.SetBool(2, true)
	base.SetFloat64(1, 5.0)
	base.SetBool(3, true)
	base.SetFloat64(4, 5.0)

	if err := con.CallMark(element.MarkReset); err != nil {
		t.Fatalf("MarkReset 失败: %v", err)
	}
	if base.GetBool(2) || base.GetBool(3) || base.GetFloat64(1) != 0 || base.GetFloat64(4) != 0 {
		t.Fatalf("Reset 后运行态未回初值: q_state=%v initialized=%v prev_clk=%v prev_d=%v",
			base.GetBool(2), base.GetBool(3), base.GetFloat64(1), base.GetFloat64(4))
	}

	// 幂等：重复 Reset 结果不变。
	if err := con.CallMark(element.MarkReset); err != nil {
		t.Fatalf("重复 MarkReset 失败: %v", err)
	}
	if base.GetBool(2) || base.GetBool(3) || base.GetFloat64(1) != 0 || base.GetFloat64(4) != 0 {
		t.Fatalf("重复 Reset 不幂等")
	}

	// 输出回初值：跑短仿真，clk=d=0 时 Q=0V、NQ=5V。
	con.Time, err = etime.NewTimeMNA(1e-3)
	if err != nil {
		t.Fatalf("创建时间控制器失败: %v", err)
	}
	if err := con.Time.SetStepLimits(1e-6, 1e-4); err != nil {
		t.Fatalf("设置步长失败: %v", err)
	}
	if err := etime.TransientSimulation(con, func([]float64) {}); err != nil {
		t.Fatalf("仿真失败: %v", err)
	}
	q := con.GetRawNodeVoltage(3)
	nq := con.GetRawNodeVoltage(4)
	if math.Abs(q) > 0.1 || math.Abs(nq-5.0) > 0.1 {
		t.Fatalf("Reset 后输出未回初值: Q=%.3fV（期望 0）NQ=%.3fV（期望 5）", q, nq)
	}
}
