package maths

// luSignature_test.go —— 「等度不同连」反证用例（K3：结构签名与置换缓存接口）。
//
// 落地：m2 按父体指令把骨架（runs/circuit-perf-optimization/_results/m2/luSignature_test.go.skeleton，
// sha256 = 3603895139980876172812481dcab5eb1c541741df5b50906090686a0aa3f667）落到此处并改写本头部注释；
// 与骨架相比只有本注释块不同，`import (` 起逐字节相同。
// 该路径在 40-plan.md 的写域互斥表里登记为 m3 的写域，故 m3 接管本文件（补强/扩展）时视为自己的写域。
//
// 断言的对象：结构签名只依赖每行的列号集合。
// 反证的对象：只依赖「每行非零个数」的旧实现在「拓扑变了、度序列没变」时会命中缓存，
// 复用陈旧 perm（静默数值错）。
//
// 复算：
//   go test -count=1 -run 'Signature' ./maths/
// 旧实现反证（把 maths/lu.go 换回 HEAD 版本后本用例必须失败；见 _results/m2/summary.md）：
//   go test -count=1 -run 'TestStructureSignatureSameDegreeDifferentEdges' -overlay <before-overlay.json> ./maths/

import (
	"math"
	"testing"
)

// signatureTestN 取 ≥500：computeStaticPerm 才会真正计算签名（n<500 仍走 identityPerm）。
const signatureTestN = 600

// signatureFailingMatrix 模拟「GetRow 读不出可用结构」的矩阵（值向量缺失）。
type signatureFailingMatrix struct {
	*rowSparseMatrix[float64]
}

// GetRow 故意返回缺失的值向量，用于验证哨兵语义。
func (m *signatureFailingMatrix) GetRow(row int) ([]int, Vector[float64]) {
	return []int{1, 2}, nil
}

// signatureCorpus 构造 band 带状矩阵：|i-j| ≤ band 的位置非零。
func signatureCorpus(n, band int) *rowSparseMatrix[float64] {
	m := newRowSparseMatrix[float64](n, n)
	for i := 0; i < n; i++ {
		for d := -band; d <= band; d++ {
			j := i + d
			if j < 0 || j >= n {
				continue
			}
			m.Set(i, j, float64(d+band+1))
		}
	}
	return m
}

// signaturePermutedColumns 按列置换复制矩阵：每行非零**个数**逐行不变，但列号集合改变。
// mul 必须与 n 互质（n=600、mul=7），否则不是双射。
func signaturePermutedColumns(src *rowSparseMatrix[float64], mul, add int) *rowSparseMatrix[float64] {
	n := src.rows
	dst := newRowSparseMatrix[float64](n, n)
	for i := 0; i < n; i++ {
		cols, vals := src.GetRow(i)
		for idx, c := range cols {
			dst.Set(i, (c*mul+add)%n, vals.Get(idx))
		}
	}
	return dst
}

// degreeOnlySignature 复刻被替换掉的旧实现（每行非零个数 FNV），
// 用于证明「等度不同连」确实会让旧签名相撞。
func degreeOnlySignature(m *rowSparseMatrix[float64]) uint64 {
	sig := uint64(14695981039346656037)
	for i := 0; i < m.rows; i++ {
		cols, _ := m.GetRow(i)
		sig = (sig ^ uint64(len(cols))) * 1099511628211
	}
	return sig
}

// signatureOf 对任意矩阵取结构签名。
func signatureOf(t *testing.T, m Matrix[float64]) uint64 {
	t.Helper()
	lu := &luSparse[float64]{baseLU: baseLU[float64]{n: m.Rows()}}
	return lu.structureSignature(m)
}

// TestStructureSignatureSameDegreeDifferentEdges 是核心反证用例：
// 列置换后每行非零个数逐行相同（旧签名相同），但列号集合不同 → 新签名必须不同，
// 且 computeStaticPerm 不得复用陈旧 perm。
func TestStructureSignatureSameDegreeDifferentEdges(t *testing.T) {
	base := signatureCorpus(signatureTestN, 1)
	scrambled := signaturePermutedColumns(base, 7, 13)
	for i := 0; i < signatureTestN; i++ {
		ca, _ := base.GetRow(i)
		cb, _ := scrambled.GetRow(i)
		if len(ca) != len(cb) {
			t.Fatalf("反证前提不成立：第 %d 行非零个数不同（%d vs %d）", i, len(ca), len(cb))
		}
	}
	if degreeOnlySignature(base) != degreeOnlySignature(scrambled) {
		t.Fatal("反证前提不成立：旧（每行非零个数）签名已能区分这两个矩阵")
	}
	sigBase, sigScrambled := signatureOf(t, base), signatureOf(t, scrambled)
	if sigBase == sigScrambled {
		t.Fatalf("新签名未能区分「等度不同连」：两矩阵签名都是 %d", sigBase)
	}
	if sigBase == math.MaxUint64 || sigScrambled == math.MaxUint64 {
		t.Fatal("真实签名不得等于哨兵 math.MaxUint64")
	}

	lu := &luSparse[float64]{baseLU: baseLU[float64]{n: signatureTestN}}
	first := lu.computeStaticPerm(base)
	second := lu.computeStaticPerm(scrambled)
	if first == nil || second == nil {
		t.Fatal("computeStaticPerm 返回 nil")
	}
	if lu.permSig != sigScrambled {
		t.Fatalf("permSig 未更新为当前结构签名：got %d want %d", lu.permSig, sigScrambled)
	}
	if &first[0] == &second[0] {
		t.Fatal("结构已变（等度不同连）仍复用陈旧 perm")
	}
}

// TestStructureSignatureReuseOnlyWhenUnchanged 验证正向复用路径：
// 结构不变时必须复用同一个 perm 切片。
func TestStructureSignatureReuseOnlyWhenUnchanged(t *testing.T) {
	m := signatureCorpus(signatureTestN, 1)
	lu := &luSparse[float64]{baseLU: baseLU[float64]{n: signatureTestN}}
	first := lu.computeStaticPerm(m)
	second := lu.computeStaticPerm(m)
	if first == nil || second == nil {
		t.Fatal("computeStaticPerm 返回 nil")
	}
	if &first[0] != &second[0] {
		t.Fatal("结构未变却未复用 perm")
	}
}

// TestStructureSignatureIgnoresValuesAndRowOrder 验证签名只依赖结构：
// 改数值、改行内插入顺序都不得改变签名。
func TestStructureSignatureIgnoresValuesAndRowOrder(t *testing.T) {
	n := signatureTestN
	base := signatureCorpus(n, 1)
	revalued := signatureCorpus(n, 1)
	for i := 0; i < n; i++ {
		revalued.Set(i, i, 123.5)
	}
	if signatureOf(t, base) != signatureOf(t, revalued) {
		t.Fatal("签名依赖数值")
	}
	// 同一列集合、相反插入顺序
	reordered := newRowSparseMatrix[float64](n, n)
	for i := 0; i < n; i++ {
		if i+1 < n {
			reordered.Set(i, i+1, -1)
		}
		reordered.Set(i, i, 2)
		if i-1 >= 0 {
			reordered.Set(i, i-1, -1)
		}
	}
	if signatureOf(t, base) != signatureOf(t, reordered) {
		t.Fatal("签名依赖行内存储顺序")
	}
}

// TestStructureSignatureEdgeCases 验证空矩阵 / 单行矩阵确定且不 panic，
// 以及 GetRow 不可用时返回哨兵（不得返回 0 冒充合法签名）。
func TestStructureSignatureEdgeCases(t *testing.T) {
	empty := newRowSparseMatrix[float64](0, 0)
	if a, b := signatureOf(t, empty), signatureOf(t, empty); a != b {
		t.Fatalf("空矩阵签名不确定：%d vs %d", a, b)
	}
	single := newRowSparseMatrix[float64](1, 1)
	single.Set(0, 0, 3)
	if sig := signatureOf(t, single); sig == 0 {
		t.Fatal("单行矩阵签名不得为 0（0 会被当成合法签名）")
	}
	failing := &signatureFailingMatrix{rowSparseMatrix: newRowSparseMatrix[float64](4, 4)}
	if got := signatureOf(t, failing); got != math.MaxUint64 {
		t.Fatalf("GetRow 值向量缺失时应返回哨兵 math.MaxUint64，得到 %d", got)
	}
	lu := &luSparse[float64]{baseLU: baseLU[float64]{n: 4}}
	if got := lu.structureSignature(nil); got != math.MaxUint64 {
		t.Fatalf("nil 矩阵应返回哨兵 math.MaxUint64，得到 %d", got)
	}
}
