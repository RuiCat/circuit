// verify_cpu8_scale_test.go 用 54b 预加载 CPU 复制多份构造万级节点，测每步墙钟。
package main_test

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "circuit/element/register"
	etime "circuit/element/time"
	"circuit/load"
	"circuit/maths"
)

// TestCPU8Scale5x 把 54b 预加载 CPU 复制 5 份（约 1.1e4 节点），只跑 0.5ms 瞬态
// 测每步成本；不跑完整 24ms 程序（5 份完整运行会到小时级）。
func TestCPU8Scale5x(t *testing.T) {
	if testing.Short() {
		t.Skip("scale test 在 -short 下跳过")
	}
	t.Setenv("CIRCUIT_MAX_NODES", "30000")
	t.Setenv("CIRCUIT_LUSPARSE", "1")
	// 收敛参数允许外部覆盖（用于 A/B 对比），未设置时用 54b 基线值。
	setDefault := func(k, v string) {
		if os.Getenv(k) == "" {
			t.Setenv(k, v)
		}
	}
	setDefault("CIRCUIT_GMCONT", "1")
	setDefault("CIRCUIT_GMINFINAL", "1e-5")
	setDefault("CIRCUIT_GLOBALGMIN", "1e-3")
	nl := readNetlist(t, "54b_cpu8_preloaded.net")
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
	// 删除原来的 Q0..Q7 下拉（对应实例已被 5 份替换）。
	for j := 0; j < 8; j++ {
		nl = strings.Replace(nl, fmt.Sprintf("R%d [Q%d,-1] [10000]\n", 14+j, j), "", 1)
	}
	nl = strings.Replace(nl, "X1 [clk,Q[0:7]] cpu8", top.String(), 1)

	con, err := load.LoadString(nl)
	if err != nil {
		t.Fatalf("加载 5 份 CPU 网表失败: %v", err)
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
	maths.ResetMatrixStats()
	start := time.Now()
	if err := etime.TransientSimulation(con, func([]float64) { steps++ }); err != nil {
		t.Fatalf("仿真失败: %v", err)
	}
	wall := time.Since(start)
	luCount, luTime := maths.KluStatsSnapshot()
	getRowCount, getRowTime := maths.MatrixStatsSnapshot()
	setN, incN, getN := maths.MatrixOpsSnapshot()
	if steps == 0 {
		t.Fatal("步数为 0")
	}
	t.Logf("元件=%d 步数=%d 墙钟=%s 每步=%s 分解次数=%d 分解总耗时=%s 分解占比=%.1f%% GetRow次数=%d GetRow耗时=%s GetRow占比=%.1f%% Set=%d Inc=%d Get=%d",
		len(con.Nodelist), steps, wall, wall/time.Duration(steps), luCount, luTime,
		100*float64(luTime)/float64(wall), getRowCount, getRowTime,
		100*float64(getRowTime)/float64(wall), setN, incN, getN)
	if steps < 5 {
		t.Fatalf("步数过少: %d", steps)
	}
}
