// orderingCriterion_test.go —— 静态重排择优判据（UD8）的回归防护。
//
// 背景：旧判据 `maxDeg > 10*avgDeg` 只看度数比，会漏掉两类真实结构——
// 度数比 ≈4 的 hub（电源/时钟总线）与度数比 ≈1.0 的随机稀疏；两者都会让自然顺序
// 的 fill-in 爆炸。新判据改为「按 SymbolicFillCount 预测，在自然顺序与最小度之间择优」，
// 并以 `fillID > 3n` 作为成本护栏（带状/三对角填充本就小，不付最小度重排的开销）。
//
// 本文件用合成结构（生成器见 benchlib_test.go / benchmark_test.go）锁定判据行为：
//   - hub600/hub800/random600：必须选最小度重排，且 U nnz 显著低于自然顺序；
//   - band600/tridiag1000：必须保持自然顺序（既有数值指纹不变）。
//
// 附带的数值安全判据见 TestPermChoiceNumericalSafety：换序只引入浮点噪声（≤1e-6）。
package maths

import (
	"math"
	"math/rand"
	"testing"
)

// orderingCriterionCases 构造五种 ≥500 结构（与 m3/UD8 量测所用语料一致）。
func orderingCriterionCases() map[string]*rowSparseMatrix[float64] {
	rng := rand.New(rand.NewSource(7))
	tridiag := func() *rowSparseMatrix[float64] {
		m := newRowSparseMatrix[float64](1000, 1000)
		for i := 0; i < 1000; i++ {
			m.Set(i, i, 10)
			if i > 0 {
				m.Set(i, i-1, 1)
			}
			if i < 999 {
				m.Set(i, i+1, 1)
			}
		}
		return m
	}()
	random := func() *rowSparseMatrix[float64] {
		m := newRowSparseMatrix[float64](600, 600)
		for i := 0; i < 600; i++ {
			m.Set(i, i, 1+rng.Float64())
			for k := 0; k < 8; k++ {
				j := rng.Intn(600)
				if j != i {
					m.Set(i, j, rng.Float64()*2-1)
				}
			}
		}
		return m
	}()
	return map[string]*rowSparseMatrix[float64]{
		"hub600":      benchSparseCorpus(),
		"band600":     benchlibSparseBand(600, 6, 20261012),
		"hub800":      benchlibSparseHub(800, 4, 8, 99),
		"tridiag1000": tridiag,
		"random600":   random,
	}
}

// orderingProductionRun 走生产路径（Decompose → computeStaticPerm），返回 (是否自然顺序, U nnz)。
func orderingProductionRun(t *testing.T, m Matrix[float64]) (bool, int) {
	t.Helper()
	lu, err := NewLUSparse[float64](m.Rows())
	if err != nil {
		t.Fatal(err)
	}
	lus := lu.(*kluSparse[float64])
	if err := lus.Decompose(m); err != nil {
		t.Fatalf("Decompose: %v", err)
	}
	isIdentity := true
	for i, p := range lus.perm {
		if i != p {
			isIdentity = false
			break
		}
	}
	return isIdentity, lus.U.NonZeroCount()
}

// TestOrderingCriterionSelection 锁定判据在五种结构上的选择与 U nnz 上界。
func TestOrderingCriterionSelection(t *testing.T) {
	type want struct {
		identity bool
		maxNnz   int
	}
	wants := map[string]want{
		// 旧判据下这三个都会走自然顺序（U nnz 31778 / 51120 / 107486）
		"hub600":    {identity: false, maxNnz: 6000},
		"hub800":    {identity: false, maxNnz: 8000},
		"random600": {identity: false, maxNnz: 90000},
		// 这两个必须保持自然顺序（换序只会打乱带宽）
		"band600":     {identity: true, maxNnz: 5000},
		"tridiag1000": {identity: true, maxNnz: 3000},
	}
	cases := orderingCriterionCases()
	for name, w := range wants {
		m, ok := cases[name]
		if !ok {
			t.Fatalf("缺少语料 %s", name)
		}
		isIdentity, nnz := orderingProductionRun(t, m)
		if isIdentity != w.identity {
			t.Errorf("%s：选择自然顺序=%v，期望 %v（判据回归）", name, isIdentity, w.identity)
		}
		if nnz > w.maxNnz {
			t.Errorf("%s：U nnz=%d 超过上界 %d（填充控制回归）", name, nnz, w.maxNnz)
		}
	}
}

// TestPermChoiceNumericalSafety 在「判据会改变选择」的结构上验证换序只引入浮点噪声。
//
// 口径：相对误差 ≤ 1e-6（与 maths/lu_sparse_test.go:121-123、element/time/sparse_test.go:49 一致）。
func TestPermChoiceNumericalSafety(t *testing.T) {
	cases := orderingCriterionCases()
	for _, name := range []string{"hub600", "hub800", "random600"} {
		m, ok := cases[name]
		if !ok {
			t.Fatalf("缺少语料 %s", name)
		}
		n := m.Rows()
		adj, _, _ := benchAdjOf(m)

		b := NewDenseVector[float64](n)
		for i := 0; i < n; i++ {
			b.Set(i, math.Sin(float64(i)*0.7)+0.3*math.Cos(float64(i)*1.3))
		}
		xID, err := permSolveWith(t, m, identityPerm(n), b)
		if err != nil {
			t.Fatalf("%s：自然顺序求解失败: %v", name, err)
		}
		xMD, err := permSolveWith(t, m, MinimumDegreeOrder(n, adj), b)
		if err != nil {
			t.Fatalf("%s：最小度求解失败: %v", name, err)
		}
		maxRel := 0.0
		for i := 0; i < n; i++ {
			a, c := xID.Get(i), xMD.Get(i)
			if rel := math.Abs(a-c) / math.Max(1.0, math.Abs(a)); rel > maxRel {
				maxRel = rel
			}
		}
		if maxRel > 1e-6 {
			t.Errorf("%s：两种重排的解最大相对误差 %.3g > 1e-6（换序改变了数值解）", name, maxRel)
		}
	}
}

// benchAdjOf 取邻接表与度数统计（判据量测用）。
func benchAdjOf(m Matrix[float64]) (adj [][]int, maxDeg, avgDeg float64) {
	n := m.Rows()
	adj = make([][]int, n)
	total, md := 0, 0
	for i := 0; i < n; i++ {
		cols, _ := m.GetRow(i)
		adj[i] = cols
		total += len(cols)
		if len(cols) > md {
			md = len(cols)
		}
	}
	return adj, float64(md), float64(total) / float64(n)
}

// permSolveWith 用给定置换分解并求解 A·x=b。
func permSolveWith(t *testing.T, m Matrix[float64], perm []int, b Vector[float64]) (Vector[float64], error) {
	t.Helper()
	lu, err := NewLUSparse[float64](m.Rows())
	if err != nil {
		return nil, err
	}
	lus := lu.(*kluSparse[float64])
	lus.perm = perm
	lus.permSig = structureSignature(m)
	if err := lus.Decompose(m); err != nil {
		return nil, err
	}
	x := NewDenseVector[float64](m.Rows())
	if err := lus.SolveReuse(b, x); err != nil {
		return nil, err
	}
	return x, nil
}
