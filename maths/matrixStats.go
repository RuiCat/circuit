// matrixStats.go 提供装配层 GetRow 与矩阵读写操作的诊断计数，仅用于性能定位。
package maths

import (
	"sync/atomic"
	"time"
)

var getRowCount int64
var getRowNanos int64
var matrixSetCount int64
var matrixIncCount int64
var matrixGetCount int64

// ResetMatrixStats 清零装配层诊断计数。
func ResetMatrixStats() {
	atomic.StoreInt64(&getRowCount, 0)
	atomic.StoreInt64(&getRowNanos, 0)
	atomic.StoreInt64(&matrixSetCount, 0)
	atomic.StoreInt64(&matrixIncCount, 0)
	atomic.StoreInt64(&matrixGetCount, 0)
}

// MatrixStatsSnapshot 返回自上次 Reset 以来的 GetRow 次数与累计耗时。
func MatrixStatsSnapshot() (count int64, elapsed time.Duration) {
	return atomic.LoadInt64(&getRowCount), time.Duration(atomic.LoadInt64(&getRowNanos))
}

// MatrixOpsSnapshot 返回矩阵 Set/Increment/Get 的调用次数。
func MatrixOpsSnapshot() (setN, incN, getN int64) {
	return atomic.LoadInt64(&matrixSetCount), atomic.LoadInt64(&matrixIncCount), atomic.LoadInt64(&matrixGetCount)
}

// recordGetRow 累加一次 GetRow 的诊断数据。
func recordGetRow(d time.Duration) {
	atomic.AddInt64(&getRowCount, 1)
	atomic.AddInt64(&getRowNanos, d.Nanoseconds())
}

// recordMatrixSet 累加一次矩阵 Set 调用。
func recordMatrixSet() { atomic.AddInt64(&matrixSetCount, 1) }

// recordMatrixInc 累加一次矩阵 Increment 调用。
func recordMatrixInc() { atomic.AddInt64(&matrixIncCount, 1) }

// recordMatrixGet 累加一次矩阵 Get 调用。
func recordMatrixGet() { atomic.AddInt64(&matrixGetCount, 1) }
