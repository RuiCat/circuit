package base

import (
	"circuit/element"
	"circuit/element/time"
	"circuit/mna"
	"math"
	"testing"
)

func TestTransformer(t *testing.T) {
	netlist := `
	v1 0 -1 AC 0 50 0 5
	r1 0 1 10
	xfmr1 1 -1 2 -1 4.0 1.0 0.999
	r2 2 -1 1000
	`
	ele, err := element.LoadNetlistFromString(netlist)
	if err != nil {
		t.Fatalf("加载网表失败: %s", err)
	}

	// 创建求解
	mnaSolver := mna.NewUpdateMNA(time.GetNum(ele))
	timeMNA, err := time.NewTimeMNA(0.1)
	if err != nil {
		t.Fatalf("创建仿真时间失败 %s", err)
	}

	var maxV1, maxV2 float64

	// 4. 执行仿真
	time.TransientSimulation(timeMNA, mnaSolver, ele, func(voltages []float64) {
		// 记录节点 1 和 节点 2 的最大绝对值（峰值）
		v1 := math.Abs(mnaSolver.GetNodeVoltage(1))
		v2 := math.Abs(mnaSolver.GetNodeVoltage(2))
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
