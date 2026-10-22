package maths

import (
	"math"
	"testing"
)

// arrowMatrix 构造 n 阶箭头矩阵:对角 4、首行/首列非对角 −1(其余为零)。
func arrowMatrix(n int) Matrix[float64] {
	A := NewDenseMatrix[float64](n, n)
	for i := 0; i < n; i++ {
		A.Set(i, i, 4)
	}
	for i := 1; i < n; i++ {
		A.Set(0, i, -1)
		A.Set(i, 0, -1)
	}
	return A
}

// bandMatrix 构造 n 阶带宽 w 的对称带状矩阵(对角 4、带宽内 −1)。
func bandMatrix(n, w int) Matrix[float64] {
	A := NewDenseMatrix[float64](n, n)
	for i := 0; i < n; i++ {
		A.Set(i, i, 4)
		for d := 1; d <= w && i+d < n; d++ {
			A.Set(i, i+d, -1)
			A.Set(i+d, i, -1)
		}
	}
	return A
}

// permutationMatrix 构造零对角置换矩阵(A[i][(i+1)%n] = 1):必须靠行交换才能分解。
func permutationMatrix(n int) Matrix[float64] {
	A := NewDenseMatrix[float64](n, n)
	for i := 0; i < n; i++ {
		A.Set(i, (i+1)%n, 1)
	}
	return A
}

// solveBoth 用稀疏 LU 与稠密 LU 各解一次,返回两者解与稀疏求解器(便于检查内部状态)。
func solveBoth(t *testing.T, A Matrix[float64], b []float64) ([]float64, []float64, LU[float64]) {
	t.Helper()
	n := A.Rows()
	slu, err := NewSparseLU[float64](n)
	if err != nil {
		t.Fatalf("NewSparseLU: %v", err)
	}
	if err := slu.Decompose(A); err != nil {
		t.Fatalf("稀疏 Decompose: %v", err)
	}
	sx := NewDenseVector[float64](n)
	if err := slu.SolveReuse(NewDenseVectorWithData(b), sx); err != nil {
		t.Fatalf("稀疏 SolveReuse: %v", err)
	}
	return sx.ToDense(), triLU(t, A, b), slu
}

// TestSparseLUAgainstDense 在多类稀疏结构上与稠密 LU 交叉验证。
func TestSparseLUAgainstDense(t *testing.T) {
	cases := []struct {
		name string
		A    Matrix[float64]
	}{
		{"三对角", tridiagonalMatrix(16)},
		{"带状", bandMatrix(20, 3)},
		{"箭头", arrowMatrix(12)},
		{"置换", permutationMatrix(8)},
		{"非对称五点1", nonsymmetric2D(t, 6, 9101)},
		{"非对称五点2", nonsymmetric2D(t, 8, 42)},
	}
	for _, c := range cases {
		n := c.A.Rows()
		x := make([]float64, n)
		for i := range x {
			x[i] = math.Sin(float64(i)*0.31) + 0.5
		}
		b := c.A.MatrixVectorMultiply(NewDenseVectorWithData(x)).ToDense()
		got, want, _ := solveBoth(t, c.A, b)
		if e := relErrInf(got, want); e > 1e-10 {
			t.Fatalf("%s 解与稠密 LU 相对误差 %.3e > 1e-10", c.name, e)
		}
		if e := relErrInf(got, x); e > 1e-10 {
			t.Fatalf("%s 解与真解相对误差 %.3e > 1e-10", c.name, e)
		}
	}
}

// TestSparseLUNoFill 断言三对角结构不产生 fill:nnz(U) = 2n−1、nnz(L) = 2n−1
// (L 含 n 个显式对角 1 与 n−1 个乘子)。
func TestSparseLUNoFill(t *testing.T) {
	const n = 16
	A := tridiagonalMatrix(n)
	b := make([]float64, n)
	for i := range b {
		b[i] = 1
	}
	_, _, slu := solveBoth(t, A, b)
	ls := slu.(*kluSparse[float64]).L.(*kluMatrix[float64])
	us := slu.(*kluSparse[float64]).U.(*kluMatrix[float64])
	if got := ls.NonZeroCount(); got != 2*n-1 {
		t.Fatalf("nnz(L) = %d,期望 %d(无 fill)", got, 2*n-1)
	}
	if got := us.NonZeroCount(); got != 2*n-1 {
		t.Fatalf("nnz(U) = %d,期望 %d(无 fill)", got, 2*n-1)
	}
}

// TestSparseLUSingular 断言奇异/近奇异矩阵返回 error(而不是 NaN 解)。
func TestSparseLUSingular(t *testing.T) {
	if _, err := NewSparseLU[float64](4); err != nil {
		t.Fatalf("NewSparseLU: %v", err)
	}
	zero := NewDenseMatrix[float64](4, 4)
	lu, err := NewSparseLU[float64](4)
	if err != nil {
		t.Fatalf("NewSparseLU: %v", err)
	}
	if err := lu.Decompose(zero); err == nil {
		t.Fatal("零矩阵应返回奇异 error")
	}
	ones := NewDenseMatrix[float64](4, 4)
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			ones.Set(i, j, 1)
		}
	}
	lu2, err := NewSparseLU[float64](4)
	if err != nil {
		t.Fatalf("NewSparseLU: %v", err)
	}
	if err := lu2.Decompose(ones); err == nil {
		t.Fatal("秩亏矩阵应返回奇异 error")
	}
}

// TestSparseLUErrors 覆盖维度/非方阵/向量维度错误。
func TestSparseLUErrors(t *testing.T) {
	if _, err := NewSparseLU[float64](0); err == nil {
		t.Fatal("维度 0 应返回 error")
	}
	lu, err := NewSparseLU[float64](3)
	if err != nil {
		t.Fatalf("NewSparseLU: %v", err)
	}
	if err := lu.Decompose(NewDenseMatrix[float64](3, 2)); err == nil {
		t.Fatal("非方阵应返回 error")
	}
	if err := lu.Decompose(NewDenseMatrix[float64](4, 4)); err == nil {
		t.Fatal("维度不符应返回 error")
	}
	A := tridiagonalMatrix(3)
	if err := lu.Decompose(A); err != nil {
		t.Fatalf("Decompose: %v", err)
	}
	if err := lu.SolveReuse(NewDenseVector[float64](2), NewDenseVector[float64](3)); err == nil {
		t.Fatal("b 维度不符应返回 error")
	}
	if err := lu.SolveReuse(NewDenseVector[float64](3), NewDenseVector[float64](2)); err == nil {
		t.Fatal("x 维度不符应返回 error")
	}
}

// TestSparseLUDeterminism 断言同输入两次独立分解的 perm/槽位/解逐位一致。
func TestSparseLUDeterminism(t *testing.T) {
	A := nonsymmetric2D(t, 6, 9101)
	n := A.Rows()
	b := nonsymmetricRHS(A, n)
	x1, _, lu1 := solveBoth(t, A, b)
	x2, _, lu2 := solveBoth(t, A, b)
	for i := range x1 {
		if x1[i] != x2[i] {
			t.Fatalf("两次分解的解在第 %d 位不同: %v vs %v", i, x1[i], x2[i])
		}
	}
	s1 := lu1.(*kluSparse[float64])
	s2 := lu2.(*kluSparse[float64])
	if len(s1.perm) != len(s2.perm) {
		t.Fatal("perm 长度不同")
	}
	for i := range s1.perm {
		if s1.perm[i] != s2.perm[i] {
			t.Fatalf("perm[%d] = %d vs %d", i, s1.perm[i], s2.perm[i])
		}
	}
	if s1.L.(*kluMatrix[float64]).NonZeroCount() != s2.L.(*kluMatrix[float64]).NonZeroCount() ||
		s1.U.(*kluMatrix[float64]).NonZeroCount() != s2.U.(*kluMatrix[float64]).NonZeroCount() {
		t.Fatal("两次分解的槽位数不同")
	}
}

// TestSparseLUPlanReplay 断言结构不变、值变化时走计划重放(不重建计划),
// 结果与独立分解一致。
func TestSparseLUPlanReplay(t *testing.T) {
	A1 := nonsymmetric2D(t, 6, 9101)
	A2 := nonsymmetric2D(t, 6, 9202) // 同一结构、不同数值
	n := A1.Rows()
	b := nonsymmetricRHS(A2, n)
	lu, err := NewSparseLU[float64](n)
	if err != nil {
		t.Fatalf("NewSparseLU: %v", err)
	}
	if err := lu.Decompose(A1); err != nil {
		t.Fatalf("A1 Decompose: %v", err)
	}
	s := lu.(*kluSparse[float64])
	if s.plan == nil {
		t.Fatal("首次分解后应有消元计划")
	}
	before := s.plan
	if err := lu.Decompose(A2); err != nil {
		t.Fatalf("A2 Decompose(重放): %v", err)
	}
	if s.plan != before {
		t.Fatal("结构不变时应走重放(计划对象不应被替换)")
	}
	x := NewDenseVector[float64](n)
	if err := lu.SolveReuse(NewDenseVectorWithData(b), x); err != nil {
		t.Fatalf("SolveReuse: %v", err)
	}
	want := triLU(t, A2, b)
	if e := relErrInf(x.ToDense(), want); e > 1e-10 {
		t.Fatalf("重放解与稠密 LU 相对误差 %.3e > 1e-10", e)
	}
	if e := relErrInf(x.ToDense(), mustSolveFreshSparse(t, A2, b)); e > 1e-12 {
		t.Fatalf("重放解与独立稀疏分解相对误差 %.3e > 1e-12", e)
	}

	// 多次换值仍应复用同一计划。
	for _, seed := range []int64{9301, 9302, 9303} {
		if err := lu.Decompose(nonsymmetric2D(t, 6, seed)); err != nil {
			t.Fatalf("seed=%d Decompose: %v", seed, err)
		}
		if s.plan != before {
			t.Fatalf("seed=%d 结构不变时应复用同一计划", seed)
		}
	}
}

// mustSolveFreshSparse 用全新的稀疏 LU 独立分解并求解。
func mustSolveFreshSparse(t *testing.T, A Matrix[float64], b []float64) []float64 {
	t.Helper()
	n := A.Rows()
	lu, err := NewSparseLU[float64](n)
	if err != nil {
		t.Fatalf("NewSparseLU: %v", err)
	}
	if err := lu.Decompose(A); err != nil {
		t.Fatalf("Decompose: %v", err)
	}
	x := NewDenseVector[float64](n)
	if err := lu.SolveReuse(NewDenseVectorWithData(b), x); err != nil {
		t.Fatalf("SolveReuse: %v", err)
	}
	return x.ToDense()
}

// TestSparseLUPlanStale 构造"同结构但主元行改变"的两组数值:重放应判失效并
// 回退动态路径,结果仍与稠密 LU 一致。
func TestSparseLUPlanStale(t *testing.T) {
	const n = 8
	// 结构相同:对角与超对角(环形)非零;A1 对角取极小值(主元必须换到末行),
	// A2 对角取 10(主元就地)。对角都显式非零,结构签名才一致。
	makeMat := func(diag float64) Matrix[float64] {
		A := NewDenseMatrix[float64](n, n)
		for i := 0; i < n; i++ {
			A.Set(i, i, diag)
			A.Set(i, (i+1)%n, 1)
		}
		return A
	}
	A1, A2 := makeMat(1e-12), makeMat(10)
	if structureSignature(A1) != structureSignature(A2) {
		t.Fatal("测试前提:两组数值的结构签名必须相同")
	}
	lu, err := NewSparseLU[float64](n)
	if err != nil {
		t.Fatalf("NewSparseLU: %v", err)
	}
	if err := lu.Decompose(A1); err != nil {
		t.Fatalf("A1 Decompose: %v", err)
	}
	s := lu.(*kluSparse[float64])
	before := s.plan
	if err := lu.Decompose(A2); err != nil {
		t.Fatalf("A2 Decompose: %v", err)
	}
	if s.plan == before {
		t.Fatal("主元行改变时应判计划失效并重建(计划对象应被替换)")
	}
	b := make([]float64, n)
	for i := range b {
		b[i] = float64(i + 1)
	}
	x := NewDenseVector[float64](n)
	if err := lu.SolveReuse(NewDenseVectorWithData(b), x); err != nil {
		t.Fatalf("SolveReuse: %v", err)
	}
	if e := relErrInf(x.ToDense(), triLU(t, A2, b)); e > 1e-10 {
		t.Fatalf("回退后解与稠密 LU 相对误差 %.3e > 1e-10", e)
	}
}

// TestSparseLUPlanSafetyValve 断言连续失效 3 次后停用计划(回头路被切断时
// 不再反复付"重放失败 + 动态重建"的双开销),且结果仍正确。
func TestSparseLUPlanSafetyValve(t *testing.T) {
	const n = 8
	makeMat := func(diag float64) Matrix[float64] {
		A := NewDenseMatrix[float64](n, n)
		for i := 0; i < n; i++ {
			A.Set(i, i, diag)
			A.Set(i, (i+1)%n, 1)
		}
		return A
	}
	lu, err := NewSparseLU[float64](n)
	if err != nil {
		t.Fatalf("NewSparseLU: %v", err)
	}
	seq := []Matrix[float64]{makeMat(1e-12), makeMat(10), makeMat(1e-12), makeMat(10)}
	for i, A := range seq {
		if err := lu.Decompose(A); err != nil {
			t.Fatalf("第 %d 次 Decompose: %v", i, err)
		}
	}
	s := lu.(*kluSparse[float64])
	if !s.skelOff {
		t.Fatal("连续 3 次计划失效后应停用计划(安全阀)")
	}
	if s.plan != nil {
		t.Fatal("停用期间不应继续固化计划")
	}
	// 停用后仍能正确分解并求解。
	if err := lu.Decompose(makeMat(10)); err != nil {
		t.Fatalf("停用后 Decompose: %v", err)
	}
	b := []float64{1, 2, 3, 4, 5, 6, 7, 8}
	x := NewDenseVector[float64](n)
	if err := lu.SolveReuse(NewDenseVectorWithData(b), x); err != nil {
		t.Fatalf("停用后 SolveReuse: %v", err)
	}
	if e := relErrInf(x.ToDense(), triLU(t, makeMat(10), b)); e > 1e-10 {
		t.Fatalf("停用后解与稠密 LU 相对误差 %.3e > 1e-10", e)
	}
}

// TestSparseLUGridResidual 在 32×32 网格(n=1024)上验证稀疏 LU 的数值正确性:
// 以 xtrue 全 1 生成 b,解回后相对误差与残差都应在 1e-9 内;同时记录 nnz(L)/nnz(U),
// 断言 nnz(U) < n²/4(松上界,只防退化成稠密)。
func TestSparseLUGridResidual(t *testing.T) {
	const m = 32
	n := m * m
	A := sparseGridMatrix(m)
	xtrue := make([]float64, n)
	for i := range xtrue {
		xtrue[i] = 1
	}
	b := A.MatrixVectorMultiply(NewDenseVectorWithData(xtrue)).ToDense()
	lu, err := NewSparseLU[float64](n)
	if err != nil {
		t.Fatalf("NewSparseLU: %v", err)
	}
	if err := lu.Decompose(A); err != nil {
		t.Fatalf("Decompose: %v", err)
	}
	x := NewDenseVector[float64](n)
	if err := lu.SolveReuse(NewDenseVectorWithData(b), x); err != nil {
		t.Fatalf("SolveReuse: %v", err)
	}
	got := x.ToDense()
	if e := relErrInf(got, xtrue); e > 1e-9 {
		t.Fatalf("解与真解相对误差 %.3e > 1e-9", e)
	}
	residual := 0.0
	ax := A.MatrixVectorMultiply(x).ToDense()
	for i := range b {
		if d := math.Abs(b[i] - ax[i]); d > residual {
			residual = d
		}
	}
	if residual > 1e-9 {
		t.Fatalf("残差 %.3e > 1e-9", residual)
	}
	s := lu.(*kluSparse[float64])
	nnzL := s.L.(*kluMatrix[float64]).NonZeroCount()
	nnzU := s.U.(*kluMatrix[float64]).NonZeroCount()
	if nnzU >= n*n/4 {
		t.Fatalf("nnz(U)=%d 不应接近稠密(n²/4=%d)", nnzU, n*n/4)
	}
	t.Logf("n=%d nnz(L)=%d nnz(U)=%d fill(L)=%.2f fill(U)=%.2f", n, nnzL, nnzU,
		float64(nnzL)/float64(n), float64(nnzU)/float64(n))
}

// TestSparseLUThresholdPivot 断言阈值主元选项:τ 越界/NaN → error;零值按 1;
// τ=1 与 NewSparseLU 逐位一致;τ=0.1 在对角占优结构上仍选就地主元,解与稠密 LU 一致。
func TestSparseLUThresholdPivot(t *testing.T) {
	for _, tau := range []float64{-0.1, 1.5, math.NaN()} {
		if _, err := NewSparseLUWithOptions[float64](4, SparseLUOptions{PivotThreshold: tau}); err == nil {
			t.Fatalf("PivotThreshold=%v 应返回 error", tau)
		}
	}
	A := nonsymmetric2D(t, 6, 9101)
	n := A.Rows()
	xtrue := make([]float64, n)
	for i := range xtrue {
		xtrue[i] = math.Cos(float64(i) * 0.27)
	}
	b := A.MatrixVectorMultiply(NewDenseVectorWithData(xtrue)).ToDense()

	base, err := NewSparseLU[float64](n)
	if err != nil {
		t.Fatalf("NewSparseLU: %v", err)
	}
	if err := base.Decompose(A); err != nil {
		t.Fatalf("默认 Decompose: %v", err)
	}
	xBase := NewDenseVector[float64](n)
	if err := base.SolveReuse(NewDenseVectorWithData(b), xBase); err != nil {
		t.Fatalf("默认 SolveReuse: %v", err)
	}

	withDefault, err := NewSparseLUWithOptions[float64](n, SparseLUOptions{})
	if err != nil {
		t.Fatalf("NewSparseLUWithOptions: %v", err)
	}
	if err := withDefault.Decompose(A); err != nil {
		t.Fatalf("τ=0 选项 Decompose: %v", err)
	}
	xDef := NewDenseVector[float64](n)
	if err := withDefault.SolveReuse(NewDenseVectorWithData(b), xDef); err != nil {
		t.Fatalf("τ=0 选项 SolveReuse: %v", err)
	}
	for i := 0; i < n; i++ {
		if xBase.ToDense()[i] != xDef.ToDense()[i] {
			t.Fatalf("默认选项与 NewSparseLU 解在第 %d 位不同", i)
		}
	}

	thr, err := NewSparseLUWithOptions[float64](n, SparseLUOptions{PivotThreshold: 0.1})
	if err != nil {
		t.Fatalf("τ=0.1 构造: %v", err)
	}
	if err := thr.Decompose(A); err != nil {
		t.Fatalf("τ=0.1 Decompose: %v", err)
	}
	xThr := NewDenseVector[float64](n)
	if err := thr.SolveReuse(NewDenseVectorWithData(b), xThr); err != nil {
		t.Fatalf("τ=0.1 SolveReuse: %v", err)
	}
	if e := relErrInf(xThr.ToDense(), triLU(t, A, b)); e > 1e-10 {
		t.Fatalf("τ=0.1 解与稠密 LU 相对误差 %.3e > 1e-10", e)
	}
	if e := relErrInf(xThr.ToDense(), xtrue); e > 1e-10 {
		t.Fatalf("τ=0.1 解与真解相对误差 %.3e > 1e-10", e)
	}
}

// TestSparseLUDropTolerance 断言可选的 fill 丢弃:参数非法 → error;
// τ=0 与默认逐位一致;开启后在「微小 fill」结构上 nnz 严格减少、解仍在
// 可控误差内,且不再固化消元计划(结构随数值变化)。
func TestSparseLUDropTolerance(t *testing.T) {
	for _, drop := range []float64{-1e-9, math.NaN(), math.Inf(1)} {
		if _, err := NewSparseLUWithOptions[float64](4, SparseLUOptions{DropTolerance: drop}); err == nil {
			t.Fatalf("DropTolerance=%v 应返回 error", drop)
		}
	}

	const n = 10
	makeMat := func(eps float64) Matrix[float64] {
		A := NewDenseMatrix[float64](n, n)
		for i := 0; i < n; i++ {
			A.Set(i, i, 4)
			if i+1 < n {
				A.Set(i, i+1, -1)
				A.Set(i+1, i, -1)
			}
		}
		A.Set(0, n-1, eps)
		A.Set(n-1, 0, eps)
		return A
	}
	A := makeMat(1e-10)
	xtrue := make([]float64, n)
	for i := range xtrue {
		xtrue[i] = 1 + 0.5*float64(i)
	}
	b := A.MatrixVectorMultiply(NewDenseVectorWithData(xtrue)).ToDense()

	solveWith := func(opt SparseLUOptions) (*kluSparse[float64], []float64) {
		lu, err := NewSparseLUWithOptions[float64](n, opt)
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

	plain, xPlain := solveWith(SparseLUOptions{})
	_, xZero := solveWith(SparseLUOptions{DropTolerance: 0})
	for i := 0; i < n; i++ {
		if xPlain[i] != xZero[i] {
			t.Fatalf("DropTolerance=0 应与默认逐位一致(第 %d 位不同)", i)
		}
	}
	if _, err := NewSparseLUWithOptions[float64](n, SparseLUOptions{DropTolerance: 1e-6}); err != nil {
		t.Fatalf("构造 drop: %v", err)
	}
	dropped, xDrop := solveWith(SparseLUOptions{DropTolerance: 1e-6})
	if got, want := dropped.L.(*kluMatrix[float64]).NonZeroCount(), plain.L.(*kluMatrix[float64]).NonZeroCount(); got >= want {
		t.Fatalf("drop 应减少 nnz(L):%d 未小于 %d", got, want)
	}
	if got, want := dropped.U.(*kluMatrix[float64]).NonZeroCount(), plain.U.(*kluMatrix[float64]).NonZeroCount(); got >= want {
		t.Fatalf("drop 应减少 nnz(U):%d 未小于 %d", got, want)
	}
	if e := relErrInf(xDrop, triLU(t, A, b)); e > 1e-6 {
		t.Fatalf("drop 解与稠密 LU 相对误差 %.3e > 1e-6", e)
	}
	for i, v := range xDrop {
		if !IsValidFloat64(v) {
			t.Fatalf("drop 解第 %d 位非有限: %v", i, v)
		}
	}
	if dropped.plan != nil {
		t.Fatal("开启 drop tolerance 后不应固化消元计划")
	}
	if !dropped.skelOff {
		t.Fatal("开启 drop tolerance 后 plan 重放应被禁用")
	}
}
