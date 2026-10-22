// verify_macro_scale_test.go 用 58 宏模型网表复制 5 份，验证万级规模下的每步成本。
package main_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	_ "circuit/element/register"
	etime "circuit/element/time"
	"circuit/load"
	"circuit/maths"
)

// TestCPU8MacroScale5x 把 58 宏模型 CPU 复制 5 份，跑 0.5ms 测每步成本，
// 与原 54b×5（22834 节点、9.8s/步）对比。
func TestCPU8MacroScale5x(t *testing.T) {
	if testing.Short() {
		t.Skip("scale test 在 -short 下跳过")
	}
	t.Setenv("CIRCUIT_MAX_NODES", "30000")
	t.Setenv("CIRCUIT_LUSPARSE", "1")
	t.Setenv("CIRCUIT_GMCONT", "1")
	t.Setenv("CIRCUIT_GMINFINAL", "1e-5")
	t.Setenv("CIRCUIT_GLOBALGMIN", "1e-3")
	nl := readNetlist(t, "58_cpu8v2_macro.net")
	parts := 5
	var top strings.Builder
	for i := 1; i <= parts; i++ {
		fmt.Fprintf(&top, "X%d [clk", i)
		for j := 0; j < 8; j++ {
			fmt.Fprintf(&top, ",Q%c%d", 'a'+i-1, j)
		}
		top.WriteString("] cpu8\n")
	}
	for i := 1; i <= parts; i++ {
		for j := 0; j < 8; j++ {
			fmt.Fprintf(&top, "R%d [Q%c%d,-1] [10000]\n", 100+i*8+j, 'a'+i-1, j)
		}
	}
	for j := 0; j < 8; j++ {
		nl = strings.Replace(nl, fmt.Sprintf("R%d [Q%d,-1] [10000]\n", 14+j, j), "", 1)
	}
	nl = strings.Replace(nl, "X1 [clk,Q[0:7]] cpu8", top.String(), 1)

	con, err := load.LoadString(nl)
	if err != nil {
		t.Fatalf("加载 5 份宏模型 CPU 失败: %v", err)
	}
	con.Time, err = etime.NewTimeMNA(5e-4)
	if err != nil {
		t.Fatalf("time: %v", err)
	}
	if err := con.Time.SetStepLimits(1e-6, 5e-5); err != nil {
		t.Fatalf("步长: %v", err)
	}
	steps := 0
	maths.ResetKluStats()
	start := time.Now()
	if err := etime.TransientSimulation(con, func([]float64) { steps++ }); err != nil {
		t.Fatalf("仿真失败: %v", err)
	}
	wall := time.Since(start)
	luCount, luTime := maths.KluStatsSnapshot()
	if steps == 0 {
		t.Fatal("步数为 0")
	}
	t.Logf("宏模型×5：元件=%d 步数=%d 墙钟=%s 每步=%s 分解次数=%d 分解总耗时=%s 分解占比=%.1f%%",
		len(con.Nodelist), steps, wall, wall/time.Duration(steps), luCount, luTime,
		100*float64(luTime)/float64(wall))
}
