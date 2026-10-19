// loaderRegister_test.go：LOADER 的注册与网表可实例化验证（K6/C8/C9）。
//
// 用独立测试包走「导入 register → 网表加载」入口：只有 init() 真正注册了 LOADER，
// 网表里的 LOADER 实例才能被解析出来（单独 go build 不能作为可实例化的证据）。
package loader_test

import (
	"testing"

	"circuit/element"
	"circuit/element/loader"
	_ "circuit/element/register"
	"circuit/load"
)

// loaderNetlist 是一个只含 LOADER 实例的最小网表：28 个引脚接 0..27，
// 参数为 [LenMax, StepLimit, CpuMode]（与 m3/m4 组装时的位置一致）。
const loaderNetlist = `LOADER1 [0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,27] [32,1.0417e-5,v3]
`

// TestLoaderRegistered 验证 NodeType=49/名称 LOADER 唯一注册，且网表能实例化 28 引脚 + 19 输出源。
func TestLoaderRegistered(t *testing.T) {
	if got := element.ElementListName["loader"]; got != element.NodeType(49) {
		t.Fatalf("LOADER 注册名对应 NodeType=%d，期望 49", got)
	}
	con, err := load.LoadString(loaderNetlist)
	if err != nil {
		t.Fatalf("含 LOADER 的网表加载失败（注册缺失或引脚表不符）：%v", err)
	}
	found := false
	for _, elem := range con.Nodelist {
		if elem.Type() != element.NodeType(49) {
			continue
		}
		found = true
		if got := elem.Config().PinNum(); got != 28 {
			t.Fatalf("LOADER 引脚数 = %d，期望 28", got)
		}
		if got := elem.Config().VoltageNum(); got != 19 {
			t.Fatalf("LOADER 输出源数 = %d，期望 19（LA/LD/LOAD/LWE/ACK）", got)
		}
		if got := elem.GetFloat64(0); got != 32 {
			t.Fatalf("LenMax 参数 = %v，期望 32", got)
		}
		if got := elem.GetString(2); got != "v3" {
			t.Fatalf("CpuMode 参数 = %q，期望 v3", got)
		}
	}
	if !found {
		t.Fatal("网表加载成功但没有解析出 LOADER 实例")
	}
}

// TestLoaderExportedSlots 验证宿主可观测槽常量已导出且数值与 K2/D5 冻结值一致。
// m4/m6 必须引用这些常量，不得在网表/测试里硬编码 22/23/24。
func TestLoaderExportedSlots(t *testing.T) {
	if loader.SlotError != 22 || loader.SlotDone != 23 || loader.SlotAddr != 24 {
		t.Fatalf("宿主槽常量 = %d/%d/%d，期望 error=22/done=23/addr=24",
			loader.SlotError, loader.SlotDone, loader.SlotAddr)
	}
	if loader.SlotPhase != 25 || loader.SlotFrame != 26 || loader.SlotPendingByte != 32 {
		t.Fatalf("只读观测槽常量 = %d/%d/%d，期望 phase=25/frame=26/pendingByte=32",
			loader.SlotPhase, loader.SlotFrame, loader.SlotPendingByte)
	}
}
