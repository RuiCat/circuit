package maths

import (
	"reflect"
	"testing"
)

func TestSparseMatrixResize(t *testing.T) {
	// 1. 创建一个稀疏矩阵并添加一些元素
	sm := NewSparseMatrix[float64](3, 3)
	sm.Set(0, 0, 1.0)
	sm.Set(1, 1, 2.0)
	sm.Set(2, 2, 3.0)

	if sm.NonZeroCount() != 3 {
		t.Fatalf("希望在调整大小前有3个非零元素, 得到 %d", sm.NonZeroCount())
	}

	// 2. 调整矩阵大小
	sm.Resize(5, 5)

	// 3. 验证新状态
	if sm.Rows() != 5 {
		t.Errorf("希望调整大小后的行为5, 得到 %d", sm.Rows())
	}
	if sm.Cols() != 5 {
		t.Errorf("希望调整大小后的列为5, 得到 %d", sm.Cols())
	}

	// 4. 验证矩阵是否为空（所有元素已清除）
	// 这间接验证了其底层的DataManager大小是否也为0
	if sm.NonZeroCount() != 0 {
		t.Errorf("希望调整大小后有0个非零元素, 得到 %d", sm.NonZeroCount())
	}

	// 5. 验证Get对所有元素返回0
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			if sm.Get(i, j) != 0.0 {
				t.Errorf("希望在(%d, %d)处的元素在调整大小后为0, 得到 %f", i, j, sm.Get(i, j))
			}
		}
	}
}

func TestDenseMatrixGetRow(t *testing.T) {
	// 1. 创建一个具有零和非零值的稠密矩阵并设置一行
	dm := NewDenseMatrix[float64](3, 4)
	dm.Set(1, 0, 10.0)
	dm.Set(1, 1, 0.0) // 这应该被忽略
	dm.Set(1, 2, 30.0)
	dm.Set(1, 3, 0.0) // 这应该被忽略

	// 2. 调用 GetRow
	cols, valuesVec := dm.GetRow(1)
	values := valuesVec.ToDense()

	// 3. 定义预期结果（仅非零元素）
	expectedCols := []int{0, 2}
	expectedValues := []float64{10.0, 30.0}

	// 4. 断言结果
	if !reflect.DeepEqual(cols, expectedCols) {
		t.Errorf("GetRow 返回了不正确的列. 希望得到 %v, 得到 %v", expectedCols, cols)
	}

	if !reflect.DeepEqual(values, expectedValues) {
		t.Errorf("GetRow 返回了不正确的值. 希望得到 %v, 得到 %v", expectedValues, values)
	}
}
