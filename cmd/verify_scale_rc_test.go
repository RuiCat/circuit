// verify_scale_rc_test.go 用 100×100 RC 网格（约 1e4 节点）验证引擎万级仿真与每步成本。
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

// TestRCScale1e4 构造 1e4 节点 RC 网格，跑 1ms 瞬态，测步数与每步墙钟。
func TestRCScale1e4(t *testing.T) {
	if testing.Short() {
		t.Skip("scale test 在 -short 下跳过")
	}
	t.Setenv("CIRCUIT_MAX_NODES", "30000")
	const width = 100
	var b strings.Builder
	b.WriteString("V1 [vcc,-1] [0,0,0,0,5]\n")
	for i := 0; i < width; i++ {
		for j := 0; j < width; j++ {
			id := i*width + j
			fmt.Fprintf(&b, "R%d [vcc,n%d] [1000]\n", 100000+id, id)
			fmt.Fprintf(&b, "C%d [n%d,-1] [1e-9]\n", 400000+id, id)
			if j+1 < width {
				fmt.Fprintf(&b, "R%d [n%d,n%d] [100]\n", 200000+id, id, id+1)
			}
			if i+1 < width {
				fmt.Fprintf(&b, "R%d [n%d,n%d] [100]\n", 300000+id, id, id+width)
			}
		}
	}
	con, err := load.LoadString(b.String())
	if err != nil {
		t.Fatalf("加载 RC 网格失败: %v", err)
	}
	con.Time, err = etime.NewTimeMNA(1e-4)
	if err != nil {
		t.Fatalf("time: %v", err)
	}
	if err := con.Time.SetStepLimits(1e-6, 1e-5); err != nil {
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
}
