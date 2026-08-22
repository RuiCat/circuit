package time

import (
	"testing"
)

// TestForceStampAtomic 验证 InvalidateLinearStamp/ConsumeForceStamp 的
// 原子语义：置位一次只能消费一次，重复消费返回 false。
func TestForceStampAtomic(t *testing.T) {
	tm, err := NewTimeMNA(1.0)
	if err != nil {
		t.Fatalf("NewTimeMNA 失败: %v", err)
	}
	if tm.ConsumeForceStamp() {
		t.Fatal("初始状态不应有强制重盖请求")
	}
	tm.InvalidateLinearStamp()
	if !tm.ConsumeForceStamp() {
		t.Fatal("置位后应消费到一次请求")
	}
	if tm.ConsumeForceStamp() {
		t.Fatal("请求只能消费一次")
	}
	// 多次置位合并为一次（布尔语义）
	tm.InvalidateLinearStamp()
	tm.InvalidateLinearStamp()
	if !tm.ConsumeForceStamp() {
		t.Fatal("多次置位应消费到一次请求")
	}
}
