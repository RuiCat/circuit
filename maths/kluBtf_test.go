package maths

import "testing"

// blockMatrix 构造块上三角结构的矩阵:块内相邻成环(保证每块强连通),
// 块间只在相邻块之间加一条前向边。
func blockMatrix(blocks [][]int) Matrix[float64] {
	n := 0
	for _, b := range blocks {
		n += len(b)
	}
	A := NewDenseMatrix[float64](n, n)
	for bi, mem := range blocks {
		for k, u := range mem {
			A.Set(u, u, 4)
			if len(mem) > 1 {
				v := mem[(k+1)%len(mem)]
				A.Set(u, v, -0.5)
				A.Set(v, u, -0.5)
			}
		}
		if bi+1 < len(blocks) {
			A.Set(mem[0], blocks[bi+1][0], -0.25)
		}
	}
	return A
}

// blockOfByStarts 由块起始下标得到「每个位置所属块号」。
func blockOfByStarts(n int, starts []int) []int {
	of := make([]int, n)
	for b := 0; b < len(starts); b++ {
		end := n
		if b+1 < len(starts) {
			end = starts[b+1]
		}
		for k := starts[b]; k < end; k++ {
			of[k] = b
		}
	}
	return of
}

// assertBlockTriangular 断言置换后非零元只落在对角块与右上块:
// A[perm[i]][perm[j]] ≠ 0 ⇒ block(i) ≤ block(j)。
func assertBlockTriangular(t *testing.T, A Matrix[float64], perm, starts []int) {
	t.Helper()
	of := blockOfByStarts(len(perm), starts)
	for i := 0; i < len(perm); i++ {
		for j := 0; j < len(perm); j++ {
			if A.Get(perm[i], perm[j]) != 0 && of[i] > of[j] {
				t.Fatalf("块三角化被破坏:(i=%d,j=%d) 非零但 block %d > %d", i, j, of[i], of[j])
			}
		}
	}
}

// TestBTFBlocks 断言 SCC 块顺序:三块链得到 3 块且块三角性成立;
// 单环(强连通)退回 nil;不相连的两块得到 2 块。
func TestBTFBlocks(t *testing.T) {
	btfOf := func(A Matrix[float64]) (*kluSparse[float64], []int, []int) {
		t.Helper()
		lu, err := NewSparseLUWithOptions[float64](A.Rows(), SparseLUOptions{UseBTF: true})
		if err != nil {
			t.Fatalf("NewSparseLUWithOptions: %v", err)
		}
		s := lu.(*kluSparse[float64])
		perm, starts := s.btfOrder(A)
		return s, perm, starts
	}

	blocks := [][]int{{0, 1}, {2, 3, 4}, {5}}
	A := blockMatrix(blocks)
	_, perm, starts := btfOf(A)
	if perm == nil || len(starts) != 3 {
		t.Fatalf("三块链应得到 3 块:starts=%v", starts)
	}
	assertBlockTriangular(t, A, perm, starts)

	cycle := NewDenseMatrix[float64](4, 4)
	for i := 0; i < 4; i++ {
		cycle.Set(i, i, 4)
		cycle.Set(i, (i+1)%4, -1)
	}
	if _, p, st := btfOf(cycle); p != nil || st != nil {
		t.Fatalf("单强连通分量应退回 nil:perm=%v starts=%v", p, st)
	}

	disconnected := blockMatrix([][]int{{0, 1}, {2, 3}})
	_, perm2, starts2 := btfOf(disconnected)
	if perm2 == nil || len(starts2) != 2 {
		t.Fatalf("两个不相连块应得到 2 块:starts=%v", starts2)
	}
	assertBlockTriangular(t, disconnected, perm2, starts2)
}

// TestSparseLUBTF 断言启用 BTF 后:解与稠密 LU 一致;L 只在块内、U 满足块上三角;
// 未启用时 blocks 保持 nil(默认行为不变)。
func TestSparseLUBTF(t *testing.T) {
	blocks := [][]int{{0, 1, 2}, {3, 4, 5, 6}, {7, 8}}
	A := blockMatrix(blocks)
	n := A.Rows()
	xtrue := make([]float64, n)
	for i := range xtrue {
		xtrue[i] = 1 + 0.1*float64(i)
	}
	b := A.MatrixVectorMultiply(NewDenseVectorWithData(xtrue)).ToDense()

	lu, err := NewSparseLUWithOptions[float64](n, SparseLUOptions{UseBTF: true})
	if err != nil {
		t.Fatalf("NewSparseLUWithOptions: %v", err)
	}
	if err := lu.Decompose(A); err != nil {
		t.Fatalf("BTF Decompose: %v", err)
	}
	s := lu.(*kluSparse[float64])
	if len(s.blocks) != 3 {
		t.Fatalf("应启用 3 块 BTF,实际 starts=%v", s.blocks)
	}
	x := NewDenseVector[float64](n)
	if err := lu.SolveReuse(NewDenseVectorWithData(b), x); err != nil {
		t.Fatalf("BTF SolveReuse: %v", err)
	}
	if e := relErrInf(x.ToDense(), xtrue); e > 1e-10 {
		t.Fatalf("BTF 解与真解相对误差 %.3e > 1e-10", e)
	}
	if e := relErrInf(x.ToDense(), triLU(t, A, b)); e > 1e-10 {
		t.Fatalf("BTF 解与稠密 LU 相对误差 %.3e > 1e-10", e)
	}

	of := blockOfByStarts(n, s.blocks)
	ls := s.L.(*kluMatrix[float64])
	us := s.U.(*kluMatrix[float64])
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if ls.hasSlot(i, j) && of[i] != of[j] {
				t.Fatalf("L 槽位 (%d,%d) 跨块(block %d vs %d)", i, j, of[i], of[j])
			}
			if us.hasSlot(i, j) && of[i] > of[j] {
				t.Fatalf("U 槽位 (%d,%d) 违反块上三角(block %d > %d)", i, j, of[i], of[j])
			}
		}
	}

	plain, err := NewSparseLU[float64](n)
	if err != nil {
		t.Fatalf("NewSparseLU: %v", err)
	}
	if err := plain.Decompose(A); err != nil {
		t.Fatalf("默认 Decompose: %v", err)
	}
	if got := plain.(*kluSparse[float64]).blocks; got != nil {
		t.Fatalf("未启用 BTF 时 blocks 应为 nil,实际 %v", got)
	}
}

// TestSparseLUBTFFill 断言交错块结构下 BTF 真的减少 fill(结构量断言,
// 不依赖墙钟):nnz(L)/nnz(U) 都必须严格小于不启用 BTF 的分解,且解仍正确。
func TestSparseLUBTFFill(t *testing.T) {
	const n = 64
	even := make([]int, 0, n/2)
	odd := make([]int, 0, n/2)
	for i := 0; i < n; i++ {
		if i%2 == 0 {
			even = append(even, i)
		} else {
			odd = append(odd, i)
		}
	}
	A := blockMatrix([][]int{even, odd})
	xtrue := make([]float64, n)
	for i := range xtrue {
		xtrue[i] = 1
	}
	b := A.MatrixVectorMultiply(NewDenseVectorWithData(xtrue)).ToDense()

	run := func(useBTF bool) (*kluSparse[float64], []float64) {
		var lu LU[float64]
		var err error
		if useBTF {
			lu, err = NewSparseLUWithOptions[float64](n, SparseLUOptions{UseBTF: true})
		} else {
			lu, err = NewSparseLU[float64](n)
		}
		if err != nil {
			t.Fatalf("构造: %v", err)
		}
		if err := lu.Decompose(A); err != nil {
			t.Fatalf("Decompose: %v", err)
		}
		x := NewDenseVector[float64](n)
		if err := lu.SolveReuse(NewDenseVectorWithData(b), x); err != nil {
			t.Fatalf("SolveReuse: %v", err)
		}
		return lu.(*kluSparse[float64]), x.ToDense()
	}
	plain, xPlain := run(false)
	withBTF, xBTF := run(true)
	if len(withBTF.blocks) != 2 {
		t.Fatalf("应得到 2 块,实际 %v", withBTF.blocks)
	}
	nnzPlainL := plain.L.(*kluMatrix[float64]).NonZeroCount()
	nnzPlainU := plain.U.(*kluMatrix[float64]).NonZeroCount()
	nnzBTFL := withBTF.L.(*kluMatrix[float64]).NonZeroCount()
	nnzBTFU := withBTF.U.(*kluMatrix[float64]).NonZeroCount()
	if !(nnzBTFL < nnzPlainL) || !(nnzBTFU < nnzPlainU) {
		t.Fatalf("BTF 未减少 fill:L %d→%d、U %d→%d", nnzPlainL, nnzBTFL, nnzPlainU, nnzBTFU)
	}
	if e := relErrInf(xBTF, xPlain); e > 1e-10 {
		t.Fatalf("BTF 与无 BTF 的解相对误差 %.3e > 1e-10", e)
	}
	if e := relErrInf(xBTF, xtrue); e > 1e-10 {
		t.Fatalf("BTF 解与真解相对误差 %.3e > 1e-10", e)
	}
	t.Logf("fill 对比:L %d→%d、U %d→%d", nnzPlainL, nnzBTFL, nnzPlainU, nnzBTFU)
}
