// matrixUpdateBatch.go：稀疏矩阵的批量缓存提交（m4）。
// 逐元素 Set 会走 insertElement→InsertInPlace，每插入一个元素就搬移整行/整体数据，
// 1e5 节点每步累计分配可达百 GB；这里按行归并脏列，一次重建 CSR。
package maths

import "sort"

// updateBatch 把 cache（线性下标 → 值）批量合并进稀疏矩阵 CSR：
// 每行与底层非零列归并排序，脏元素覆盖同列；一次重建 rowPtr/colInd/DataManager。
func (m *sparseMatrix[T]) updateBatch(cache map[int]T) {
	if len(cache) == 0 {
		return
	}
	cols := m.cols
	byRow := make(map[int]map[int]T, len(cache))
	for idx, val := range cache {
		row := idx / cols
		if byRow[row] == nil {
			byRow[row] = make(map[int]T)
		}
		byRow[row][idx%cols] = val
	}
	newCols := make([]int, 0, len(m.colInd)+len(cache))
	newVals := make([]T, 0, len(m.colInd)+len(cache))
	newRowPtr := make([]int, m.rows+1)
	var zero T
	for i := 0; i < m.rows; i++ {
		newRowPtr[i] = len(newCols)
		dirty := byRow[i]
		dirtyCols := make([]int, 0, len(dirty))
		for c := range dirty {
			dirtyCols = append(dirtyCols, c)
		}
		sort.Ints(dirtyCols)
		start, end := m.rowPtr[i], m.rowPtr[i+1]
		bi, di := start, 0
		for bi < end || di < len(dirtyCols) {
			switch {
			case di >= len(dirtyCols) || (bi < end && m.colInd[bi] < dirtyCols[di]):
				if v := m.DataManager.Get(bi); v != zero {
					newCols = append(newCols, m.colInd[bi])
					newVals = append(newVals, v)
				}
				bi++
			case bi >= end || dirtyCols[di] < m.colInd[bi]:
				if v := dirty[dirtyCols[di]]; v != zero {
					newCols = append(newCols, dirtyCols[di])
					newVals = append(newVals, v)
				}
				di++
			default:
				if v := dirty[dirtyCols[di]]; v != zero {
					newCols = append(newCols, dirtyCols[di])
					newVals = append(newVals, v)
				}
				bi++
				di++
			}
		}
	}
	newRowPtr[m.rows] = len(newCols)
	m.colInd = newCols
	m.DataManager.Zero()
	for _, v := range newVals {
		m.DataManager.AppendInPlace(v)
	}
	m.rowPtr = newRowPtr
}
