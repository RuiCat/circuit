package base

import (
	"bufio"
	"circuit/element"
	"circuit/element/time"
	"math"
	"strings"
	"testing"
)

func TestTransformer(t *testing.T) {
	netlist := `
	v1 0 -1 AC 0 50 0 5
	r1 0 1 10
	xfmr1 1 -1 2 -1 4.0 1.0 0.999
	r2 2 -1 1000
	`
	scanner := bufio.NewScanner(strings.NewReader(netlist))
	con, err := element.LoadContext(scanner)
	if err != nil {
		t.Fatalf("加载上下文失败: %s", err)
	}
	con.Time, err = time.NewTimeMNA(0.1)
	if err != nil {
		t.Fatalf("创建仿真时间失败 %s", err)
	}

	var maxV1, maxV2 float64

	// 4. 执行仿真
	time.TransientSimulation(con, func(voltages []float64) {
		// 记录节点 1 和 节点 2 的最大绝对值（峰值）
		v1 := math.Abs(con.GetNodeVoltage(1))
		v2 := math.Abs(con.GetNodeVoltage(2))
		if v1 > maxV1 {
			maxV1 = v1
		}
		if v2 > maxV2 {
			maxV2 = v2
		}
	})

	ratio := maxV2 - maxV1
	if ratio > 0.001 {
		t.Errorf("变压器耦合效率太低: %.4f", ratio)
	}

}
