package doublebuffer

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"unsafe"
)

// FlatCodec 展平+zlib 编解码器，与现有 BlockWriter/BlockReader 行为等价
// 数据先展平再使用压缩器压缩，自带 dt/x 维度头
type FlatCodec[T Number] struct {
	compressor Compressor
}

// NewFlatCodec 创建展平编解码器实例
func NewFlatCodec[T Number](c Compressor) *FlatCodec[T] {
	return &FlatCodec[T]{compressor: c}
}

// Encode 将二维数据展平后压缩，结果带有 9 字节统一头
func (fc *FlatCodec[T]) Encode(data [][]T) ([]byte, error) {
	dt, x := len(data), 0
	if dt > 0 {
		x = len(data[0])
	}
	raw, err := flatten(data)
	if err != nil {
		return nil, err
	}
	compressed, err := fc.compressor.Compress(raw)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 9+len(compressed))
	binary.LittleEndian.PutUint32(out[0:4], uint32(dt))
	binary.LittleEndian.PutUint32(out[4:8], uint32(x))
	out[8] = byte(sizeOf[T]())
	copy(out[9:], compressed)
	return out, nil
}

// Decode 解码带有 9 字节统一头的数据
func (fc *FlatCodec[T]) Decode(encoded []byte) ([][]T, error) {
	if len(encoded) < 9 {
		return nil, fmt.Errorf("FlatCodec: 数据太短")
	}
	dt := int(binary.LittleEndian.Uint32(encoded[0:4]))
	x := int(binary.LittleEndian.Uint32(encoded[4:8]))
	if elemSize := int(encoded[8]); elemSize != sizeOf[T]() {
		return nil, fmt.Errorf("FlatCodec: 元素大小 %d 与类型不符", elemSize)
	}
	raw, err := fc.compressor.Decompress(encoded[9:])
	if err != nil {
		return nil, err
	}
	return unflatten[T](raw, dt, x)
}

// RLE 编码常量
const (
	rleTagZeroSkip = 0x00
	rleTagDelta    = 0x01
	rleTagEndCol   = 0xFF
	rleEpsilon     = 1e-12
)

// RLECompressor 列感知零值跳过 + Delta 编解码器
// 对每个 block [dt][x]T，按列独立编码：
//
//	每列格式: [base_value: elemSize bytes]
//	          重复: [tag: 1 byte]
//	                 0x00 ZERO_SKIP -> [skip_count: uvarint]
//	                 0x01 DELTA     -> [delta: elemSize bytes]
//	                 0xFF END_COLUMN
type RLECompressor[T Number] struct{}

// NewRLECompressor 创建 RLE 编解码器实例
func NewRLECompressor[T Number]() *RLECompressor[T] {
	return &RLECompressor[T]{}
}

// Encode 将二维数据按列进行 RLE+Delta 编码，结果带有 9 字节统一头
func (rc *RLECompressor[T]) Encode(data [][]T) ([]byte, error) {
	dt, x := len(data), 0
	if dt > 0 {
		x = len(data[0])
	}
	elemSize := sizeOf[T]()

	var buf bytes.Buffer
	header := make([]byte, 9)
	binary.LittleEndian.PutUint32(header[0:4], uint32(dt))
	binary.LittleEndian.PutUint32(header[4:8], uint32(x))
	header[8] = byte(elemSize)
	buf.Write(header)

	for col := 0; col < x; col++ {
		baseBytes := floatToBytes(data[0][col], elemSize)
		buf.Write(baseBytes)

		zeroRun := 0
		for row := 1; row < dt; row++ {
			delta := floatSub(data[row][col], data[row-1][col])
			if floatAbs(delta) < rleEpsilon {
				zeroRun++
				if zeroRun >= (1 << 28) {
					writeZeroRun(&buf, zeroRun)
					zeroRun = 0
				}
			} else {
				if zeroRun > 0 {
					writeZeroRun(&buf, zeroRun)
					zeroRun = 0
				}
				buf.WriteByte(rleTagDelta)
				deltaBytes := floatToBytes(delta, elemSize)
				buf.Write(deltaBytes)
			}
		}
		if zeroRun > 0 {
			writeZeroRun(&buf, zeroRun)
		}
		buf.WriteByte(rleTagEndCol)
	}
	return buf.Bytes(), nil
}

// Decode 解码 RLE+Delta 编码的数据
func (rc *RLECompressor[T]) Decode(encoded []byte) ([][]T, error) {
	if len(encoded) < 9 {
		return nil, fmt.Errorf("RLE: 数据太短")
	}
	dt := int(binary.LittleEndian.Uint32(encoded[0:4]))
	x := int(binary.LittleEndian.Uint32(encoded[4:8]))
	elemSize := int(encoded[8])
	payload := encoded[9:]

	// 校验头部字段，防止恶意数据触发越界/OOM。
	if err := validateDecodeDims(dt, x, elemSize); err != nil {
		return nil, err
	}
	if elemSize != sizeOf[T]() {
		return nil, fmt.Errorf("RLE: 元素大小 %d 与类型不符", elemSize)
	}
	if dt == 0 || x == 0 {
		return [][]T{}, nil
	}

	result := make([][]T, dt)
	for row := 0; row < dt; row++ {
		result[row] = make([]T, x)
	}

	offset := 0
	for col := 0; col < x; col++ {
		if offset+elemSize > len(payload) {
			return nil, fmt.Errorf("RLE: 数据截断，无法读取 base 值")
		}
		base := bytesToFloat[T](payload[offset:offset+elemSize], elemSize)
		offset += elemSize

		result[0][col] = base
		prev := base
		row := 1

		for row < dt {
			if offset >= len(payload) {
				return nil, fmt.Errorf("RLE: 数据截断，第 %d 列第 %d 行", col, row)
			}
			tag := payload[offset]
			offset++
			switch tag {
			case rleTagZeroSkip:
				if offset >= len(payload) {
					return nil, fmt.Errorf("RLE: 数据截断，无法读取 zero_run")
				}
				run, n := binary.Uvarint(payload[offset:])
				if n <= 0 {
					return nil, fmt.Errorf("RLE: 无效的 uvarint 编码")
				}
				// 防止恶意数据构造极大 run 值导致内存耗尽或 32 位平台溢出
				if run > math.MaxInt32 {
					return nil, fmt.Errorf("RLE: run 值 %d 超出安全范围", run)
				}
				offset += n
				for k := 0; k < int(run) && row < dt; k++ {
					result[row][col] = prev
					row++
				}
			case rleTagDelta:
				if offset+elemSize > len(payload) {
					return nil, fmt.Errorf("RLE: 数据截断，无法读取 delta 值")
				}
				delta := bytesToFloat[T](payload[offset:offset+elemSize], elemSize)
				offset += elemSize
				prev = floatAdd(prev, delta)
				if row < dt {
					result[row][col] = prev
					row++
				}
			case rleTagEndCol:
				// 行未填满就遇到列结束标签，说明数据损坏
				return nil, fmt.Errorf("RLE: 第 %d 列在第 %d 行提前结束", col, row)
			default:
				return nil, fmt.Errorf("RLE: 未知标签 0x%02X", tag)
			}
		}
		// 消费本列的 END_COL 标签
		if offset < len(payload) && payload[offset] == rleTagEndCol {
			offset++
		}
	}
	return result, nil
}

// writeZeroRun 写入零值跳过标记和计数
func writeZeroRun(buf *bytes.Buffer, run int) {
	buf.WriteByte(rleTagZeroSkip)
	var tmp [10]byte
	n := binary.PutUvarint(tmp[:], uint64(run))
	buf.Write(tmp[:n])
}

// floatToBytes 将 T 值转换为其 IEEE 754 字节表示
func floatToBytes[T Number](v T, elemSize int) []byte {
	b := make([]byte, elemSize)
	if elemSize == 8 {
		f := *(*float64)(unsafe.Pointer(&v))
		binary.LittleEndian.PutUint64(b, math.Float64bits(f))
	} else {
		f := *(*float32)(unsafe.Pointer(&v))
		binary.LittleEndian.PutUint32(b, math.Float32bits(f))
	}
	return b
}

// bytesToFloat 将 IEEE 754 字节表示转换回 T 值
func bytesToFloat[T Number](b []byte, elemSize int) T {
	var v T
	if elemSize == 8 {
		bits := binary.LittleEndian.Uint64(b)
		f := math.Float64frombits(bits)
		v = *(*T)(unsafe.Pointer(&f))
	} else {
		bits := binary.LittleEndian.Uint32(b)
		f := math.Float32frombits(bits)
		v = *(*T)(unsafe.Pointer(&f))
	}
	return v
}

// floatSub 计算 a - b，根据类型大小分派
func floatSub[T Number](a, b T) T {
	switch size := sizeOf[T](); size {
	case 4:
		af := *(*float32)(unsafe.Pointer(&a))
		bf := *(*float32)(unsafe.Pointer(&b))
		r := af - bf
		return *(*T)(unsafe.Pointer(&r))
	default:
		af := *(*float64)(unsafe.Pointer(&a))
		bf := *(*float64)(unsafe.Pointer(&b))
		r := af - bf
		return *(*T)(unsafe.Pointer(&r))
	}
}

// floatAdd 计算 a + b，根据类型大小分派
func floatAdd[T Number](a, b T) T {
	switch size := sizeOf[T](); size {
	case 4:
		af := *(*float32)(unsafe.Pointer(&a))
		bf := *(*float32)(unsafe.Pointer(&b))
		r := af + bf
		return *(*T)(unsafe.Pointer(&r))
	default:
		af := *(*float64)(unsafe.Pointer(&a))
		bf := *(*float64)(unsafe.Pointer(&b))
		r := af + bf
		return *(*T)(unsafe.Pointer(&r))
	}
}

// floatAbs 返回绝对值（float64 精度）
func floatAbs[T Number](v T) float64 {
	switch size := sizeOf[T](); size {
	case 4:
		f := *(*float32)(unsafe.Pointer(&v))
		return float64(math.Abs(float64(f)))
	default:
		f := *(*float64)(unsafe.Pointer(&v))
		return math.Abs(f)
	}
}
