// timeMnaGuard_test.go —— UD7：TransientSimulation 入口必须显式拒绝非 *TimeMNA 的 Time。
//
// 缺陷形态：仿真循环里消费的策略状态（ApplyStrategy）、有界牛顿事件计数、强制重盖标志
// （forceStamp）都挂在具体类型 *TimeMNA 上。此前这些点用 `con.Time.(*TimeMNA)` **逐处断言**，
// 一旦 con.Time 被装饰器包装（例如为了观测/拦截而包一层转发接口），断言失败会**静默跳过**：
// 仿真照常跑完、结果看似正常，但策略未同步、强制重盖请求被丢弃、计数不增 —— 静默错。
//
// 修法：入口一次性断言并**显式报错**（失败要响），不再逐处静默跳过。
//
// 本文件的判据是**判别式**的：装饰器包装下必须返回错误（修复前会静默跑完、不报错）。
package time

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "circuit/element/register"
	"circuit/load"
	"circuit/mna"
)

// timeDecorator 只转发 mna.Time（**缺少**引擎状态方法）：入口必须拒绝它。
// 这正是 UD7 的缺陷形态——旧实现下它会静默跑完，策略不生效、重盖请求丢失。
type timeBase interface {
	mna.Time
}

type timeDecorator struct {
	timeBase
}

// engineTimeDecorator 转发 EngineTime（约定写法）：入口必须接受它并正常跑完。
type engineTimeDecorator struct {
	EngineTime
}

// TestTransientSimulationRejectsNonTimeMNA 断言入口对装饰器的显式拒绝。
func TestTransientSimulationRejectsNonTimeMNA(t *testing.T) {
	con, err := load.LoadString(readGuardNetlist(t))
	if err != nil {
		t.Fatalf("加载网表失败: %v", err)
	}
	tm, err := NewTimeMNA(1e-3)
	if err != nil {
		t.Fatalf("NewTimeMNA: %v", err)
	}
	// 先确认未被包装时能正常跑（对照组，避免把"引擎坏了"误判成"判据生效"）
	con.Time = tm
	if err := TransientSimulation(con, func([]float64) {}); err != nil {
		t.Fatalf("未包装时应正常运行，实际报错: %v", err)
	}

	// 再确认包装后必须报错（修复前这里会静默跑完并返回 nil）
	tm2, err := NewTimeMNA(1e-3)
	if err != nil {
		t.Fatalf("NewTimeMNA: %v", err)
	}
	con.Time = &timeDecorator{timeBase: tm2}
	err = TransientSimulation(con, func([]float64) {})
	if err == nil {
		t.Fatalf("只转发 mna.Time 的装饰器应显式报错（UD7：不得静默丢语义），实际返回 nil")
	}
	if !strings.Contains(err.Error(), "EngineTime") {
		t.Errorf("错误信息应点明需要 element/time.EngineTime，实际: %v", err)
	}
	t.Logf("拒绝信息: %v", err)

	// 约定写法必须可用：匿名嵌入 EngineTime 的装饰器整体转发，应当正常运行。
	tm3, err := NewTimeMNA(1e-3)
	if err != nil {
		t.Fatalf("NewTimeMNA: %v", err)
	}
	con.Time = &engineTimeDecorator{EngineTime: tm3}
	if err := TransientSimulation(con, func([]float64) {}); err != nil {
		t.Fatalf("嵌入 EngineTime 的装饰器应被接受并正常跑完，实际: %v", err)
	}
}

// TestForceStampConsumedBySimulationLoop 正向判据：强制重盖请求必须被仿真循环消费。
//
// 该判据保护"消费点不再依赖逐处断言"这一改动——若入口断言被移除且消费点退回静默跳过，
// 本用例仍会通过（因为类型正确），因此它与上一条**成对**使用：上一条管"拒绝装饰器"，
// 本用例管"正常路径仍消费标志"。
func TestForceStampConsumedBySimulationLoop(t *testing.T) {
	con, err := load.LoadString(readGuardNetlist(t))
	if err != nil {
		t.Fatalf("加载网表失败: %v", err)
	}
	tm, err := NewTimeMNA(1e-3)
	if err != nil {
		t.Fatalf("NewTimeMNA: %v", err)
	}
	con.Time = tm
	tm.InvalidateLinearStamp()
	if !tm.ConsumeForceStamp() {
		t.Fatalf("置位后第一次消费应返回 true")
	}
	if tm.ConsumeForceStamp() {
		t.Errorf("同一次置位不应被消费两次（CompareAndSwap 语义）")
	}
}

// readGuardNetlist 读取一个最小的 RC 网表用于本文件的用例。
func readGuardNetlist(t *testing.T) string {
	t.Helper()
	for _, p := range []string{
		filepath.Join("..", "..", "cmd", "circuits", "02_rc_filter.net"),
		filepath.Join("cmd", "circuits", "02_rc_filter.net"),
	} {
		if b, err := os.ReadFile(p); err == nil {
			return string(b)
		}
	}
	t.Fatalf("找不到测试网表 02_rc_filter.net")
	return ""
}
