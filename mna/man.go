package mna

import (
	"circuit/maths"
	"fmt"
)

// updateMNA 带更新的矩阵
type updateMNA struct {
	MNA
	A maths.UpdateMatrix // 求解矩阵
	Z maths.UpdateVector // 已知向量
	X maths.UpdateVector // 未知向量
}

// Zero 重置
func (mna *updateMNA) Zero() {
	mna.A.Clear()
	mna.Z.Clear()
	mna.X.Clear()
}

// Update 更新数据到底层
func (mna *updateMNA) Update() {
	mna.Z.Update()
	mna.X.Update()
}

// Rollback 放弃数据
func (mna *updateMNA) Rollback() {
	mna.Z.Rollback()
	mna.X.Rollback()
}

// NewUpdateMNA 创建更新求解矩阵
func NewUpdateMNA(NodesNum, VoltageSourcesNum int) UpdateMNA {
	n := NodesNum + VoltageSourcesNum
	a, z, x := maths.NewDenseMatrix(n, n), maths.NewDenseVector(n), maths.NewDenseVector(n)
	return &updateMNA{
		MNA: &mna{
			A:                 a,
			Z:                 z,
			X:                 x,
			NodesNum:          NodesNum,
			VoltageSourcesNum: VoltageSourcesNum,
		},
		A: maths.NewUpdateMatrixPtr(a),
		Z: maths.NewUpdateVectorPtr(z),
		X: maths.NewUpdateVectorPtr(x),
	}
}

// mna 矩阵操作
// x = A⁻¹z
type mna struct {
	A                 maths.Matrix // 求解矩阵
	Z                 maths.Vector // 已知向量
	X                 maths.Vector // 未知向量
	NodesNum          int          // 电路节点数量
	VoltageSourcesNum int          // 独立电压源数量
}

// NewMNA 创建求解矩阵
func NewMNA(NodesNum, VoltageSourcesNum int) MNA {
	n := NodesNum + VoltageSourcesNum
	return &mna{
		A:                 maths.NewDenseMatrix(n, n),
		Z:                 maths.NewDenseVector(n),
		X:                 maths.NewDenseVector(n),
		NodesNum:          NodesNum,
		VoltageSourcesNum: VoltageSourcesNum,
	}
}
func (mna *mna) GetA() maths.Matrix { return mna.A }
func (mna *mna) GetZ() maths.Vector { return mna.Z }
func (mna *mna) GetX() maths.Vector { return mna.X }

// Zero 重置
func (mna *mna) Zero() {
	mna.A.Clear()
	mna.Z.Clear()
	mna.X.Clear()
}

// GetNodesNum 获取电路节点数量
func (mna *mna) GetNodesNum() int { return mna.NodesNum }

// GetVoltageSourcesNum 获取独立电压源数量
func (mna *mna) GetVoltageSourcesNum() int { return mna.VoltageSourcesNum }

// GetVoltage 获取节点电压
func (mna *mna) GetVoltage(i NodeID) float64 {
	if i == Gnd {
		return 0
	}
	return mna.X.Get(i)
}

// GetCurrent 获取电压源电流
func (mna *mna) GetCurrent(vs NodeID) float64 {
	if vs == Gnd {
		return 0
	}
	return mna.X.Get(vs + mna.NodesNum)
}

// StampMatrix 在矩阵A的(i,j)位置叠加值
func (mna *mna) StampMatrix(i, j NodeID, value float64) {
	// Gnd节点(-1)不参与矩阵计算
	if i == Gnd || j == Gnd {
		return
	}
	mna.A.Increment(i, j, value)
}

// StampMatrixSet 在矩阵A的(i,j)位置设置值
func (mna *mna) StampMatrixSet(i, j NodeID, v float64) {
	// Gnd节点(-1)不参与矩阵计算
	if i == Gnd || j == Gnd {
		return
	}
	mna.A.Set(i, j, v)
}

// StampRightSide 在右侧向量B的i位置叠加值
func (mna *mna) StampRightSide(i NodeID, value float64) {
	// Gnd节点(-1)不参与矩阵计算
	if i == Gnd {
		return
	}
	mna.Z.Increment(i, value)
}

// StampRightSideSet 在右侧向量B的i位置设置值
func (mna *mna) StampRightSideSet(i NodeID, v float64) {
	// Gnd节点(-1)不参与矩阵计算
	if i == Gnd {
		return
	}
	mna.Z.Set(i, v)
}

// StampResistor 加盖电阻元件
// 根据MNA算法，电阻在G矩阵中的贡献为：
// - 对角元素：+1/R
// - 非对角元素：-1/R
func (mna *mna) StampResistor(n1, n2 NodeID, r float64) {
	if r == 0 {
		return
	}
	g := 1.0 / r // 电导
	// 对角元素
	if n1 != Gnd {
		mna.StampMatrix(n1, n1, g)
	}
	if n2 != Gnd {
		mna.StampMatrix(n2, n2, g)
	}
	// 非对角元素
	if n1 != Gnd && n2 != Gnd {
		mna.StampMatrix(n1, n2, -g)
		mna.StampMatrix(n2, n1, -g)
	}
}

// StampConductance 加盖电导元件
func (mna *mna) StampConductance(n1, n2 NodeID, g float64) {
	// 对角元素
	if n1 != Gnd {
		mna.StampMatrix(n1, n1, g)
	}
	if n2 != Gnd {
		mna.StampMatrix(n2, n2, g)
	}
	// 非对角元素
	if n1 != Gnd && n2 != Gnd {
		mna.StampMatrix(n1, n2, -g)
		mna.StampMatrix(n2, n1, -g)
	}
}

// StampCurrentSource 加盖电流源
// 电流源在右侧向量中的贡献：
// - 流出节点：-I
// - 流入节点：+I
func (mna *mna) StampCurrentSource(n1, n2 NodeID, i float64) {
	if n1 != Gnd {
		mna.StampRightSide(n1, -i) // 电流从n1流出
	}
	if n2 != Gnd {
		mna.StampRightSide(n2, i) // 电流流入n2
	}
}

// StampVoltageSource 加盖电压源
// 电压源在MNA矩阵中的贡献：
// - B矩阵：+1在n1行，-1在n2行
// - C矩阵：+1在n1列，-1在n2列
// - 右侧向量：电压源值在对应位置
func (mna *mna) StampVoltageSource(n1, n2 NodeID, vs NodeID, v float64) {
	// 电压源在矩阵中的位置
	vsRow := vs + mna.NodesNum // 电压源在矩阵下半部分
	// B矩阵部分：电压源对节点电流的贡献
	if n1 != Gnd {
		mna.StampMatrix(n1, vsRow, 1.0)
	}
	if n2 != Gnd {
		mna.StampMatrix(n2, vsRow, -1.0)
	}
	// C矩阵部分：电压源方程
	mna.StampMatrix(vsRow, n1, 1.0)
	mna.StampMatrix(vsRow, n2, -1.0)
	// 右侧向量：电压源值
	mna.StampRightSideSet(vsRow, v)
}

// StampVCVS 加盖电压控制电压源
// (v_N+ - v_N-) = coef * (v_NC+ - v_NC-)
func (mna *mna) StampVCVS(n1, n2 NodeID, vs NodeID, coef float64) {
	// 电压源在矩阵中的位置
	vsRow := vs + mna.NodesNum
	// B矩阵部分
	if n1 != Gnd {
		mna.StampMatrix(n1, vsRow, 1.0)
	}
	if n2 != Gnd {
		mna.StampMatrix(n2, vsRow, -1.0)
	}
	// C矩阵部分：VCVS方程
	// (v_N+ - v_N-) + coef*(v_NC- - v_NC+) = 0
	mna.StampMatrix(vsRow, n1, 1.0)
	mna.StampMatrix(vsRow, n2, -1.0)
	mna.StampMatrix(vsRow, vs, -coef) // 控制电压源的正端
	mna.StampMatrix(vsRow, vs, coef)  // 控制电压源的负端
}

// StampCCCS 加盖电流控制电流源
// 输出电流 = gain * 控制电流
func (mna *mna) StampCCCS(n1, n2 NodeID, vs NodeID, gain float64) {
	// 控制电压源在矩阵中的位置
	vsCol := vs + mna.NodesNum
	// 在B矩阵中修改控制电压源对应的列
	if n1 != Gnd {
		mna.StampMatrix(n1, vsCol, gain)
	}
	if n2 != Gnd {
		mna.StampMatrix(n2, vsCol, -gain)
	}
}

// StampVCCurrentSource 加盖电压控制电流源
// 输出电流 = gain * (v_vn1 - v_vn2)
func (mna *mna) StampVCCurrentSource(cn1, cn2, vn1, vn2 NodeID, gain float64) {
	// 在G矩阵中修改
	// 输出节点cn1的方程
	if cn1 != Gnd {
		if vn1 != Gnd {
			mna.StampMatrix(cn1, vn1, gain)
		}
		if vn2 != Gnd {
			mna.StampMatrix(cn1, vn2, -gain)
		}
	}
	// 输出节点cn2的方程
	if cn2 != Gnd {
		if vn1 != Gnd {
			mna.StampMatrix(cn2, vn1, -gain)
		}
		if vn2 != Gnd {
			mna.StampMatrix(cn2, vn2, gain)
		}
	}
}

// UpdateVoltageSource 更新电压源值
func (mna *mna) UpdateVoltageSource(vs NodeID, v float64) {
	if vs > Gnd {
		mna.StampRightSideSet(NodeID(v)+mna.NodesNum, v)
	}
}

// String 格式化输出
func (mna *mna) String() string {
	return fmt.Sprintf("A:\n%s\nX:%s\nZ:%s\n", mna.A, mna.X, mna.Z)
}
