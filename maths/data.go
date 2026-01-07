package maths

import "fmt"

// dataManager 一维浮点数据管理器（底层存储核心）
// 提供向量/矩阵底层数据的增删改查、扩容等基础操作
type dataManager struct {
	Data []float64 // 底层数据存储切片
	Len  int       // 数据长度（与len(data)一致）
}

// NewDataManager 创建指定长度的空数据管理器
func NewDataManager(length int) DataManager {
	if length < 0 {
		panic("invalid length: cannot be negative")
	}
	return &dataManager{
		Data: make([]float64, length),
		Len:  length,
	}
}

// NewDataManagerWithData 从现有切片创建数据管理器
func NewDataManagerWithData(data []float64) DataManager {
	return &dataManager{
		Data: append([]float64(nil), data...), // 深拷贝避免外部修改
		Len:  len(data),
	}
}

// Resize 修改底层数据大小
func (dm *dataManager) Resize(length int) {
	dm.Len = length
	dm.Data = make([]float64, length)
}

// Set 设置指定索引的值（索引越界会panic）
func (dm *dataManager) Set(index int, value float64) {
	if index < 0 || index >= dm.Len {
		panic(fmt.Sprintf("index out of range: %d (length: %d)", index, dm.Len))
	}
	dm.Data[index] = value
}

// Get 获取指定索引的值（索引越界会panic）
func (dm *dataManager) Get(index int) float64 {
	if index < 0 || index >= dm.Len {
		panic(fmt.Sprintf("index out of range: %d (length: %d)", index, dm.Len))
	}
	return dm.Data[index]
}

// Increment 增量更新指定索引的值（value累加，索引越界会panic）
func (dm *dataManager) Increment(index int, value float64) {
	if index < 0 || index >= dm.Len {
		panic(fmt.Sprintf("index out of range: %d (length: %d)", index, dm.Len))
	}
	dm.Data[index] += value
}

// Length 返回数据长度
func (dm *dataManager) Length() int {
	return dm.Len
}

// DataCopy 返回底层数据切片的拷贝（避免外部修改原数据）
func (dm *dataManager) DataCopy() []float64 {
	return append([]float64(nil), dm.Data...)
}

// DataPtr 返回底层数据切片指针
func (dm *dataManager) DataPtr() []float64 {
	return dm.Data
}

// Zero 清空所有数据（设置为0）
func (dm *dataManager) Zero() {
	clear(dm.Data) // 高效置零（Go 1.21+支持）
}

// ResizeInPlace 调整数据长度（原地扩容/缩容，保留前N个元素）
func (dm *dataManager) ResizeInPlace(newLength int) {
	if newLength < 0 {
		panic("invalid length: cannot be negative")
	}
	if newLength == dm.Len {
		return
	}
	newData := make([]float64, newLength)
	copyLength := min(dm.Len, newLength)
	copy(newData, dm.Data[:copyLength])
	dm.Data = newData
	dm.Len = newLength
}

// AppendInPlace 追加数据（原地扩展切片）
func (dm *dataManager) AppendInPlace(values ...float64) {
	if len(values) == 0 {
		return
	}
	newLength := dm.Len + len(values)
	newData := make([]float64, newLength)
	copy(newData, dm.Data)         // 复制原有数据
	copy(newData[dm.Len:], values) // 追加新数据
	dm.Data = newData
	dm.Len = newLength
}

// InsertInPlace 在指定索引插入数据（原地扩展，索引越界会panic）
func (dm *dataManager) InsertInPlace(index int, values ...float64) {
	if index < 0 || index > dm.Len {
		panic(fmt.Sprintf("insert index out of range: %d (length: %d)", index, dm.Len))
	}
	if len(values) == 0 {
		return
	}
	newLength := dm.Len + len(values)
	newData := make([]float64, newLength)
	copy(newData, dm.Data[:index])                     // 复制插入点前数据
	copy(newData[index:], values)                      // 插入新数据
	copy(newData[index+len(values):], dm.Data[index:]) // 复制插入点后数据
	dm.Data = newData
	dm.Len = newLength
}

// RemoveInPlace 从指定索引删除count个元素（原地缩容，越界会panic）
func (dm *dataManager) RemoveInPlace(index int, count int) {
	if index < 0 || index+count > dm.Len {
		panic(fmt.Sprintf("remove range out of range: index=%d, count=%d (length: %d)", index, count, dm.Len))
	}
	if count <= 0 {
		panic("invalid count: must be positive")
	}
	newLength := dm.Len - count
	newData := make([]float64, newLength)
	copy(newData, dm.Data[:index])               // 复制删除点前数据
	copy(newData[index:], dm.Data[index+count:]) // 复制删除点后数据
	dm.Data = newData
	dm.Len = newLength
}

// ReplaceInPlace 替换指定索引开始的元素（越界会panic）
func (dm *dataManager) ReplaceInPlace(index int, values ...float64) {
	if index < 0 || index+len(values) > dm.Len {
		panic(fmt.Sprintf("replace range out of range: index=%d, count=%d (length: %d)", index, len(values), dm.Len))
	}
	copy(dm.Data[index:], values)
}

// FillInPlace 填充所有元素为指定值
func (dm *dataManager) FillInPlace(value float64) {
	for i := range dm.Data {
		dm.Data[i] = value
	}
}

// ZeroInPlace 清空数据（等价于Clear，兼容旧逻辑）
func (dm *dataManager) ZeroInPlace() {
	dm.Zero()
}

// Copy 复制自身数据到目标dataManager（维度不匹配会panic）
func (dm *dataManager) Copy(target DataManager) {
	if target.Length() != dm.Len {
		panic(fmt.Sprintf("dimension mismatch: source length=%d, target length=%d", dm.Len, target.Length()))
	}
	copy(target.DataPtr(), dm.Data)
}

// NonZeroCount 统计非零元素数量（浮点数精度：|x| > 1e-16 视为非零）
func (dm *dataManager) NonZeroCount() int {
	count := 0
	epsilon := 1e-16
	for i := 0; i < dm.Len; i++ {
		if dm.Data[i] < -epsilon || dm.Data[i] > epsilon {
			count++
		}
	}
	return count
}

// String 格式化输出数据（保留4位小数）
func (dm *dataManager) String() string {
	result := "["
	for i := 0; i < dm.Len; i++ {
		result += fmt.Sprintf("%8.4f ", dm.Data[i])
	}
	return result + "]"
}

// MatrixDataManager 矩阵数据管理器（基于dataManager实现行优先存储）
type MatrixDataManager struct {
	DataManager *dataManager // 嵌入dataManager复用功能
	rows, cols  int          // 矩阵维度（rows行cols列）
}

// NewMatrixDataManager 创建指定维度的矩阵数据管理器
func NewMatrixDataManager(rows, cols int) *MatrixDataManager {
	if rows < 0 || cols < 0 {
		panic("invalid matrix dimensions: cannot be negative")
	}
	return &MatrixDataManager{
		DataManager: NewDataManager(rows * cols).(*dataManager),
		rows:        rows,
		cols:        cols,
	}
}

// NewMatrixDataManagerWithData 从切片创建矩阵数据管理器（行优先存储）
func NewMatrixDataManagerWithData(data []float64, rows, cols int) *MatrixDataManager {
	if len(data) != rows*cols {
		panic(fmt.Sprintf("data dimension mismatch: len(data)=%d, rows*cols=%d", len(data), rows*cols))
	}
	return &MatrixDataManager{
		DataManager: NewDataManagerWithData(data).(*dataManager),
		rows:        rows,
		cols:        cols,
	}
}

// SetMatrix 设置矩阵指定行列的值（越界会panic）
func (mdm *MatrixDataManager) SetMatrix(row, col int, value float64) {
	if row < 0 || row >= mdm.rows || col < 0 || col >= mdm.cols {
		panic(fmt.Sprintf("matrix index out of range: row=%d, col=%d (rows=%d, cols=%d)", row, col, mdm.rows, mdm.cols))
	}
	mdm.DataManager.Set(row*mdm.cols+col, value)
}

// GetMatrix 获取矩阵指定行列的值（越界会panic）
func (mdm *MatrixDataManager) GetMatrix(row, col int) float64 {
	if row < 0 || row >= mdm.rows || col < 0 || col >= mdm.cols {
		panic(fmt.Sprintf("matrix index out of range: row=%d, col=%d (rows=%d, cols=%d)", row, col, mdm.rows, mdm.cols))
	}
	return mdm.DataManager.Get(row*mdm.cols + col)
}

// IncrementMatrix 增量更新矩阵元素（越界会panic）
func (mdm *MatrixDataManager) IncrementMatrix(row, col int, value float64) {
	if row < 0 || row >= mdm.rows || col < 0 || col >= mdm.cols {
		panic(fmt.Sprintf("matrix index out of range: row=%d, col=%d (rows=%d, cols=%d)", row, col, mdm.rows, mdm.cols))
	}
	mdm.DataManager.Increment(row*mdm.cols+col, value)
}

// Rows 返回矩阵行数
func (mdm *MatrixDataManager) Rows() int {
	return mdm.rows
}

// Cols 返回矩阵列数
func (mdm *MatrixDataManager) Cols() int {
	return mdm.cols
}

// IsSquare 判断是否为方阵
func (mdm *MatrixDataManager) IsSquare() bool {
	return mdm.rows == mdm.cols
}

// GetRow 获取指定行的列索引和值（返回：列索引切片+值向量）
func (mdm *MatrixDataManager) GetRow(row int) ([]int, []float64) {
	if row < 0 || row >= mdm.rows {
		panic(fmt.Sprintf("row index out of range: %d (rows: %d)", row, mdm.rows))
	}
	cols := make([]int, 0, mdm.cols)
	values := make([]float64, 0, mdm.cols)
	rowStart := row * mdm.cols
	for j := 0; j < mdm.cols; j++ {
		val := mdm.DataManager.Get(rowStart + j)
		if val != 0 { // 仅返回非零元素（稀疏优化）
			cols = append(cols, j)
			values = append(values, val)
		}
	}
	return cols, values
}

// MatrixVectorMultiply 矩阵向量乘法（A*x，返回新向量）
func (mdm *MatrixDataManager) MatrixVectorMultiply(x []float64) []float64 {
	if len(x) != mdm.cols {
		panic(fmt.Sprintf("vector dimension mismatch: len(x)=%d, cols=%d", len(x), mdm.cols))
	}
	result := make([]float64, mdm.rows)
	for i := 0; i < mdm.rows; i++ {
		sum := 0.0
		rowStart := i * mdm.cols
		for j := 0; j < mdm.cols; j++ {
			sum += mdm.DataManager.Get(rowStart+j) * x[j]
		}
		result[i] = sum
	}
	return result
}

// BuildFromDense 从稠密矩阵构建（行优先填充）
func (mdm *MatrixDataManager) BuildFromDense(dense [][]float64) {
	if len(dense) != mdm.rows || (len(dense) > 0 && len(dense[0]) != mdm.cols) {
		panic(fmt.Sprintf("dense matrix dimension mismatch: expected %dx%d, got %dx%d", mdm.rows, mdm.cols, len(dense), len(dense[0])))
	}
	for i := 0; i < mdm.rows; i++ {
		for j := 0; j < mdm.cols; j++ {
			mdm.SetMatrix(i, j, dense[i][j])
		}
	}
}

// ToDense 转换为稠密切片（行优先展开）
func (mdm *MatrixDataManager) ToDense() []float64 {
	return mdm.DataManager.DataPtr() // 非拷贝
}

// String 格式化输出矩阵（每行一行数据）
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
