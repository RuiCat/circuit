package mna

import (
	"circuit/graph"
	"circuit/mna/mat"
	"circuit/types"
	"math"
)

// Matrix 矩阵内容
type Matrix struct {
	// 图表信息
	*graph.Graph
	// 稀疏矩阵系统
	MatJ  mat.UpdateMatrix // 动态矩阵（基于位图缓存）
	OrigJ mat.Matrix       // 线性贡献
	// 备份实现
	VecX mat.UpdateVector // 未知量向量(节点电压+支路电流)
	VecB mat.UpdateVector // 右侧激励向量
	// LU分解
	Lu mat.LU // LU分解器
}

// 在矩阵A的(i,j)位置叠加值
func (mna *Matrix) StampMatrix(i, j types.NodeID, v float64) {
	if i > types.ElementGndNodeID && j > types.ElementGndNodeID {
		mna.MatJ.Increment(i, j, v)
	}
}

// 在右侧向量B的i位置叠加值
func (mna *Matrix) StampRightSide(i types.NodeID, v float64) {
	if i > types.ElementGndNodeID {
		mna.VecB.Increment(i, v)
	}
}

// 加盖电阻元件
func (mna *Matrix) StampResistor(n1, n2 types.NodeID, r float64) {
	mna.StampConductance(n1, n2, 1.0/math.Max(r, 1e-12))
}

// 加盖电导元件
func (mna *Matrix) StampConductance(n1, n2 types.NodeID, g float64) {
	mna.StampMatrix(n1, n1, g)
	mna.StampMatrix(n2, n2, g)
	mna.StampMatrix(n1, n2, -g)
	mna.StampMatrix(n2, n1, -g)
}

// 加盖电流源
func (mna *Matrix) StampCurrentSource(n1, n2 types.NodeID, i float64) {
	mna.StampRightSide(n1, -i)
	mna.StampRightSide(n2, i)
}

// 加盖电压源
func (mna *Matrix) StampVoltageSource(n1, n2 types.NodeID, vs types.VoltageID, v float64) {
	vn := mna.NumNodes + vs
	mna.StampMatrix(vn, n1, -1)
	mna.StampMatrix(vn, n2, 1)
	mna.StampRightSide(vn, v)
	mna.StampMatrix(n1, vn, 1)
	mna.StampMatrix(n2, vn, -1)
}

// 更新电压源值
func (mna *Matrix) UpdateVoltageSource(vs types.VoltageID, v float64) {
	mna.StampRightSide(mna.NumNodes+vs, v)
}

// StampVCVS 加盖电压控制电压源
func (mna *Matrix) StampVCVS(n1, n2 types.NodeID, vs types.VoltageID, coef float64) {
	vn := mna.NumNodes + vs
	mna.StampMatrix(vn, n1, coef)
	mna.StampMatrix(vn, n2, -coef)
}

// StampVCCurrentSource 加盖电压控制电流源
func (mna *Matrix) StampVCCurrentSource(cn1, cn2, vn1, vn2 types.NodeID, gain float64) {
	// 控制电压差
	mna.StampMatrix(cn1, vn1, gain)
	mna.StampMatrix(cn2, vn2, gain)
	mna.StampMatrix(cn1, vn2, -gain)
	mna.StampMatrix(cn2, vn1, -gain)
}

// StampCCCS 加盖电流控制电流源
func (mna *Matrix) StampCCCS(n1, n2 types.NodeID, vs types.VoltageID, gain float64) {
	vn := mna.NumNodes + vs
	mna.StampMatrix(n1, vn, gain)
	mna.StampMatrix(n2, vn, -gain)
}

// SetValue 设置元件的值
func (mna *Matrix) SetValue(id types.ElementID, value types.ValueMap) {
	if v, ok := mna.ElementList[id]; ok {
		v.Value.SetValue(value)
	}
}

// SetConverged 标记元件无法收敛
func (mna *Matrix) SetConverged() {
	mna.Converged = false
}

// GetGraph 获取底层
func (mna *Matrix) GetGraph() *types.ElementGraph {
	return &mna.ElementGraph
}

// GetValue 得到元件的值
func (mna *Matrix) GetValue(id types.ElementID) (value types.ValueMap) {
	if v, ok := mna.ElementList[id]; ok {
		value = v.Value.GetValue()
	}
	return value
}

// 返回电路节点数量,不包含电压数量
func (mna *Matrix) GetNumNodes() int {
	return mna.NumNodes
}

// 返回电路电压数量
func (mna *Matrix) GetNumVoltage() int {
	return mna.NumVoltageSources
}

// 返回节点电压
func (mna *Matrix) GetVoltage(i types.NodeID) float64 {
	switch {
	case i == types.ElementGndNodeID:
		return 0
	case i >= 0 && i < mna.NumNodes:
		return mna.VecX.Get(i)
	}
	return 0
}

// 设置节点电压
func (mna *Matrix) SetVoltage(i types.NodeID, v float64) {
	if i > types.ElementGndNodeID && i < mna.NumNodes {
		mna.VecX.Increment(i, v)
	}
}
