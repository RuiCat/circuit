package maths

import (
	"fmt"
)

// DataManager 通用数据管理器
// 使用 []float64 作为底层数据结构，提供基本的数据管理功能
type DataManager struct {
	data   []float64 // 底层数据存储
	length int       // 数据长度
}

// NewDataManager 创建新的数据管理器
func NewDataManager(length int) *DataManager {
	return &DataManager{
		data:   make([]float64, length),
		length: length,
	}
}

// NewDataManagerWithData 从现有数据创建数据管理器
func NewDataManagerWithData(data []float64) *DataManager {
	return &DataManager{
		data:   data,
		length: len(data),
	}
}

// Set 设置指定位置的值
func (dm *DataManager) Set(index int, value float64) {
	if index < 0 || index >= dm.length {
		panic("index out of range")
	}
	dm.data[index] = value
}

// Get 获取指定位置的值
func (dm *DataManager) Get(index int) float64 {
	if index < 0 || index >= dm.length {
		panic("index out of range")
	}
	return dm.data[index]
}

// Increment 增量设置值（累加）
func (dm *DataManager) Increment(index int, value float64) {
	if index < 0 || index >= dm.length {
		panic("index out of range")
	}
	dm.data[index] += value
}

// Length 返回数据长度
func (dm *DataManager) Length() int {
	return dm.length
}

// Data 返回底层数据切片
func (dm *DataManager) Data() []float64 {
	return dm.data
}

// Clear 清空所有数据（设置为0）
func (dm *DataManager) Clear() {
	clear(dm.data)
}

// ResizeInPlace 调整数据管理器的大小
func (dm *DataManager) ResizeInPlace(newLength int) {
	if newLength < 0 {
		panic("invalid length")
	}
	if newLength == dm.length {
		return
	}
	newData := make([]float64, newLength)
	// 复制现有数据
	copyLength := min(dm.length, newLength)
	copy(newData, dm.data[:copyLength])
	dm.data = newData
	dm.length = newLength
}

// AppendInPlace 追加数据
// 扩展数据存储
func (dm *DataManager) AppendInPlace(values ...float64) {
	if len(values) == 0 {
		return
	}
	newLength := dm.length + len(values)
	newData := make([]float64, newLength)
	// 复制现有数据
	copy(newData, dm.data)
	// 追加新数据
	copy(newData[dm.length:], values)
	dm.data = newData
	dm.length = newLength
}

// InsertInPlace 插入数据
// 在特定位置插入数据
func (dm *DataManager) InsertInPlace(index int, values ...float64) {
	if index < 0 || index > dm.length {
		panic("index out of range")
	}
	if len(values) == 0 {
		return
	}
	newLength := dm.length + len(values)
	newData := make([]float64, newLength)
	// 复制插入点之前的数据
	copy(newData, dm.data[:index])
	// 插入新数据
	copy(newData[index:], values)
	// 复制插入点之后的数据
	copy(newData[index+len(values):], dm.data[index:])
	dm.data = newData
	dm.length = newLength
}

// RemoveInPlace 移除数据
// 删除特定位置的数据
func (dm *DataManager) RemoveInPlace(index int, count int) {
	if index < 0 || index+count > dm.length {
		panic("index out of range")
	}
	if count <= 0 {
		panic("invalid count")
	}
	if count == 0 {
		return
	}
	newLength := dm.length - count
	newData := make([]float64, newLength)
	// 复制移除点之前的数据
	copy(newData, dm.data[:index])
	// 复制移除点之后的数据
	copy(newData[index:], dm.data[index+count:])
	dm.data = newData
	dm.length = newLength
}

// ReplaceInPlace 替换数据
// 替换特定位置的数据
func (dm *DataManager) ReplaceInPlace(index int, values ...float64) {
	if index < 0 || index+len(values) > dm.length {
		panic("index out of range")
	}
	copy(dm.data[index:], values)
}

// FillInPlace 填充数据
func (dm *DataManager) FillInPlace(value float64) {
	for i := range dm.data {
		dm.data[i] = value
	}
}

// ZeroInPlace 清零数据
// 将所有数据设置为0
func (dm *DataManager) ZeroInPlace() {
	clear(dm.data)
}

// Copy 复制数据到另一个数据管理器
func (dm *DataManager) Copy(target *DataManager) {
	if target.length != dm.length {
		panic("dimension mismatch")
	}
	copy(target.data, dm.data)
}

// NonZeroCount 返回非零元素数量
func (dm *DataManager) NonZeroCount() int {
	count := 0
	for i := 0; i < dm.length; i++ {
		if dm.data[i] != 0 {
			count++
		}
	}
	return count
}

// String 返回数据的字符串表示
func (dm *DataManager) String() string {
	result := "["
	for i := 0; i < dm.length; i++ {
		result += fmt.Sprintf("%8.4f ", dm.data[i])
	}
	result += "]"
	return result
}

// ==================== 矩阵数据管理器 ====================

// MatrixDataManager 矩阵数据管理器
// 基于 DataManager 实现矩阵数据管理
type MatrixDataManager struct {
	*DataManager
	rows, cols int // 矩阵维度
}

// NewMatrixDataManager 创建新的矩阵数据管理器
func NewMatrixDataManager(rows, cols int) *MatrixDataManager {
	return &MatrixDataManager{
		DataManager: NewDataManager(rows * cols),
		rows:        rows,
		cols:        cols,
	}
}

// NewMatrixDataManagerWithData 从现有数据创建矩阵数据管理器
func NewMatrixDataManagerWithData(data []float64, rows, cols int) *MatrixDataManager {
	if len(data) != rows*cols {
		panic("data dimension mismatch")
	}
	return &MatrixDataManager{
		DataManager: NewDataManagerWithData(data),
		rows:        rows,
		cols:        cols,
	}
}

// SetMatrix 设置矩阵元素
func (mdm *MatrixDataManager) SetMatrix(row, col int, value float64) {
	if row < 0 || row >= mdm.rows || col < 0 || col >= mdm.cols {
		panic("index out of range")
	}
	mdm.Set(row*mdm.cols+col, value)
}

// GetMatrix 获取矩阵元素
func (mdm *MatrixDataManager) GetMatrix(row, col int) float64 {
	if row < 0 || row >= mdm.rows || col < 0 || col >= mdm.cols {
		panic("index out of range")
	}
	return mdm.Get(row*mdm.cols + col)
}

// IncrementMatrix 增量设置矩阵元素
func (mdm *MatrixDataManager) IncrementMatrix(row, col int, value float64) {
	if row < 0 || row >= mdm.rows || col < 0 || col >= mdm.cols {
		panic("index out of range")
	}
	mdm.Increment(row*mdm.cols+col, value)
}

// Rows 返回矩阵行数
func (mdm *MatrixDataManager) Rows() int {
	return mdm.rows
}

// Cols 返回矩阵列数
func (mdm *MatrixDataManager) Cols() int {
	return mdm.cols
}

// IsSquare 检查是否为方阵
func (mdm *MatrixDataManager) IsSquare() bool {
	return mdm.rows == mdm.cols
}

// GetRow 获取指定行的数据
func (mdm *MatrixDataManager) GetRow(row int) ([]int, []float64) {
	if row < 0 || row >= mdm.rows {
		panic("row index out of range")
	}
	cols := make([]int, mdm.cols)
	values := make([]float64, mdm.cols)
	for j := 0; j < mdm.cols; j++ {
		cols[j] = j
		values[j] = mdm.GetMatrix(row, j)
	}
	return cols, values
}

// MatrixVectorMultiply 执行矩阵向量乘法
func (mdm *MatrixDataManager) MatrixVectorMultiply(x []float64) []float64 {
	if len(x) != mdm.cols {
		panic("vector dimension mismatch")
	}
	result := make([]float64, mdm.rows)
	for i := 0; i < mdm.rows; i++ {
		sum := 0.0
		for j := 0; j < mdm.cols; j++ {
			sum += mdm.GetMatrix(i, j) * x[j]
		}
		result[i] = sum
	}
	return result
}

// BuildFromDense 从稠密矩阵构建
func (mdm *MatrixDataManager) BuildFromDense(dense [][]float64) {
	if len(dense) != mdm.rows || (len(dense) > 0 && len(dense[0]) != mdm.cols) {
		panic("dimension mismatch")
	}
	for i := 0; i < mdm.rows; i++ {
		for j := 0; j < mdm.cols; j++ {
			mdm.SetMatrix(i, j, dense[i][j])
		}
	}
}

// ToDense 转换为稠密向量
func (mdm *MatrixDataManager) ToDense() []float64 {
	return mdm.Data()
}

// String 返回矩阵的字符串表示
func (mdm *MatrixDataManager) String() string {
	result := ""
	for i := 0; i < mdm.rows; i++ {
		for j := 0; j < mdm.cols; j++ {
			result += fmt.Sprintf("%8.4f ", mdm.GetMatrix(i, j))
		}
		result += "\n"
	}
	return result
}
