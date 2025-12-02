package element

import (
	"circuit/mna"
	"circuit/utils"
	"fmt"
)

// GetEventValue 得到底层值
func GetEventValue(val utils.EventValue) *EventValue {
	return val.(*EventValue)
}

// GetNum 得到节点数量
func GetNum(lise []utils.EventValue) (NodesNum, VoltageSourcesNum int) {
	nodesNum, voltageSourcesNum := make(map[mna.NodeID]struct{}), make(map[mna.NodeID]struct{})
	for i := range len(lise) {
		base := GetEventValue(lise[i]).Value.Base.Base().Graph
		for _, i := range base.Nodes {
			nodesNum[i] = struct{}{}
		}
		for _, i := range base.NodesInternal {
			nodesNum[i] = struct{}{}
		}
		for _, i := range base.VoltSource {
			voltageSourcesNum[i] = struct{}{}
		}
	}
	return len(nodesNum) - 1, len(voltageSourcesNum)
}

// GetElementBase 得到底层
func GetElementBase(val utils.EventValue) *mna.ElementBase {
	return (val.(*EventValue)).Value.ElementBase()
}

// GetElementBase 得到底层
func GetElementBaseString(val utils.EventValue) string {
	value := GetEventValue(val).Value.Base
	base := value.Base()
	return fmt.Sprintf("Pin:%s Voltage:%s Internal:%s Value:%v Nodes:%v VoltSource:%v NodesInternal:%v", base.Pin, base.Voltage, base.Internal, base.ElementConfigBase.Value,
		base.Graph.Nodes, base.Graph.VoltSource, base.Graph.NodesInternal)
}

// SetMNA 设置求解矩阵接口
func SetMNA(val utils.EventValue, mna mna.MNA) {
	GetEventValue(val).Value.MNA = mna
}

// SetNodes 设置节点索引
func SetNodes(val utils.EventValue, n ...mna.NodeID) {
	GetElementBase(val).Graph.Nodes = n
}

// SetVoltSource 设置电压索引
func SetVoltSource(val utils.EventValue, n ...mna.NodeID) {
	GetElementBase(val).Graph.VoltSource = n
}

// SetNodesInternal 设置指定内部索引
func SetNodesInternal(val utils.EventValue, n ...mna.NodeID) {
	GetElementBase(val).Graph.NodesInternal = n
}

// EventSendValue 发送
func EventSendValue(cxt utils.Context, mark utils.EventMark, list ...utils.EventValue) {
	for i := range len(list) {
		// 设置
		list[i].SetMark(mark)
		// 传递
		cxt.EventSendValue(list[i])
	}
}

// Callback 调用
func Callback(cxt utils.Context, mark utils.EventMark, list ...utils.EventValue) {
	for i := range list {
		// 设置
		list[i].SetMark(mark)
		// 传递
		cxt.Callback(list[i])
	}
}

// Update 更新数据到底层
func UpdateElements(mna mna.UpdateMNA, ele []utils.EventValue) {
	mna.UpdateX()
	for i := range ele {
		GetElementBase(ele[i]).Update()
	}
}

// Rollback 放弃数据
func RollbackElements(mna mna.UpdateMNA, ele []utils.EventValue) {
	mna.RollbackX()
	for i := range ele {
		GetElementBase(ele[i]).Rollback()
	}
}
