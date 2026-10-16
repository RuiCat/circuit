// stepGuard_test.go：StepGuard 的步长超界判据及其反证。
package digital

import (
	"math"
	"strings"
	"testing"
)

// TestStepGuardRejectsOversizedStep 覆盖超界报错、错误文本，以及「放大上界后同用例不再报错」的反证。
func TestStepGuardRejectsOversizedStep(t *testing.T) {
	tt := &stubTime{step: 1e-6}
	err := StepGuard(tt, 1e-7) // 步长 1e-6 > 上界 1e-7
	if err == nil {
		t.Fatal("步长 1e-6 超过上界 1e-7，必须返回错误")
	}
	msg := err.Error()
	if !strings.Contains(msg, "当前步长") || !strings.Contains(msg, "上界") {
		t.Fatalf("错误文本必须含「当前步长」与「上界」，实际: %s", msg)
	}
	if !strings.Contains(msg, "1e-06") || !strings.Contains(msg, "1e-07") {
		t.Fatalf("错误文本应给出具体数值（1e-06 / 1e-07），实际: %s", msg)
	}

	// 反证：把上界放大到步长之上，同一输入必须不再报错
	// （否则判据只是「永远报错」，没有在比较步长与上界）。
	if err := StepGuard(tt, 1e-5); err != nil {
		t.Fatalf("上界 1e-5 大于步长 1e-6，应通过，实际: %v", err)
	}
	// 边界：步长恰好等于上界不算超界。
	if err := StepGuard(tt, 1e-6); err != nil {
		t.Fatalf("步长等于上界不应报错，实际: %v", err)
	}
	// 上界必须按元件标称时序算：limit <= 标称/10 时不得放行更粗的步长。
	if err := StepGuard(tt, 1e-6/10); err == nil {
		t.Fatal("上界取标称时序 1/10 时，标称步长必须报错")
	}
}

// TestStepGuardRejectsInvalidLimitAndTime 覆盖 limit<=0、nil 时间接口与 NaN 步长。
func TestStepGuardRejectsInvalidLimitAndTime(t *testing.T) {
	err := StepGuard(&stubTime{step: 1e-9}, 0)
	if err == nil {
		t.Fatal("limit=0 必须报错")
	}
	if !strings.Contains(err.Error(), "当前步长") || !strings.Contains(err.Error(), "上界") {
		t.Fatalf("limit=0 的错误文本应同样含两项，实际: %s", err.Error())
	}
	if err := StepGuard(&stubTime{step: 1e-9}, -1e-7); err == nil {
		t.Fatal("limit<0 必须报错")
	}
	if err := StepGuard(nil, 1e-7); err == nil {
		t.Fatal("仿真时间接口为 nil 必须报错而不是 panic")
	}
	if err := StepGuard(&stubTime{step: math.NaN()}, 1e-7); err == nil {
		t.Fatal("步长为 NaN 必须报错而不是静默放行")
	}
}
