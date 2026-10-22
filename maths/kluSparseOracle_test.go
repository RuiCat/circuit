package maths

import (
	"testing"
)

// symbolicSlots 是按「同一 perm + 同一主元行身份序列」做符号消元的独立预言机:
// 用稠密 bool 模式矩阵模拟消元(不碰任何数值、不复用被测实现),返回
// L 的槽位集(含对角)与 U 的槽位集(仅上三角含对角)。
//
// 判据来自计划文档:同一消元计划下,槽位集是确定的结构上界;被测实现的
// L/U 槽位必须与之集合相等(不多不少)。
func symbolicSlots(t *testing.T, A Matrix[float64], perm, pivotIDs []int) (lSlots, uSlots [][]bool) {
	t.Helper()
	n := A.Rows()
	inv := make([]int, n)
	for i, p := range perm {
		inv[p] = i
	}
	pat := make([][]bool, n)
	for i := range pat {
		pat[i] = make([]bool, n)
	}
	for i := 0; i < n; i++ {
		cols, _ := A.GetRow(perm[i])
		for _, c := range cols {
			pat[i][inv[c]] = true
		}
	}
	identity := make([]int, n)
	for i := range identity {
		identity[i] = i
	}
	lSlots = make([][]bool, n)
	uSlots = make([][]bool, n)
	for i := 0; i < n; i++ {
		lSlots[i] = make([]bool, n)
		uSlots[i] = make([]bool, n)
		lSlots[i][i] = true // L 单位对角
	}
	for k := 0; k < n; k++ {
		pos := -1
		for p := 0; p < n; p++ {
			if identity[p] == pivotIDs[k] {
				pos = p
				break
			}
		}
		if pos < 0 {
			t.Fatalf("第 %d 步的主元身份 %d 不在行序中", k, pivotIDs[k])
		}
		if pos != k {
			pat[pos], pat[k] = pat[k], pat[pos]
			identity[pos], identity[k] = identity[k], identity[pos]
		}
		if !pat[k][k] {
			t.Fatalf("第 %d 步主元行在列 %d 上无结构非零(计划本身不合法)", k, k)
		}
		for i := k + 1; i < n; i++ {
			if !pat[i][k] {
				continue
			}
			lSlots[i][k] = true
			for j := k + 1; j < n; j++ {
				if pat[k][j] {
					pat[i][j] = true
				}
			}
			pat[i][k] = false
		}
	}
	for i := 0; i < n; i++ {
		for j := i; j < n; j++ {
			uSlots[i][j] = pat[i][j]
		}
	}
	return lSlots, uSlots
}

// scaledMatrix 返回与 A 同结构、所有非零元乘 factor 的矩阵。
func scaledMatrix(A Matrix[float64], factor float64) Matrix[float64] {
	n := A.Rows()
	out := NewDenseMatrix[float64](n, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if v := A.Get(i, j); v != 0 {
				out.Set(i, j, factor*v)
			}
		}
	}
	return out
}

// assertSlotsEqual 逐格比较预言机槽位集与实际稀疏 L/U 结构。
func assertSlotsEqual(t *testing.T, name string, want [][]bool, got *kluMatrix[float64], upper bool) {
	t.Helper()
	for i := 0; i < got.Rows(); i++ {
		for j := 0; j < got.Cols(); j++ {
			expect := want[i][j]
			if upper && j < i {
				expect = false // U 是上三角(含对角)
			}
			if !upper && j > i {
				expect = false // L 是下三角(含对角)
			}
			if has := got.hasSlot(i, j); has != expect {
				t.Fatalf("%s 槽位 (%d,%d) = %v,预言机期望 %v", name, i, j, has, expect)
			}
		}
	}
}

// TestSparseLUSymbolicOracle 用独立符号消元校验稀疏 LU 的 L/U 槽位集:
// 在多种稀疏结构(含零对角置换矩阵)上,槽位必须与「同一 perm + 同一主元序列」
// 的符号消元结果集合相等;对重放路径(结构不变、值变)同样成立。
func TestSparseLUSymbolicOracle(t *testing.T) {
	cases := []struct {
		name string
		A    Matrix[float64]
	}{
		{"非对称五点", nonsymmetric2D(t, 4, 9101)},
		{"箭头", arrowMatrix(8)},
		{"置换", permutationMatrix(6)},
		{"三对角", tridiagonalMatrix(10)},
	}
	for _, c := range cases {
		n := c.A.Rows()
		lu, err := NewSparseLU[float64](n)
		if err != nil {
			t.Fatalf("%s NewSparseLU: %v", c.name, err)
		}
		if err := lu.Decompose(c.A); err != nil {
			t.Fatalf("%s Decompose: %v", c.name, err)
		}
		s := lu.(*kluSparse[float64])
		if s.plan == nil {
			t.Fatalf("%s 分解后应有消元计划", c.name)
		}
		lSlots, uSlots := symbolicSlots(t, c.A, s.perm, s.plan.pivotIDs)
		assertSlotsEqual(t, c.name+" L", lSlots, s.L.(*kluMatrix[float64]), false)
		assertSlotsEqual(t, c.name+" U", uSlots, s.U.(*kluMatrix[float64]), true)

		// 结构不变、值整体缩放(主元相对大小不变 ⇒ 计划仍有效)→ 重放路径的
		// 槽位集必须与同一预言机一致。
		A2 := scaledMatrix(c.A, 3)
		if err := lu.Decompose(A2); err != nil {
			t.Fatalf("%s 重放 Decompose: %v", c.name, err)
		}
		if s.plan == nil {
			t.Fatalf("%s 重放后计划不应丢失", c.name)
		}
		lSlots2, uSlots2 := symbolicSlots(t, A2, s.perm, s.plan.pivotIDs)
		assertSlotsEqual(t, c.name+" 重放 L", lSlots2, s.L.(*kluMatrix[float64]), false)
		assertSlotsEqual(t, c.name+" 重放 U", uSlots2, s.U.(*kluMatrix[float64]), true)

		// 结构不得随重放漂移:重放后的槽位数必须与全新动态分解一致
		// (回归:早期实现保留上轮零槽位,重放会让结构变稠)。
		fresh, err := NewSparseLU[float64](n)
		if err != nil {
			t.Fatalf("%s NewSparseLU: %v", c.name, err)
		}
		if err := fresh.Decompose(A2); err != nil {
			t.Fatalf("%s 全新分解: %v", c.name, err)
		}
		fs := fresh.(*kluSparse[float64])
		if got, want := s.L.(*kluMatrix[float64]).NonZeroCount(), fs.L.(*kluMatrix[float64]).NonZeroCount(); got != want {
			t.Fatalf("%s 重放 nnz(L)=%d,全新分解=%d(结构漂移)", c.name, got, want)
		}
		if got, want := s.U.(*kluMatrix[float64]).NonZeroCount(), fs.U.(*kluMatrix[float64]).NonZeroCount(); got != want {
			t.Fatalf("%s 重放 nnz(U)=%d,全新分解=%d(结构漂移)", c.name, got, want)
		}
	}
}
