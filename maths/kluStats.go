// kluStats.go 提供稀疏 LU 分解的诊断计数（分解次数与累计耗时），仅用于性能定位。
package maths

import (
	"sync/atomic"
	"time"
)

var kluDecomposeCount int64
var kluDecomposeNanos int64

// ResetKluStats 清零诊断计数；不影响求解行为。
func ResetKluStats() {
	atomic.StoreInt64(&kluDecomposeCount, 0)
	atomic.StoreInt64(&kluDecomposeNanos, 0)
}

// KluStatsSnapshot 返回自上次 Reset 以来的分解次数与累计分解耗时。
func KluStatsSnapshot() (count int64, elapsed time.Duration) {
	count = atomic.LoadInt64(&kluDecomposeCount)
	elapsed = time.Duration(atomic.LoadInt64(&kluDecomposeNanos))
	return count, elapsed
}

// recordKluDecompose 累加一次分解的诊断数据。
func recordKluDecompose(d time.Duration) {
	atomic.AddInt64(&kluDecomposeCount, 1)
	atomic.AddInt64(&kluDecomposeNanos, d.Nanoseconds())
}
