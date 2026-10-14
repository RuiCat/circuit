// luKernel.go —— 稀疏 LU 的结构签名（K3 冻结接口：structureSignature）。
//
// 职责：只做「矩阵结构（每行列号集合）→ uint64 签名」这一件事，供 luSparse 判断
// 矩阵结构是否变化，从而决定能否复用静态重排序 perm。
//
// 与旧内联实现（原 maths/lu.go 里每行非零个数 FNV）的差异：旧签名只依赖每行的非零
// **个数**，于是「拓扑变了但度序列不变」（等度不同连）会命中缓存并复用陈旧 perm（静默
// 数值错）。本实现把每行的**列号集合**先排序再混入哈希：签名只依赖结构（行列号），
// 不依赖数值，也不依赖行内存储顺序。
package maths

import (
	"math"
	"slices"
)

const (
	// STRUCTURE_SIGNATURE_UNAVAILABLE 是「签名不可用」的哨兵：GetRow 读不出可用结构时返回它，
	// 调用方据此走「不复用 perm」分支。取 math.MaxUint64 以便与任何真实签名区分；
	// structureSignature 在返回前做构造性处理，保证真实签名不等于本值。
	STRUCTURE_SIGNATURE_UNAVAILABLE = math.MaxUint64
	// STRUCTURE_HASH_OFFSET 与 STRUCTURE_HASH_PRIME 是 FNV-1a 64 位的偏移基与质数。
	STRUCTURE_HASH_OFFSET = 14695981039346656037
	STRUCTURE_HASH_PRIME  = 1099511628211
	// STRUCTURE_ROW_DELIMITER 作为行分隔符参与哈希，使行边界本身进入签名。
	STRUCTURE_ROW_DELIMITER = 0xFFFFFFFFFFFFFFFF
)

// structureMix 把一个值混入 FNV-1a 哈希（^ 后乘质数）。
func structureMix(hash, value uint64) uint64 {
	return (hash ^ value) * STRUCTURE_HASH_PRIME
}

// structureAvoidSentinel 把哈希值挪离哨兵：与一个非零常量异或必然改变取值，
// 因此「真实签名 ≠ 哨兵」是构造性保证，而不是概率性假设。
func structureAvoidSentinel(sig uint64) uint64 {
	if sig == STRUCTURE_SIGNATURE_UNAVAILABLE {
		return sig ^ STRUCTURE_HASH_PRIME
	}
	return sig
}

// structureSignature 计算矩阵的结构签名：逐行取列号集合，每行先排序再混入哈希，
// 最后再混入行分隔符。
//
// 输入：只读 matrix 的 GetRow（列号 + 值）；值只用于校验行结构是否可用，不进入签名。
// 输出：uint64 签名；相同签名 ⇒ 每行列号集合逐行相同。
// 错误语义：matrix 为 nil，或 GetRow 返回的值向量缺失 / 与列号个数不对齐 → 返回
// STRUCTURE_SIGNATURE_UNAVAILABLE（哨兵），调用方必须走「不复用 perm」，不得当作合法签名。
// 不变量：n == 0 与单行矩阵返回确定签名且不 panic；签名与数值无关。
//
// 实现注记：行号与行分隔符都参与哈希，因此「n=0 的空矩阵」「单行矩阵」与任何 n ≥ 1 的
// 非空结构都不会共享同一个签名值。
func (lu *luSparse[T]) structureSignature(matrix Matrix[T]) uint64 {
	if matrix == nil {
		return STRUCTURE_SIGNATURE_UNAVAILABLE
	}
	sig := uint64(STRUCTURE_HASH_OFFSET)
	// scratch 复用同一块缓冲：每行拷入列号后原地排序，避免逐行分配；
	// 必须拷贝——GetRow 的实现（如 sparseMatrix）可能返回内部数组的切片视图，就地排序会改坏矩阵。
	scratch := make([]int, 0, 16)
	for i := 0; i < lu.n; i++ {
		rowCols, rowVals := matrix.GetRow(i)
		if rowVals == nil || rowVals.Length() != len(rowCols) {
			return STRUCTURE_SIGNATURE_UNAVAILABLE
		}
		scratch = append(scratch[:0], rowCols...)
		slices.Sort(scratch)
		sig = structureMix(sig, uint64(i))
		for _, col := range scratch {
			sig = structureMix(sig, uint64(col))
		}
		sig = structureMix(sig, STRUCTURE_ROW_DELIMITER)
	}
	return structureAvoidSentinel(sig)
}
