package doublebuffer

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"unsafe"
)

// 错误常量
var (
	// ErrWriterNotSet writer 未设置时触发
	ErrWriterNotSet = errors.New("doublebuffer: writer 未设置")
	// ErrInvalidLength Append 时 values 长度不等于 x 时触发
	ErrInvalidLength = errors.New("doublebuffer: Append 长度与 Cols 不匹配")
	// ErrWriteFailed 之前的写入操作已失败
	ErrWriteFailed = errors.New("doublebuffer: 之前的写入操作已失败，缓冲区不可用")
)

// doubleBuffer 双缓冲核心实现
type doubleBuffer[T Number] struct {
	buffers          [2][][]T
	active           int
	dt               int
	n                int
	x                int
	nonActiveHasData bool
	writer           io.Writer
	compressor       Compressor
	codec            BlockCodec[T]
	bw               *BlockWriter
	writeFailed      bool
}

// NewBuffer 创建双缓冲实例
// n: 每个缓冲区的最大行数
// x: 每行的节点数量（电压值数量）
func NewBuffer[T Number](n, x int) Buffer[T] {
	db := &doubleBuffer[T]{
		n:          n,
		x:          x,
		compressor: NewZlibCompressor(),
	}
	// 预分配两个缓冲区
	for i := 0; i < 2; i++ {
		db.buffers[i] = make([][]T, n)
		for j := 0; j < n; j++ {
			db.buffers[i][j] = make([]T, x)
		}
	}
	return db
}

// Append 追加一行数据到活跃缓冲区
// 若 dt >= n 则自动切换缓冲并将非活跃缓冲压缩写入文件
// 返回 switched 表示是否发生了缓冲切换
func (db *doubleBuffer[T]) Append(values []T) (switched bool, err error) {
	if db.writeFailed {
		return false, ErrWriteFailed
	}
	if len(values) != db.x {
		return false, ErrInvalidLength
	}

	// 复制数据到活跃缓冲
	copy(db.buffers[db.active][db.dt], values)
	db.dt++

	if db.dt >= db.n {
		// 先刷新非活跃缓冲（若有旧数据）
		if err := db.flushNonActive(); err != nil {
			return false, err
		}
		// 标记刚写满的缓冲为待刷新
		db.nonActiveHasData = true
		// 切换活跃缓冲
		db.active ^= 1
		db.dt = 0
		return true, nil
	}
	return false, nil
}

// ActiveLen 返回当前活跃缓冲已写入的行数 (dt)
func (db *doubleBuffer[T]) ActiveLen() int {
	return db.dt
}

// Cap 返回单个缓冲区的最大行数 (n)
func (db *doubleBuffer[T]) Cap() int {
	return db.n
}

// Cols 返回每行的节点数 (x)
func (db *doubleBuffer[T]) Cols() int {
	return db.x
}

// Flush 手动刷新所有缓冲中的残留数据到文件
// 先刷新非活跃缓冲（若有），再刷新当前活跃缓冲（若有）
// 用于仿真结束时处理未写满的残留数据
func (db *doubleBuffer[T]) Flush() error {
	if db.writeFailed {
		return ErrWriteFailed
	}
	if err := db.flushNonActive(); err != nil {
		return err
	}
	if db.dt == 0 {
		return nil
	}
	return db.flushActive()
}

// SetWriter 设置写入目标
func (db *doubleBuffer[T]) SetWriter(w io.Writer) {
	db.writer = w
	db.bw = nil // 重置 block writer
}

// SetCompressor 设置压缩器
func (db *doubleBuffer[T]) SetCompressor(c Compressor) {
	db.compressor = c
	db.bw = nil // 重置 block writer
}

// SetCodec 设置块编解码器（替代 Compressor，在结构化层面压缩）
func (db *doubleBuffer[T]) SetCodec(c BlockCodec[T]) {
	db.codec = c
	db.compressor = &NoopCompressor{}
	db.bw = nil
}

// Reset 重置缓冲区状态（清空活跃缓冲但不影响文件）
func (db *doubleBuffer[T]) Reset() {
	db.dt = 0
	db.writeFailed = false
	db.nonActiveHasData = false
}

// NewReader 基于当前压缩器配置创建泛型块读取器
func (db *doubleBuffer[T]) NewReader(r io.Reader) *BufferReader[T] {
	return NewBufferReader[T](r, db.compressor, db.codec)
}

// BufferReader 泛型块读取器
// 封装 BlockReader + unflatten，自动完成 读取→解压→反序列化 全流程
type BufferReader[T Number] struct {
	br    *BlockReader
	codec BlockCodec[T]
}

// NewBufferReader 创建泛型块读取器实例
// r: 数据来源，c: 压缩器（需与写入时一致）
func NewBufferReader[T Number](r io.Reader, c Compressor, codec BlockCodec[T]) *BufferReader[T] {
	return &BufferReader[T]{
		br:    NewBlockReader(r, c),
		codec: codec,
	}
}

// ReadBlock 读取下一个块并返回 [][]T 数据和块头信息
// 如果读取完毕返回 io.EOF
func (br *BufferReader[T]) ReadBlock() ([][]T, BlockHeader, error) {
	dt, x, raw, err := br.br.ReadBlock()
	if err != nil {
		return nil, BlockHeader{}, err
	}
	if br.codec != nil {
		data, err := br.codec.Decode(raw)
		if err != nil {
			return nil, BlockHeader{}, err
		}
		return data, BlockHeader{Dt: dt, X: x}, nil
	}
	data, err := unflatten[T](raw, dt, x)
	if err != nil {
		return nil, BlockHeader{}, err
	}
	return data, BlockHeader{Dt: dt, X: x}, nil
}

// flushActive 将当前活跃缓冲的前 dt 行数据写入文件
func (db *doubleBuffer[T]) flushActive() error {
	if db.writer == nil {
		return ErrWriterNotSet
	}
	data := db.buffers[db.active][:db.dt]
	var raw []byte
	if db.codec != nil {
		var err error
		raw, err = db.codec.Encode(data)
		if err != nil {
			db.writeFailed = true
			return err
		}
	} else {
		raw = flatten(data)
	}
	if db.bw == nil {
		db.bw = NewBlockWriter(db.writer, db.compressor)
	}
	if err := db.bw.WriteBlock(db.dt, db.x, raw); err != nil {
		db.writeFailed = true
		return err
	}
	db.dt = 0
	return nil
}

// flushNonActive 将非活跃缓冲数据写入文件（若存在）
func (db *doubleBuffer[T]) flushNonActive() error {
	if !db.nonActiveHasData {
		return nil
	}
	if db.writer == nil {
		return ErrWriterNotSet
	}
	data := db.buffers[1-db.active][:db.n]
	var raw []byte
	if db.codec != nil {
		var err error
		raw, err = db.codec.Encode(data)
		if err != nil {
			db.writeFailed = true
			return err
		}
	} else {
		raw = flatten(data)
	}
	if db.bw == nil {
		db.bw = NewBlockWriter(db.writer, db.compressor)
	}
	if err := db.bw.WriteBlock(db.n, db.x, raw); err != nil {
		db.writeFailed = true
		return err
	}
	db.nonActiveHasData = false
	return nil
}

// sizeOf 返回 Number 类型对应的字节大小
func sizeOf[T Number]() int {
	var v T
	switch any(v).(type) {
	case float32:
		return 4
	case float64:
		return 8
	default:
		// 命名类型（如 type myFloat float64），使用 reflect 判断底层类型
		if reflect.TypeOf(v).Kind() == reflect.Float32 {
			return 4
		}
		return 8
	}
}

// flatten 将 [][]T 按行优先展平为 []byte
func flatten[T Number](data [][]T) []byte {
	rows := len(data)
	if rows == 0 {
		return nil
	}
	cols := len(data[0])
	for i := 1; i < rows; i++ {
		if len(data[i]) != cols {
			// 所有行必须等长，否则数据损坏
			return nil
		}
	}
	elemSize := sizeOf[T]()
	total := rows * cols * elemSize
	buf := make([]byte, total)
	offset := 0
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			switch any(data[i][j]).(type) {
			case float64:
				binary.LittleEndian.PutUint64(buf[offset:], math.Float64bits(any(data[i][j]).(float64)))
			case float32:
				binary.LittleEndian.PutUint32(buf[offset:], math.Float32bits(any(data[i][j]).(float32)))
			default:
				// 命名类型（如 type myFloat float64），通过 unsafe 直接访问底层 float 值
				if elemSize == 4 {
					v := *(*float32)(unsafe.Pointer(&data[i][j]))
					binary.LittleEndian.PutUint32(buf[offset:], math.Float32bits(v))
				} else {
					v := *(*float64)(unsafe.Pointer(&data[i][j]))
					binary.LittleEndian.PutUint64(buf[offset:], math.Float64bits(v))
				}
			}
			offset += elemSize
		}
	}
	return buf
}

// unflatten 将 []byte 按行优先还原为 [][]T
func unflatten[T Number](raw []byte, dt, x int) ([][]T, error) {
	elemSize := sizeOf[T]()
	expectedLen := dt * x * elemSize
	if len(raw) != expectedLen {
		return nil, fmt.Errorf("doublebuffer: 数据长度不匹配: 期望 %d 字节，实际 %d 字节", expectedLen, len(raw))
	}
	result := make([][]T, dt)
	offset := 0
	for i := 0; i < dt; i++ {
		result[i] = make([]T, x)
		for j := 0; j < x; j++ {
			// 直接从 raw 读取底层 float 值，通过 unsafe 转换为 T
			// 兼容命名类型（如 type myFloat float64）
			if elemSize == 4 {
				bits := binary.LittleEndian.Uint32(raw[offset:])
				f := math.Float32frombits(bits)
				result[i][j] = *(*T)(unsafe.Pointer(&f))
			} else {
				bits := binary.LittleEndian.Uint64(raw[offset:])
				f := math.Float64frombits(bits)
				result[i][j] = *(*T)(unsafe.Pointer(&f))
			}
			offset += elemSize
		}
	}
	return result, nil
}
