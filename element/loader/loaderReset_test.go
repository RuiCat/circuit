// loaderReset_test.go：Reset 用例——同一 Context 重新加载时相位/帧状态/幂等戳全部复位，
// 且复位后重新保持 LOAD 高（CPU 冻结）再可重复同一加载结果（C12/K5 的重载判据）。
package loader

import (
	"math"
	"testing"

	"circuit/element"
)

// TestLoaderResetReloads 验证 Reset 回到 idle 后同一实例可重复加载且结果一致。
func TestLoaderResetReloads(t *testing.T) {
	prog := []byte{0x5A, 0xA5, 0x00, 0xFF}
	node := newLoaderNode(t, "LOADER1")
	first := runLoader(t, node, nil, buildFrame(prog), 5*testStep, testStep, 5000)
	if got := node.GetFloat64(slotDone); got != 1 {
		t.Fatalf("首次加载应成功，done=%v error=%v", got, node.GetFloat64(slotError))
	}
	assertWriteTiming(t, first.samples, prog)

	element.ElementList[LoaderType].Reset(node)
	if got := node.GetFloat64(slotPhase); got != float64(phaseIdle) {
		t.Fatalf("Reset 后相位应回到 idle，实际 %v", got)
	}
	if got := node.GetFloat64(slotError); got != 0 {
		t.Fatalf("Reset 后 error 应清零，实际 %v", got)
	}
	if got := node.GetFloat64(slotDone); got != 0 {
		t.Fatalf("Reset 后 done 应清零，实际 %v", got)
	}
	if got := node.GetFloat64(slotAddr); got != 0 {
		t.Fatalf("Reset 后 addr 应清零，实际 %v", got)
	}
	if got := node.GetFloat64(slotFrame); got != float64(frameWaitSync) {
		t.Fatalf("Reset 后帧状态应回到等 SYNC，实际 %v", got)
	}
	if got := node.GetFloat64(slotLastStepTime); !math.IsNaN(got) {
		t.Fatalf("Reset 后幂等戳应回到 NaN，实际 %v", got)
	}

	// 复位后重新加载同一帧：结果必须与第一次一致（LOAD 再次保持高、写时序相同）。
	second := runLoader(t, node, nil, buildFrame(prog), 5*testStep, testStep, 5000)
	if got := node.GetFloat64(slotDone); got != 1 {
		t.Fatalf("Reset 后重载应成功，done=%v error=%v", got, node.GetFloat64(slotError))
	}
	assertWriteTiming(t, second.samples, prog)
	if got := second.m.nodeV[testLOADNode]; got != 0 {
		t.Fatalf("Reset 后重载完成应释放 CPU（LOAD=0），实际 %v V", got)
	}
}
