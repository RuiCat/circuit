package maths

import (
	"errors"
	"runtime"
	"sync"
)

var (
	errLUZeroDim     = errors.New("lu: dimension must be positive")
	errLUNotSquare   = errors.New("lu: matrix must be square")
	errLUDimMismatch = errors.New("lu: matrix dimension mismatch")
	errLUSingular    = errors.New("lu: matrix is singular")
	errLUDivByZero   = errors.New("lu: division by zero (U diagonal is zero)")
)

// ParallelMatrixVectorMul 对 Matrix[T] 进行 MatrixVectorMultiply 的并行包装。
// 其余方法全部委托给内嵌的 Matrix[T]。
type ParallelMatrixVectorMul[T Number] struct {
	Matrix[T]
	numWorkers int
}

func NewParallelMatrixVectorMul[T Number](base Matrix[T], workers int) Matrix[T] {
	if workers < 1 {
		workers = runtime.GOMAXPROCS(0)
	}
	return &ParallelMatrixVectorMul[T]{
		Matrix:     base,
		numWorkers: workers,
	}
}

func (pm *ParallelMatrixVectorMul[T]) MatrixVectorMultiply(x Vector[T]) Vector[T] {
	n := pm.Rows()
	result := NewDenseVector[T](n)

	if n < pm.numWorkers*4 {
		for i := 0; i < n; i++ {
			cols, vals := pm.GetRow(i)
			var sum T
			for jIdx, col := range cols {
				sum += vals.Get(jIdx) * x.Get(col)
			}
			result.Set(i, sum)
		}
		return result
	}

	var wg sync.WaitGroup
	chunkSize := (n + pm.numWorkers - 1) / pm.numWorkers
	for w := 0; w < pm.numWorkers; w++ {
		start := w * chunkSize
		end := start + chunkSize
		if end > n {
			end = n
		}
		if start >= n {
			break
		}
		wg.Add(1)
		go func(s, e int) {
			defer wg.Done()
			for i := s; i < e; i++ {
				cols, vals := pm.GetRow(i)
				var sum T
				for jIdx, col := range cols {
					sum += vals.Get(jIdx) * x.Get(col)
				}
				result.Set(i, sum)
			}
		}(start, end)
	}
	wg.Wait()
	return result
}

// ParallelLU 实现 LU[T] 接口，Decompose 的内层行消元使用 goroutine 池并行。
type ParallelLU[T Number] struct {
	baseLU[T]
	numWorkers int
}

func NewParallelLU[T Number](n int, workers int) (LU[T], error) {
	if n < 1 {
		return nil, errLUZeroDim
	}
	if workers < 1 {
		workers = runtime.GOMAXPROCS(0)
	}
	return &ParallelLU[T]{
		baseLU: baseLU[T]{
			n:        n,
			L:        NewDenseMatrix[T](n, n),
			U:        NewDenseMatrix[T](n, n),
			Y:        NewDenseVector[T](n),
			P:        make([]int, n),
			pinverse: make([]int, n),
		},
		numWorkers: workers,
	}, nil
}

func (lu *ParallelLU[T]) Decompose(matrix Matrix[T]) error {
	if !matrix.IsSquare() {
		return errLUNotSquare
	}
	if matrix.Rows() != lu.n {
		return errLUDimMismatch
	}

	lu.init(matrix)

	const seqThreshold = 128
	if lu.n < seqThreshold || lu.numWorkers <= 1 {
		for k := 0; k < lu.n; k++ {
			if err := lu.pivotAndEliminateSequential(k); err != nil {
				return err
			}
		}
		return nil
	}

	for k := 0; k < lu.n; k++ {
		if err := lu.pivotAndEliminateParallel(k); err != nil {
			return err
		}
	}
	return nil
}

func (lu *ParallelLU[T]) pivotAndEliminateSequential(k int) error {
	maxRow := k
	maxAbsVal := Abs(lu.U.Get(k, k))
	for i := k + 1; i < lu.n; i++ {
		if v := Abs(lu.U.Get(i, k)); v > maxAbsVal {
			maxAbsVal = v
			maxRow = i
		}
	}
	if maxAbsVal < Epsilon {
		return errLUSingular
	}
	if maxRow != k {
		lu.U.SwapRows(k, maxRow)
		for j := 0; j < k; j++ {
			val1 := lu.L.Get(k, j)
			val2 := lu.L.Get(maxRow, j)
			lu.L.Set(k, j, val2)
			lu.L.Set(maxRow, j, val1)
		}
		lu.updatePermutation(k, maxRow)
	}

	pivotVal := lu.U.Get(k, k)
	for i := k + 1; i < lu.n; i++ {
		factor := lu.U.Get(i, k) / pivotVal
		lu.L.Set(i, k, factor)
		var zero T
		lu.U.Set(i, k, zero)
		for j := k + 1; j < lu.n; j++ {
			newVal := lu.U.Get(i, j) - factor*lu.U.Get(k, j)
			lu.U.Set(i, j, newVal)
		}
	}
	return nil
}

func (lu *ParallelLU[T]) pivotAndEliminateParallel(k int) error {
	maxRow := k
	maxAbsVal := Abs(lu.U.Get(k, k))
	for i := k + 1; i < lu.n; i++ {
		if v := Abs(lu.U.Get(i, k)); v > maxAbsVal {
			maxAbsVal = v
			maxRow = i
		}
	}
	if maxAbsVal < Epsilon {
		return errLUSingular
	}
	if maxRow != k {
		lu.U.SwapRows(k, maxRow)
		for j := 0; j < k; j++ {
			val1 := lu.L.Get(k, j)
			val2 := lu.L.Get(maxRow, j)
			lu.L.Set(k, j, val2)
			lu.L.Set(maxRow, j, val1)
		}
		lu.updatePermutation(k, maxRow)
	}

	pivotVal := lu.U.Get(k, k)
	remaining := lu.n - (k + 1)
	if remaining <= 0 {
		return nil
	}

	var wg sync.WaitGroup
	chunkSize := (remaining + lu.numWorkers - 1) / lu.numWorkers
	for w := 0; w < lu.numWorkers; w++ {
		start := k + 1 + w*chunkSize
		end := start + chunkSize
		if end > lu.n {
			end = lu.n
		}
		if start >= lu.n {
			break
		}
		wg.Add(1)
		go func(s, e int) {
			defer wg.Done()
			for i := s; i < e; i++ {
				factor := lu.U.Get(i, k) / pivotVal
				lu.L.Set(i, k, factor)
				var zero T
				lu.U.Set(i, k, zero)
				for j := k + 1; j < lu.n; j++ {
					newVal := lu.U.Get(i, j) - factor*lu.U.Get(k, j)
					lu.U.Set(i, j, newVal)
				}
			}
		}(start, end)
	}
	wg.Wait()
	return nil
}

func (lu *ParallelLU[T]) SolveReuse(b, x Vector[T]) error {
	if b.Length() != lu.n || x.Length() != lu.n {
		return errLUDimMismatch
	}

	lu.Y.Zero()
	for i := 0; i < lu.n; i++ {
		sum := b.Get(lu.P[i])
		for j := 0; j < i; j++ {
			sum -= lu.L.Get(i, j) * lu.Y.Get(j)
		}
		lu.Y.Set(i, sum)
	}

	x.Zero()
	for i := lu.n - 1; i >= 0; i-- {
		sum := lu.Y.Get(i)
		for j := i + 1; j < lu.n; j++ {
			sum -= lu.U.Get(i, j) * x.Get(j)
		}
		diagVal := lu.U.Get(i, i)
		if Abs(diagVal) < Epsilon {
			return errLUDivByZero
		}
		x.Set(i, sum/diagVal)
	}
	return nil
}
