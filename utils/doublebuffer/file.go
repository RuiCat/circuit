package doublebuffer

import (
	"encoding/binary"
	"fmt"
	"io"
)

// 文件块魔数：0x56425420
const blockMagic uint32 = 0x56425420

// maxCompressedSize 压缩数据最大字节数上限，防止 OOM
const maxCompressedSize = 256 * 1024 * 1024 // 256 MB

// BlockWriter 块写入器
type BlockWriter struct {
	w          io.Writer
	compressor Compressor
}

// NewBlockWriter 创建块写入器实例
func NewBlockWriter(w io.Writer, c Compressor) *BlockWriter {
	return &BlockWriter{
		w:          w,
		compressor: c,
	}
}

// WriteBlock 将原始字节数据压缩后写入文件块
// dt: 块行数, x: 每行列数, rawData: 展平后的原始字节数据
func (bw *BlockWriter) WriteBlock(dt, x int, rawData []byte) error {
	compressed, err := bw.compressor.Compress(rawData)
	if err != nil {
		return fmt.Errorf("压缩数据失败: %w", err)
	}

	// 写入头部
	header := make([]byte, 16)
	binary.LittleEndian.PutUint32(header[0:4], blockMagic)
	binary.LittleEndian.PutUint32(header[4:8], uint32(dt))
	binary.LittleEndian.PutUint32(header[8:12], uint32(x))
	binary.LittleEndian.PutUint32(header[12:16], uint32(len(compressed)))

	if _, err := bw.w.Write(header); err != nil {
		return fmt.Errorf("写入块头失败: %w", err)
	}
	if _, err := bw.w.Write(compressed); err != nil {
		return fmt.Errorf("写入压缩数据失败: %w", err)
	}
	return nil
}

// BlockReader 块读取器
type BlockReader struct {
	r          io.Reader
	compressor Compressor
	buf        []byte // 内部读取缓冲
}

// NewBlockReader 创建块读取器实例
func NewBlockReader(r io.Reader, c Compressor) *BlockReader {
	return &BlockReader{
		r:          r,
		compressor: c,
		buf:        make([]byte, 16),
	}
}

// ReadBlock 读取下一个块，返回原始字节数据及块头信息
func (br *BlockReader) ReadBlock() (dt int, x int, rawData []byte, err error) {
	// 读取头部 16 字节
	if _, err := io.ReadFull(br.r, br.buf); err != nil {
		return 0, 0, nil, err
	}

	magic := binary.LittleEndian.Uint32(br.buf[0:4])
	if magic != blockMagic {
		return 0, 0, nil, fmt.Errorf("无效的魔数: 0x%08X", magic)
	}

	dt = int(binary.LittleEndian.Uint32(br.buf[4:8]))
	x = int(binary.LittleEndian.Uint32(br.buf[8:12]))
	compressedLen := int(binary.LittleEndian.Uint32(br.buf[12:16]))

	if compressedLen > maxCompressedSize {
		return 0, 0, nil, fmt.Errorf("压缩数据长度 %d 超出上限 %d", compressedLen, maxCompressedSize)
	}

	// 读取压缩数据
	compressed := make([]byte, compressedLen)
	if _, err := io.ReadFull(br.r, compressed); err != nil {
		return 0, 0, nil, fmt.Errorf("读取压缩数据失败: %w", err)
	}

	// 解压
	rawData, err = br.compressor.Decompress(compressed)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("解压数据失败: %w", err)
	}

	return dt, x, rawData, nil
}
