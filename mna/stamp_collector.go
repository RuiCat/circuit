package mna

import (
	"circuit/maths"
)

// StampOp 盖章操作类型枚举
type StampOp uint8

const (
	OpAdmittance             StampOp = iota // 导纳操作
	OpImpedance                             // 阻抗操作
	OpCurrentSource                         // 电流源操作
	OpVoltageSource                         // 电压源操作
	OpVCVS                                  // 压控电压源操作
	OpCCCS                                  // 流控电流源操作
	OpCCVS                                  // 流控电压源操作
	OpVCCS                                  // 压控电流源操作
	OpMatrix                                // 矩阵元素累加操作
	OpMatrixSet                             // 矩阵元素设置操作
	OpRightSide                             // 右侧向量累加操作
	OpRightSideSet                          // 右侧向量设置操作
	OpUpdateVoltageSource                   // 更新电压源值操作
	OpIncrementVoltageSource                // 增量更新电压源值操作
)

// RecordedStamp 记录的一次盖章操作，包含操作类型、节点索引和值
type RecordedStamp struct {
	Op             StampOp
	N1, N2, N3, N4 NodeID
	ID1, ID2       VoltageID
	Value          float64
}

// StampCollector 盖章记录器，收集元件在DoStep期间的盖章操作并延迟执行
type StampCollector struct {
	Inner     MNAFace[float64]
	Records   []RecordedStamp
	ReadNodes map[NodeID]struct{}
	ReadVsrcs map[VoltageID]struct{}
}

// NewStampCollector 创建一个新的盖章记录器
func NewStampCollector(inner MNAFace[float64]) *StampCollector {
	return &StampCollector{
		Inner:     inner,
		ReadNodes: make(map[NodeID]struct{}),
		ReadVsrcs: make(map[VoltageID]struct{}),
	}
}

// GetNodeVoltage 获取节点电压并记录读取依赖
func (sc *StampCollector) GetNodeVoltage(id NodeID) float64 {
	sc.ReadNodes[id] = struct{}{}
	return sc.Inner.GetNodeVoltage(id)
}

// GetVoltageSourceCurrent 获取电压源电流并记录读取依赖
func (sc *StampCollector) GetVoltageSourceCurrent(id VoltageID) float64 {
	sc.ReadVsrcs[id] = struct{}{}
	return sc.Inner.GetVoltageSourceCurrent(id)
}

func (sc *StampCollector) GetA() maths.Matrix[float64] {
	return sc.Inner.GetA()
}

func (sc *StampCollector) GetZ() maths.Vector[float64] {
	return sc.Inner.GetZ()
}

func (sc *StampCollector) GetX() maths.Vector[float64] {
	return sc.Inner.GetX()
}

func (sc *StampCollector) String() string {
	return sc.Inner.String()
}

func (sc *StampCollector) Zero() {
	sc.Inner.Zero()
}

func (sc *StampCollector) GetNodeNum() int {
	return sc.Inner.GetNodeNum()
}

func (sc *StampCollector) GetVoltageSourcesNum() int {
	return sc.Inner.GetVoltageSourcesNum()
}

// StampMatrix 记录矩阵元素累加操作
func (sc *StampCollector) StampMatrix(i, j NodeID, value float64) {
	sc.Records = append(sc.Records, RecordedStamp{Op: OpMatrix, N1: i, N2: j, Value: value})
}

// StampMatrixSet 记录矩阵元素设置操作
func (sc *StampCollector) StampMatrixSet(i, j NodeID, value float64) {
	sc.Records = append(sc.Records, RecordedStamp{Op: OpMatrixSet, N1: i, N2: j, Value: value})
}

// StampRightSide 记录右侧向量累加操作
func (sc *StampCollector) StampRightSide(node NodeID, value float64) {
	sc.Records = append(sc.Records, RecordedStamp{Op: OpRightSide, N1: node, Value: value})
}

// StampRightSideSet 记录右侧向量设置操作
func (sc *StampCollector) StampRightSideSet(node NodeID, value float64) {
	sc.Records = append(sc.Records, RecordedStamp{Op: OpRightSideSet, N1: node, Value: value})
}

// StampImpedance 记录阻抗操作
func (sc *StampCollector) StampImpedance(n1, n2 NodeID, resistance float64) {
	sc.Records = append(sc.Records, RecordedStamp{Op: OpImpedance, N1: n1, N2: n2, Value: resistance})
}

// StampAdmittance 记录导纳操作
func (sc *StampCollector) StampAdmittance(n1, n2 NodeID, admittance float64) {
	sc.Records = append(sc.Records, RecordedStamp{Op: OpAdmittance, N1: n1, N2: n2, Value: admittance})
}

// StampCurrentSource 记录电流源操作
func (sc *StampCollector) StampCurrentSource(n1, n2 NodeID, current float64) {
	sc.Records = append(sc.Records, RecordedStamp{Op: OpCurrentSource, N1: n1, N2: n2, Value: current})
}

// StampVoltageSource 记录电压源操作
func (sc *StampCollector) StampVoltageSource(n1, n2 NodeID, id VoltageID, voltage float64) {
	sc.Records = append(sc.Records, RecordedStamp{Op: OpVoltageSource, N1: n1, N2: n2, ID1: id, Value: voltage})
}

// StampVCVS 记录压控电压源操作
func (sc *StampCollector) StampVCVS(on1, on2, cn1, cn2 NodeID, id VoltageID, gain float64) {
	sc.Records = append(sc.Records, RecordedStamp{Op: OpVCVS, N1: on1, N2: on2, N3: cn1, N4: cn2, ID1: id, Value: gain})
}

// StampCCCS 记录流控电流源操作
func (sc *StampCollector) StampCCCS(n1, n2 NodeID, controlVSID VoltageID, gain float64) {
	sc.Records = append(sc.Records, RecordedStamp{Op: OpCCCS, N1: n1, N2: n2, ID1: controlVSID, Value: gain})
}

// StampCCVS 记录流控电压源操作
func (sc *StampCollector) StampCCVS(on1, on2 NodeID, controlVSID, id VoltageID, gain float64) {
	sc.Records = append(sc.Records, RecordedStamp{Op: OpCCVS, N1: on1, N2: on2, ID1: controlVSID, ID2: id, Value: gain})
}

// StampVCCS 记录压控电流源操作
func (sc *StampCollector) StampVCCS(cn1, cn2, vn1, vn2 NodeID, gain float64) {
	sc.Records = append(sc.Records, RecordedStamp{Op: OpVCCS, N1: cn1, N2: cn2, N3: vn1, N4: vn2, Value: gain})
}

// UpdateVoltageSource 记录电压源更新操作
func (sc *StampCollector) UpdateVoltageSource(id VoltageID, voltage float64) {
	sc.Records = append(sc.Records, RecordedStamp{Op: OpUpdateVoltageSource, ID1: id, Value: voltage})
}

// IncrementVoltageSource 记录电压源增量更新操作
func (sc *StampCollector) IncrementVoltageSource(id VoltageID, increment float64) {
	sc.Records = append(sc.Records, RecordedStamp{Op: OpIncrementVoltageSource, ID1: id, Value: increment})
}

// Flush 将收集的所有盖章操作应用到目标对象
func (sc *StampCollector) Flush(target Stamp[float64]) {
	for _, r := range sc.Records {
		switch r.Op {
		case OpAdmittance:
			target.StampAdmittance(r.N1, r.N2, r.Value)
		case OpImpedance:
			target.StampImpedance(r.N1, r.N2, r.Value)
		case OpCurrentSource:
			target.StampCurrentSource(r.N1, r.N2, r.Value)
		case OpVoltageSource:
			target.StampVoltageSource(r.N1, r.N2, r.ID1, r.Value)
		case OpVCVS:
			target.StampVCVS(r.N1, r.N2, r.N3, r.N4, r.ID1, r.Value)
		case OpCCCS:
			target.StampCCCS(r.N1, r.N2, r.ID1, r.Value)
		case OpCCVS:
			target.StampCCVS(r.N1, r.N2, r.ID1, r.ID2, r.Value)
		case OpVCCS:
			target.StampVCCS(r.N1, r.N2, r.N3, r.N4, r.Value)
		case OpMatrix:
			target.StampMatrix(r.N1, r.N2, r.Value)
		case OpMatrixSet:
			target.StampMatrixSet(r.N1, r.N2, r.Value)
		case OpRightSide:
			target.StampRightSide(r.N1, r.Value)
		case OpRightSideSet:
			target.StampRightSideSet(r.N1, r.Value)
		case OpUpdateVoltageSource:
			target.UpdateVoltageSource(r.ID1, r.Value)
		case OpIncrementVoltageSource:
			target.IncrementVoltageSource(r.ID1, r.Value)
		}
	}
}

// FlushToMNA 将收集的盖章操作应用到MNA求解器
func (sc *StampCollector) FlushToMNA(mna *MnaUpdateType[float64]) {
	sc.Flush(mna)
}

// Reset 清空已记录的所有盖章操作
func (sc *StampCollector) Reset() {
	sc.Records = sc.Records[:0]
	sc.ReadNodes = make(map[NodeID]struct{})
	sc.ReadVsrcs = make(map[VoltageID]struct{})
}
