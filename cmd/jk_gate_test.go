package main_test

import (
	"testing"

	_ "circuit/element/register"
	etime "circuit/element/time"
	"circuit/load"
)

// TestJKGateLevel 门级 JK 触发器（基础逻辑门 U + SR 锁存器，主从结构）真值表时序验证。
//
// 网表: cmd/circuits/33_jk_flipflop_test.net
//   主锁存器: S1=AND(J,CLK,NQ), R1=AND(K,CLK,Q) → SR1 (MQ/MNQ)   [CLK 高时跟随]
//   从锁存器: S2=AND(MQ,NCLK), R2=AND(MNQ,NCLK) → SR2 (Q/NQ)     [CLK 低时采样]
//   真值表: J=0K=0 保持 | J=1K=0 置位 | J=0K=1 复位 | J=1K=1 翻转。
//
// 引擎无门传播延迟（同步更新），Q 在 CLK 每个边沿采样（每 0.5ms 一次）；
// 每段推进 2.5ms = 5 个边沿（奇数个采样点），保证 J=K=1 翻转后可判定状态。
// 每阶段重建时间控制器（时间归零重跑），SR q_state 跨仿真保留（无自定义 Reset）。
// 事件值用 float64 类型（PushEvents 有类型检查，int 会被跳过不写入）。
func TestJKGateLevel(t *testing.T) {
	nl := readNetlist(t, "33_jk_flipflop_test.net")
	con, err := load.LoadString(nl)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}

	run := func(events map[string]float64, duration float64) float64 {
		con.Time, err = etime.NewTimeMNA(duration)
		if err != nil {
			t.Fatalf("重建时间控制器失败: %v", err)
		}
		for k, v := range events {
			con.SetEvent(k, v)
		}
		if err := etime.TransientSimulation(con, func([]float64) {}); err != nil {
			t.Fatalf("仿真失败: %v", err)
		}
		return con.GetRawNodeVoltage(13) // Q
	}

	// 阶段0：复位 J=0,K=1（任意初始态 → Q=0）
	if q := run(map[string]float64{"J_EVT": 0, "K_EVT": 1}, 2.5e-3); q > lo {
		t.Errorf("复位失败: Q=%.3fV 期望低", q)
	}
	// 阶段1：保持 J=0,K=0 → Q 保持 0
	if q := run(map[string]float64{"J_EVT": 0, "K_EVT": 0}, 2.5e-3); q > lo {
		t.Errorf("保持0失败: Q=%.3fV 期望低", q)
	}
	// 阶段2：置位 J=1,K=0 → Q=1
	if q := run(map[string]float64{"J_EVT": 1, "K_EVT": 0}, 2.5e-3); q < hi {
		t.Errorf("置位失败: Q=%.3fV 期望高", q)
	}
	// 阶段3：复位 J=0,K=1 → Q=0
	if q := run(map[string]float64{"J_EVT": 0, "K_EVT": 1}, 2.5e-3); q > lo {
		t.Errorf("复位失败: Q=%.3fV 期望低", q)
	}
	// 阶段4：翻转 J=K=1（从 0）→ Q=1
	if q := run(map[string]float64{"J_EVT": 1, "K_EVT": 1}, 2.5e-3); q < hi {
		t.Errorf("翻转0→1失败: Q=%.3fV 期望高", q)
	}
	// 阶段5：再翻转 J=K=1（从 1）→ Q=0
	if q := run(map[string]float64{"J_EVT": 1, "K_EVT": 1}, 2.5e-3); q > lo {
		t.Errorf("翻转1→0失败: Q=%.3fV 期望低", q)
	}
	// 阶段6：置位收尾 J=1,K=0 → Q=1（确认连续翻转后仍可置位）
	if q := run(map[string]float64{"J_EVT": 1, "K_EVT": 0}, 2.5e-3); q < hi {
		t.Errorf("收尾置位失败: Q=%.3fV 期望高", q)
	}
}
