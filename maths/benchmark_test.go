package maths

// benchmark_test.go —— 算子级基准（K1 的 group m2/m3 判定口径）。
//
// 既有 7 个 Benchmark 分散在 lu_test.go / vector_test.go / matrixUpdate_test.go /
// matrixReducer_test.go，**不得删除或改名**（下游按名单引用）；本文件只补齐缺失方向：
// 稀疏 LU 分解/求解、rowSparse 的 Set/SetAdd/Get、向量 Increment、updateMatrix Rollback、
// 最小度重排。所有输入走 benchlib_test.go 的固定种子语料构造（同一 -count=N 的 N 次重复测同一份数据）。
//
// 注意：luSparse 的静态重排有 permSig 缓存，Decompose 基准测的是"结构不变的稳态"路径；
// 重排本身由 BenchmarkMinimumDegreeOrder 单独计量。

import "testing"

const (
	benchDenseN   = 120 // 稠密算子语料规模
	benchSparseN  = 600 // 稀疏算子语料规模（≥500 → 走最小度重排分支）
	benchSparseBW = 6   // 带状半带宽
	benchSparseHB = 4   // hub 行数量
	benchVectorN  = 1000
	benchUpdateN  = 200
)

// benchSparseCorpus 返回稀疏算子共用的固定语料（含 hub 行，结构固定）。
func benchSparseCorpus() *rowSparseMatrix[float64] {
	return benchlibSparseHub(benchSparseN, benchSparseBW, benchSparseHB, 20261012)
}

// BenchmarkLuSparseDecompose 稀疏 LU 分解（结构不变，permSig 命中后的稳态路径）。
func BenchmarkLuSparseDecompose(b *testing.B) {
	m := benchSparseCorpus()
	lu, err := NewLUSparse[float64](m.Rows())
	if err != nil {
		b.Fatalf("NewLUSparse 失败: %v", err)
	}
	if err := lu.Decompose(m); err != nil {
		b.Fatalf("首次分解失败: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := lu.Decompose(m); err != nil {
			b.Fatalf("分解失败: %v", err)
		}
	}
}

// BenchmarkLuSparseSolve 稀疏 LU 回代求解（分解已在计时外完成）。
func BenchmarkLuSparseSolve(b *testing.B) {
	m := benchSparseCorpus()
	n := m.Rows()
	lu, err := NewLUSparse[float64](n)
	if err != nil {
		b.Fatalf("NewLUSparse 失败: %v", err)
	}
	if err := lu.Decompose(m); err != nil {
		b.Fatalf("首次分解失败: %v", err)
	}
	rhs := NewDenseVector[float64](n)
	x := NewDenseVector[float64](n)
	for i := 0; i < n; i++ {
		rhs.Set(i, float64(i%7)+1)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := lu.SolveReuse(rhs, x); err != nil {
			b.Fatalf("求解失败: %v", err)
		}
	}
}

// benchSparsePositions 预取每行的若干列号，供 Set/SetAdd/Get 基准循环复用（避免循环内分配）。
type benchSparsePosition struct {
	row, col int
}

func benchSparsePositions(m *rowSparseMatrix[float64], perRow int) []benchSparsePosition {
	pos := make([]benchSparsePosition, 0, m.rows*perRow)
	for i := 0; i < m.rows; i++ {
		cols, _ := m.GetRow(i)
		for k := 0; k < perRow && k < len(cols); k++ {
			pos = append(pos, benchSparsePosition{row: i, col: cols[k]})
		}
	}
	return pos
}

// BenchmarkRowSparseSet 稀疏矩阵 Set 已有元素（含列索引同步）。
func BenchmarkRowSparseSet(b *testing.B) {
	m := benchlibSparseBand(400, 8, 4242)
	pos := benchSparsePositions(m, 5)
	if len(pos) == 0 {
		b.Fatal("语料为空")
	}
	vals := make([]float64, 64)
	for i := range vals {
		vals[i] = float64(i) + 1
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		p := pos[i%len(pos)]
		m.Set(p.row, p.col, vals[i%len(vals)])
	}
}

// BenchmarkRowSparseSetAdd 稀疏矩阵原位增量（一次扫描完成读改写）。
func BenchmarkRowSparseSetAdd(b *testing.B) {
	m := benchlibSparseBand(400, 8, 4243)
	pos := benchSparsePositions(m, 5)
	if len(pos) == 0 {
		b.Fatal("语料为空")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		p := pos[i%len(pos)]
		m.SetAdd(p.row, p.col, 0.25)
	}
}

// BenchmarkRowSparseGet 稀疏矩阵 Get（行内线性扫描）。
func BenchmarkRowSparseGet(b *testing.B) {
	m := benchlibSparseBand(400, 8, 4244)
	pos := benchSparsePositions(m, 5)
	if len(pos) == 0 {
		b.Fatal("语料为空")
	}
	sink := 0.0
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		p := pos[i%len(pos)]
		sink += m.Get(p.row, p.col)
	}
	if sink == 0 {
		b.Fatal("Get 结果恒为 0，语料无效")
	}
}

// BenchmarkVectorIncrement 稠密向量 Increment（越界检查 + 累加）。
func BenchmarkVectorIncrement(b *testing.B) {
	v := NewDenseVector[float64](benchVectorN)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		v.Increment(i%benchVectorN, 0.5)
	}
}

// BenchmarkUpdateMatrixRollback 更新矩阵 Rollback（丢弃缓存修改）。
func BenchmarkUpdateMatrixRollback(b *testing.B) {
	base := benchlibDenseDominant(benchUpdateN, 777)
	um := benchlibPatchedUpdateMatrix(base, 64, 778)
	rows, cols := base.Rows(), base.Cols()
	type patch struct {
		row, col int
		val      float64
	}
	patches := make([]patch, 32)
	for i := range patches {
		patches[i] = patch{row: (i * 7) % rows, col: (i * 11) % cols, val: float64(i) + 2}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		for _, p := range patches {
			um.Set(p.row, p.col, p.val)
		}
		um.Rollback()
	}
}

// BenchmarkMinimumDegreeOrder 最小度重排（符号阶段的主开销，m3 的 ordering.go 判定口径）。
func BenchmarkMinimumDegreeOrder(b *testing.B) {
	n := benchSparseN
	adj := benchlibAdjacency(benchSparseCorpus())
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		perm := MinimumDegreeOrder(n, adj)
		if len(perm) != n {
			b.Fatalf("MinimumDegreeOrder 返回长度 %d != %d", len(perm), n)
		}
	}
}
