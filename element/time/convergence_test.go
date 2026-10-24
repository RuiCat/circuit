// convergence_test.go：收敛鲁棒性矩阵（m6，c8）。
package time

import (
	"os"
	"testing"

	"circuit/element"
	"circuit/load"
)

// TestConvergence 对固定矩阵（RC/二极管/带源切换）跑短瞬态，统计失败率。
// 判据：3 个用例失败数 ≤ 1；失败用例必须返回 error（不得静默假成功）。
func TestConvergence(t *testing.T) {
	t.Setenv("CIRCUIT_LUSPARSE", "1")
	t.Setenv("CIRCUIT_GMCONT", "1")
	t.Setenv("CIRCUIT_GMINFINAL", "1e-5")
	t.Setenv("CIRCUIT_GLOBALGMIN", "1e-3")

	cases := []string{
		"../../cmd/circuits/01_resistor_divider.net",
		"../../cmd/circuits/02_rc_filter.net",
		"../../cmd/circuits/03_diode_rectifier.net",
	}
	failed := 0
	for _, path := range cases {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", path, err)
		}
		con, err := load.LoadString(string(data))
		if err != nil {
			t.Fatalf("加载 %s 失败: %v", path, err)
		}
		if err := runTransient(con, 5e-4); err != nil {
			t.Logf("%s 未收敛（计为失败）: %v", path, err)
			failed++
		}
	}
	if failed > 1 {
		t.Errorf("收敛矩阵失败率过高: %d/%d", failed, len(cases))
	}
}

// runTransient 跑 tStop 的瞬态；返回首个错误（不静默吞掉）。
func runTransient(con *element.Context, tStop float64) error {
	var err error
	con.Time, err = NewTimeMNA(tStop)
	if err != nil {
		return err
	}
	if err := con.Time.SetStepLimits(1e-9, 1e-5); err != nil {
		return err
	}
	return TransientSimulation(con, func([]float64) {})
}
