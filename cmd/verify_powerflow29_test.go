// verify_powerflow29_test.go：C6 降级后的 29 潮流独立用例（只验形状，不做引擎逐位哨兵）。
package main_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestPowerflow29Shape 断言 29 走 CLI 潮流路径并输出 B/BR 表头（# bus_voltage），
// 不是节点电压 CSV；它不得被用作瞬态引擎的逐位回归。
func TestPowerflow29Shape(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "-format", "csv", "circuits/29_powerflow_3bus.net")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("29 潮流 CLI 失败: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "# bus_voltage") {
		t.Errorf("29 输出缺潮流表头 # bus_voltage（形状判据）:\n%s", out)
	}
}
