package doublebuffer

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
)

// ZlibCompressor zlib 压缩器实现
type ZlibCompressor struct{}

// NewZlibCompressor 创建 zlib 压缩器实例
func NewZlibCompressor() *ZlibCompressor {
	return &ZlibCompressor{}
}

// Compress 使用 zlib 压缩字节数据
func (zc *ZlibCompressor) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decompress 使用 zlib 解压字节数据
func (zc *ZlibCompressor) Decompress(data []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	// 限制解压输出大小，防止解压炸弹（zlib 最大膨胀比约 1032:1）。
	lr := &io.LimitedReader{R: r, N: maxDecompressedSize + 1}
	buf, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(buf)) > maxDecompressedSize {
		return nil, fmt.Errorf("zlib: 解压数据超出上限 %d 字节", maxDecompressedSize)
	}
	return buf, nil
}

// DeltaZlibCompressor delta 编码 + zlib 压缩器实现
type DeltaZlibCompressor struct {
	zlib     *ZlibCompressor
	elemSize int
}

// NewDeltaZlibCompressor 创建 delta+zlib 压缩器实例
// elemSize 必须是 4 (float32) 或 8 (float64)，否则 panic
func NewDeltaZlibCompressor(elemSize int) *DeltaZlibCompressor {
	if elemSize != 4 && elemSize != 8 {
		panic("doublebuffer: NewDeltaZlibCompressor elemSize must be 4 or 8")
	}
	return &DeltaZlibCompressor{
		zlib:     NewZlibCompressor(),
		elemSize: elemSize,
	}
}

// Compress 先 delta 编码再 zlib 压缩
func (dc *DeltaZlibCompressor) Compress(data []byte) ([]byte, error) {
	deltaEncoded := dc.deltaEncode(data)
	return dc.zlib.Compress(deltaEncoded)
}

// Decompress 先 zlib 解压再 delta 解码
func (dc *DeltaZlibCompressor) Decompress(data []byte) ([]byte, error) {
	raw, err := dc.zlib.Decompress(data)
	if err != nil {
		return nil, err
	}
	return dc.deltaDecode(raw), nil
}

// deltaEncode 计算相邻值之间的无符号整数差值。
// 已知限制：对于跨越零值的 IEEE 754 浮点数（正负交替），delta 值会非常大，
// 导致 zlib 压缩率降低。未来的优化方向：在 delta 编码前对符号位进行 XOR 映射。
//
// deltaEncode 对字节数据进行 delta 编码
// 第一个值原样保留，后续值存储与前一个值的差（无符号整数回绕）
func (dc *DeltaZlibCompressor) deltaEncode(data []byte) []byte {
	elemSize := dc.elemSize
	count := len(data) / elemSize
	if count <= 1 {
		return data
	}
	result := make([]byte, len(data))
	copy(result[:elemSize], data[:elemSize])
	for i := 1; i < count; i++ {
		var cur, prev uint64
		if elemSize == 8 {
			cur = binary.LittleEndian.Uint64(data[i*8 : (i+1)*8])
			prev = binary.LittleEndian.Uint64(data[(i-1)*8 : i*8])
		} else {
			cur = uint64(binary.LittleEndian.Uint32(data[i*4 : (i+1)*4]))
			prev = uint64(binary.LittleEndian.Uint32(data[(i-1)*4 : i*4]))
		}
		delta := cur - prev
		if elemSize == 8 {
			binary.LittleEndian.PutUint64(result[i*8:], delta)
		} else {
			binary.LittleEndian.PutUint32(result[i*4:], uint32(delta))
		}
	}
	return result
}

// deltaDecode 对 delta 编码的字节数据进行解码
func (dc *DeltaZlibCompressor) deltaDecode(data []byte) []byte {
	elemSize := dc.elemSize
	count := len(data) / elemSize
	if count <= 1 {
		return data
	}
	result := make([]byte, len(data))
	copy(result[:elemSize], data[:elemSize])
	for i := 1; i < count; i++ {
		var delta, prev uint64
		if elemSize == 8 {
			delta = binary.LittleEndian.Uint64(data[i*8 : (i+1)*8])
			prev = binary.LittleEndian.Uint64(result[(i-1)*8 : i*8])
		} else {
			delta = uint64(binary.LittleEndian.Uint32(data[i*4 : (i+1)*4]))
			prev = uint64(binary.LittleEndian.Uint32(result[(i-1)*4 : i*4]))
		}
		val := prev + delta
		if elemSize == 8 {
			binary.LittleEndian.PutUint64(result[i*8:], val)
		} else {
			binary.LittleEndian.PutUint32(result[i*4:], uint32(val))
		}
	}
	return result
}

// NoopCompressor 空压缩器，数据原样通过
type NoopCompressor struct{}

func (n *NoopCompressor) Compress(data []byte) ([]byte, error) { return data, nil }
func (n *NoopCompressor) Decompress(data []byte) ([]byte, error) { return data, nil }
