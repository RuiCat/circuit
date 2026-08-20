package time

import (
	"math"
	"testing"

	_ "circuit/element/register"
	"circuit/load"
)

// TestSparseLUMatchesDense 验证稀疏 LU 模式（CIRCUIT_LUSPARSE）与稠密模式解一致。
// 覆盖含二极管（近奇异：Rs=0 用 1e9 近似短路）的电路。
func TestSparseLUMatchesDense(t *testing.T) {
	netlist := `
V1 [1,-1] [0,5,0,0,0]
R1 [1,2] [1000]
D1 [2,-1] [1e-12,0,1,0,300.15]
V2 [3,-1] [1,0,1000,0,0.1]
R2 [3,2] [100000]
`
	run := func(sparse bool) float64 {
		con, err := load.LoadString(netlist)
		if err != nil {
			t.Fatalf("加载网表失败: %v", err)
		}
		con.Time, err = NewTimeMNA(1e-3)
		if err != nil {
			t.Fatalf("创建仿真时间失败: %v", err)
		}
		if sparse {
			t.Setenv("CIRCUIT_LUSPARSE", "1")
		} else {
			t.Setenv("CIRCUIT_LUSPARSE", "")
		}
		var v2 float64
		if err := TransientSimulation(con, func(voltages []float64) {
			// 节点 2 是二极管阴极（输出），按紧凑索引记录最后一个值
			if len(voltages) > 2 {
				v2 = voltages[2]
			}
		}); err != nil {
			t.Fatalf("仿真失败(sparse=%v): %v", sparse, err)
		}
		return v2
	}

	dense := run(false)
	sparse := run(true)
	if math.Abs(dense-sparse) > 1e-6 {
		t.Fatalf("稀疏/稠密解不一致: dense=%g sparse=%g", dense, sparse)
	}
}
