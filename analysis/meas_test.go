// meas_test.go：.meas 单测（m3，c3）。
package analysis

import "testing"

// TestMeas 验证 AVG/MAX/MIN/PP/FIND。
func TestMeas(t *testing.T) {
	signals := map[string][]float64{"V(out)": {0, 1, 2, 3, 4}}
	times := []float64{0, 1, 2, 3, 4}
	text := `
.meas tran vavg AVG V(out)
.meas tran vmax MAX V(out)
.meas tran vmin MIN V(out)
.meas tran vpp PP V(out)
.meas tran vat FIND V(out) AT=2.4
`
	results, err := RunMeas(text, signals, times)
	if err != nil {
		t.Fatalf("RunMeas 失败: %v", err)
	}
	want := map[string]float64{"vavg": 2, "vmax": 4, "vmin": 0, "vpp": 4, "vat": 2}
	if len(results) != len(want) {
		t.Fatalf("结果数=%d，期望 %d", len(results), len(want))
	}
	for _, r := range results {
		if r.Value != want[r.Name] {
			t.Errorf("%s=%v，期望 %v", r.Name, r.Value, want[r.Name])
		}
	}
	if _, err := RunMeas(".meas tran bad NOPE V(out)", signals, times); err == nil {
		t.Errorf("不支持的函数应报错")
	}
	if _, err := RunMeas(".meas tran x AVG V(nope)", signals, times); err == nil {
		t.Errorf("未知信号应报错")
	}
}
