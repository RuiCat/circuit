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

	maxBlocks   int     // 最大块数，0=无限制
	blockCount  int     // 已存储块数
	savedBlocks [][][]T // 环形保存的已完成块（仅 maxBlocks>0 时使用）
	blockRing   int     // 环形写入指针，指向下一个写入位置
}

// NewBuffer 创建双缓冲实例
// n: 每个缓冲区的最大行数
// x: 每行的节点数量（电压值数量）
func NewBuffer[T Number](n, x int) Buffer[T] {
	// 校验参数，防止 n/x 为负、为零或乘积过大导致构造期 panic/OOM。
	if n < 1 || x < 1 {
		panic(fmt.Sprintf("doublebuffer: NewBuffer 需要 n>=1 且 x>=1, 收到 n=%d x=%d", n, x))
	}
	if uint64(n)*uint64(x) > maxDecodeElements {
		panic(fmt.Sprintf("doublebuffer: NewBuffer 尺寸过大 n=%d x=%d", n, x))
	}
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
// 同时清空已保存的内存块数据，但保留 maxBlocks 配置
func (db *doubleBuffer[T]) Reset() {
	db.dt = 0
	db.writeFailed = false
	db.nonActiveHasData = false
	db.savedBlocks = nil
	db.blockCount = 0
	db.blockRing = 0
	if db.maxBlocks > 0 {
		db.savedBlocks = make([][][]T, 0, db.maxBlocks)
	}
}

// NewReader 基于当前压缩器配置创建泛型块读取器
func (db *doubleBuffer[T]) NewReader(r io.Reader) *BufferReader[T] {
	return NewBufferReader(r, db.compressor, db.codec)
}

// SetMaxBlocks 设置最大存储块数。n=0 表示无限制（默认）。
// 当 maxBlocks>0 时，超过限制的最旧数据块会被丢弃（环形缓冲）。
func (db *doubleBuffer[T]) SetMaxBlocks(n int) {
	db.maxBlocks = n
	if n > 0 {
		db.savedBlocks = make([][][]T, 0, n)
		db.blockCount = 0
		db.blockRing = 0
	} else {
		db.savedBlocks = nil
		db.blockCount = 0
		db.blockRing = 0
	}
}

// MaxBlocks 返回当前最大存储块数
func (db *doubleBuffer[T]) MaxBlocks() int {
	return db.maxBlocks
}

// BlockCount 返回当前已存储的块数
func (db *doubleBuffer[T]) BlockCount() int {
	return db.blockCount
}

// TotalRows 返回当前存储的总数据行数（包括活跃缓冲和已保存块）
func (db *doubleBuffer[T]) TotalRows() int {
	// 用 uint64 累加，防止极端块数×行数导致 int 溢出回绕为负。
	total := uint64(db.dt)

	if db.maxBlocks <= 0 || len(db.savedBlocks) == 0 {
		return int(total)
	}

	if len(db.savedBlocks) < db.maxBlocks {
		for _, block := range db.savedBlocks {
			total += uint64(len(block))
		}
	} else {
		for i := 0; i < db.maxBlocks; i++ {
			idx := (db.blockRing + i) % db.maxBlocks
			total += uint64(len(db.savedBlocks[idx]))
		}
	}

	// 饱和到 int 上限，避免回绕。
	if total > uint64(^uint(0)>>1) {
		return int(^uint(0) >> 1)
	}
	return int(total)
}

// LastRows 返回最近 n 行数据（跨块查询），用于交互式查询最新电压
// 数据顺序：第 0 行为最旧，第 n-1 行为最新
// n 不能超过 TotalRows()，否则取 TotalRows()
func (db *doubleBuffer[T]) LastRows(n int) [][]T {
	if n <= 0 {
		return nil
	}

	total := db.TotalRows()
	if total == 0 {
		return nil
	}
	if n > total {
		n = total
	}

	result := make([][]T, n)
	outIdx := n - 1 // 从结果尾部向前填充（先填最新数据）

	// 1. 活跃缓冲（最新数据）
	if db.dt > 0 {
		for i := db.dt - 1; i >= 0 && outIdx >= 0; i-- {
			src := db.buffers[db.active][i]
			result[outIdx] = make([]T, len(src))
			copy(result[outIdx], src)
			outIdx--
		}
	}

	// 2. 已保存块，从新到旧遍历
	if outIdx >= 0 && db.maxBlocks > 0 && len(db.savedBlocks) > 0 {
		if len(db.savedBlocks) < db.maxBlocks {
			// 线性存储，直接倒序遍历
			for b := len(db.savedBlocks) - 1; b >= 0 && outIdx >= 0; b-- {
				outIdx = db.copyBlockReverse(result, outIdx, db.savedBlocks[b])
			}
		} else {
			// 环形缓冲，从最新块开始倒序遍历
			newest := (db.blockRing - 1 + db.maxBlocks) % db.maxBlocks
			for i := 0; i < db.maxBlocks && outIdx >= 0; i++ {
				idx := (newest - i + db.maxBlocks) % db.maxBlocks
				outIdx = db.copyBlockReverse(result, outIdx, db.savedBlocks[idx])
			}
		}
	}

	return result
}

// copyBlockReverse 将块数据从后向前复制到 result 中
// 返回更新后的 outIdx
func (db *doubleBuffer[T]) copyBlockReverse(result [][]T, outIdx int, block [][]T) int {
	for i := len(block) - 1; i >= 0 && outIdx >= 0; i-- {
		result[outIdx] = make([]T, len(block[i]))
		copy(result[outIdx], block[i])
		outIdx--
	}
	return outIdx
}

// NewMemoryReader 从环形内存缓冲创建读取器（用于交互查询历史数据）
// 仅在 maxBlocks>0 时有意义，否则返回空读取器
func (db *doubleBuffer[T]) NewMemoryReader() *BufferReader[T] {
	var blocks [][][]T

	if db.maxBlocks > 0 && db.blockCount > 0 {
		blocks = make([][][]T, 0, db.blockCount)
		if db.blockCount < db.maxBlocks {
			// 线性存储，按时间顺序排列
			for _, b := range db.savedBlocks {
				blocks = append(blocks, b)
			}
		} else {
			// 环形缓冲，从最旧到最新排列
			for i := 0; i < db.maxBlocks; i++ {
				idx := (db.blockRing + i) % db.maxBlocks
				blocks = append(blocks, db.savedBlocks[idx])
			}
		}
	}

	return &BufferReader[T]{
		codec:  db.codec,
		blocks: blocks,
	}
}

// BufferReader 泛型块读取器
// 封装 BlockReader + unflatten，自动完成 读取→解压→反序列化 全流程
// 支持两种模式：文件读取（br 非 nil）和内存读取（blocks 非 nil）
type BufferReader[T Number] struct {
	br     *BlockReader
	codec  BlockCodec[T]
	blocks [][][]T // 内存块模式
	idx    int     // 内存读取当前位置
}

// NewBufferReader 创建泛型块读取器实例（文件模式）
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
	// 内存模式
	if len(br.blocks) > 0 {
		if br.idx >= len(br.blocks) {
			return nil, BlockHeader{}, io.EOF
		}
		block := br.blocks[br.idx]
		br.idx++
		h := BlockHeader{Dt: len(block), X: 0}
		if len(block) > 0 {
			h.X = len(block[0])
		}
		return block, h, nil
	}

	// 文件模式
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
	data := db.buffers[db.active][:db.dt]

	if db.writer != nil {
		var raw []byte
		if db.codec != nil {
			var err error
			raw, err = db.codec.Encode(data)
			if err != nil {
				db.writeFailed = true
				return err
			}
		} else {
			var err error
			raw, err = flatten(data)
			if err != nil {
				db.writeFailed = true
				return err
			}
		}
		if db.bw == nil {
			db.bw = NewBlockWriter(db.writer, db.compressor)
		}
		if err := db.bw.WriteBlock(db.dt, db.x, raw); err != nil {
			db.writeFailed = true
			return err
		}
	}

	db.saveBlock(data)
	db.dt = 0
	return nil
}

// flushNonActive 将非活跃缓冲数据写入文件（若存在）
func (db *doubleBuffer[T]) flushNonActive() error {
	if !db.nonActiveHasData {
		return nil
	}
	data := db.buffers[1-db.active][:db.n]

	// 如果有文件写入器，写入文件
	if db.writer != nil {
		var raw []byte
		if db.codec != nil {
			var err error
			raw, err = db.codec.Encode(data)
			if err != nil {
				db.writeFailed = true
				return err
			}
		} else {
			var err error
			raw, err = flatten(data)
			if err != nil {
				db.writeFailed = true
				return err
			}
		}
		if db.bw == nil {
			db.bw = NewBlockWriter(db.writer, db.compressor)
		}
		if err := db.bw.WriteBlock(db.n, db.x, raw); err != nil {
			db.writeFailed = true
			return err
		}
	}

	// 保存到内存环形缓冲（无论是否有文件写入器）
	db.saveBlock(data)
	db.nonActiveHasData = false
	return nil
}

// saveBlock 将已刷新的块数据深拷贝到环形内存缓冲中
// 仅在 maxBlocks>0 时执行
func (db *doubleBuffer[T]) saveBlock(data [][]T) {
	if db.maxBlocks <= 0 {
		return
	}

	// 如果 savedBlocks 未初始化（SetMaxBlocks 未调用，但 maxBlocks 被直接设置），就地初始化
	if db.savedBlocks == nil {
		db.savedBlocks = make([][][]T, 0, db.maxBlocks)
	}

	// 深拷贝块数据
	block := make([][]T, len(data))
	for i := range data {
		block[i] = make([]T, len(data[i]))
		copy(block[i], data[i])
	}

	if len(db.savedBlocks) < db.maxBlocks {
		db.savedBlocks = append(db.savedBlocks, block)
	} else {
		db.savedBlocks[db.blockRing] = block
		db.blockRing = (db.blockRing + 1) % db.maxBlocks
	}

	if db.blockCount < db.maxBlocks {
		db.blockCount++
	}
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
func flatten[T Number](data [][]T) ([]byte, error) {
	rows := len(data)
	if rows == 0 {
		return nil, nil
	}
	cols := len(data[0])
	// 行长度不一致时返回明确错误，防止静默 nil 导致文件写入损坏
	for i := 1; i < rows; i++ {
		if len(data[i]) != cols {
			return nil, fmt.Errorf("doublebuffer: 行 %d 长度 %d 与期望 %d 不匹配", i, len(data[i]), cols)
		}
	}
	elemSize := sizeOf[T]()
	total := uint64(rows) * uint64(cols) * uint64(elemSize)
	if total > uint64(^uint(0)>>1) {
		return nil, fmt.Errorf("doublebuffer: 展平数据过大 rows=%d cols=%d", rows, cols)
	}
	buf := make([]byte, int(total))
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
	return buf, nil
}

// unflatten 将 []byte 按行优先还原为 [][]T
func unflatten[T Number](raw []byte, dt, x int) ([][]T, error) {
	elemSize := sizeOf[T]()
	// 先校验维度，防止负值/超大维度绕过长度校验触发 OOM。
	if err := validateDecodeDims(dt, x, elemSize); err != nil {
		return nil, err
	}
	// 用 uint64 计算期望长度，避免 int 乘法溢出后与 len(raw) 误匹配。
	expectedLen := uint64(dt) * uint64(x) * uint64(elemSize)
	if uint64(len(raw)) != expectedLen {
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
