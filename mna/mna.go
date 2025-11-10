package mna

import (
	"circuit/graph"
	"circuit/mna/mat"
	"circuit/types"
)

// NewMNA 创建
func NewMNA(graph *graph.Graph) types.MNA {
	mna := &Soluv{
		Matrix:           &Matrix{Graph: graph},
		Value:            NewValue(graph),
		DampingFactor:    1.0,
		MmxDampingFactor: 1e8,
		MinDampingFactor: 1e-8,
	}
	// 初始化矩阵
	n := mna.NumNodes + mna.NumVoltageSources
	if n <= 0 {
		return nil
	}
	// 创建稀疏矩阵
	mna.OrigJ = mat.NewSparseMatrix(n, n)
	mna.MatJ = mat.NewUpdateMatrix(mna.OrigJ)
	mna.VecX[0] = mat.NewDenseVector(n)
	mna.VecX[1] = mat.NewDenseVector(n)
	mna.VecX[2] = mat.NewDenseVector(n)
	mna.VecB = mat.NewUpdateVector(mat.NewDenseVector(n))
	// 构建LU分解器
	mna.Lu = mat.NewLU(n)
	// 重置
	mna.Zero()
	return mna
}
