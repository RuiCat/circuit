package base

import (
	"circuit/element"
	"circuit/element/time"
	"circuit/mna"
	"math"
	"testing"
)

func TestTransformer(t *testing.T) {
	// 1. 设置参数
	freq := 50.0
	vAmp := 5.0
	ele := []element.NodeFace{
		element.NewElementVlaue(VoltageType, int(WfAC), 0.0, freq, 0.0, vAmp, 0.0, 0.0, 0.0),
		element.NewElementVlaue(ResistorType, 10.0),
		element.NewElementVlaue(TransformerType, 4.0, 1.0, 0.999),
		element.NewElementVlaue(ResistorType, 1000.0), // 增大负载电阻减小压降
	}

	// 设置引脚
	// 电压源：正极接节点0，负极接地
	ele[0].SetNodePins(0, -1)
	// 初级电阻：一端接节点0，另一端接节点1（变压器初级正极）
	ele[1].SetNodePins(0, 1)
	// 变压器：初级p1接节点1，p2接地；次级s1接节点2，s2接地
	ele[2].SetNodePins(1, -1, 2, -1)
	ele[2].SetVoltSource(0, 1) // 设置两个电压源ID
	// 次级负载电阻：一端接节点2，另一端接地
	ele[3].SetNodePins(2, -1)

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
