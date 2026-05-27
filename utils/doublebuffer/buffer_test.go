package doublebuffer

import (
	"bytes"
	"io"
	"math"
	"testing"
)

// 自定义命名类型，用于验证 ~float32 | ~float64 约束下的序列化正确性
type myFloat float64
type myFloat32 float32

// TestNewBuffer 验证初始化后 dt=0, n/x 正确
func TestNewBuffer(t *testing.T) {
	buf := NewBuffer[float64](100, 10)
	if buf.ActiveLen() != 0 {
		t.Errorf("初始 ActiveLen 期望 0，得到 %d", buf.ActiveLen())
	}
	if buf.Cap() != 100 {
		t.Errorf("Cap 期望 100，得到 %d", buf.Cap())
	}
	if buf.Cols() != 10 {
		t.Errorf("Cols 期望 10，得到 %d", buf.Cols())
	}
}

// TestAppendNoSwitch 追加数据但不触发切换，验证 ActiveLen
func TestAppendNoSwitch(t *testing.T) {
	buf := NewBuffer[float64](10, 3)
	values := []float64{1.0, 2.0, 3.0}
	for i := 0; i < 5; i++ {
		switched, err := buf.Append(values)
		if err != nil {
			t.Fatalf("Append 失败: %v", err)
		}
		if switched {
			t.Errorf("未满时不应切换")
		}
	}
	if buf.ActiveLen() != 5 {
		t.Errorf("ActiveLen 期望 5，得到 %d", buf.ActiveLen())
	}
}

// TestAppendWithSwitch 追加足够数据触发切换，验证 switched=true
func TestAppendWithSwitch(t *testing.T) {
	buf := NewBuffer[float64](5, 2)
	var bb bytes.Buffer
	buf.SetWriter(&bb)

	values := []float64{1.0, 2.0}
	switched := false
	for i := 0; i < 5; i++ {
		s, err := buf.Append(values)
		if err != nil {
			t.Fatalf("Append 失败: %v", err)
		}
		if s {
			switched = true
		}
	}
	if !switched {
		t.Errorf("写满 5 行后应触发切换")
	}
	if buf.ActiveLen() != 0 {
		t.Errorf("切换后 ActiveLen 期望 0，得到 %d", buf.ActiveLen())
	}
}

// TestAppendAutoFlush 验证切换时自动写入文件（用 bytes.Buffer 作为 writer）
func TestAppendAutoFlush(t *testing.T) {
	n := 3
	x := 2
	buf := NewBuffer[float64](n, x)
	var bb bytes.Buffer
	buf.SetWriter(&bb)

	// 写满第一个缓冲区触发切换（第一次切换不刷新非活跃缓冲，因为非活跃缓冲为空）
	for i := 0; i < n; i++ {
		val := float64(i)
		_, err := buf.Append([]float64{val, val + 0.5})
		if err != nil {
			t.Fatalf("Append 失败: %v", err)
		}
	}
	// 此时已切换，bb 应为空（第一次切换不刷新非活跃）
	if bb.Len() != 0 {
		t.Errorf("第一次切换不应有输出，但得到了 %d 字节", bb.Len())
	}

	// 再写满第二个缓冲区触发第二次切换（此时刷新 buffer0）
	for i := 0; i < n; i++ {
		val := float64(i + 10)
		_, err := buf.Append([]float64{val, val + 0.5})
		if err != nil {
			t.Fatalf("Append 失败: %v", err)
		}
	}
	// bb 应有数据（buffer0 被刷新）
	if bb.Len() == 0 {
		t.Errorf("第二次切换后应有输出数据")
	}
}

// TestFlush 手动 Flush 残留数据
func TestFlush(t *testing.T) {
	n := 10
	x := 3
	buf := NewBuffer[float64](n, x)
	var bb bytes.Buffer
	buf.SetWriter(&bb)

	for i := 0; i < 3; i++ {
		_, err := buf.Append([]float64{float64(i), float64(i) + 0.1, float64(i) + 0.2})
		if err != nil {
			t.Fatalf("Append 失败: %v", err)
		}
	}
	if err := buf.Flush(); err != nil {
		t.Fatalf("Flush 失败: %v", err)
	}
	if buf.ActiveLen() != 0 {
		t.Errorf("Flush 后 ActiveLen 期望 0，得到 %d", buf.ActiveLen())
	}
	if bb.Len() == 0 {
		t.Errorf("Flush 后应有输出数据")
	}
}

// TestReadBack 写入后能用 BlockReader 读回来，数据一致
func TestReadBack(t *testing.T) {
	n := 5
	x := 2
	buf := NewBuffer[float64](n, x)
	var bb bytes.Buffer
	buf.SetWriter(&bb)
	buf.SetCompressor(NewZlibCompressor())

	expected := make([]float64, 0)
	for i := 0; i < 3; i++ {
		v := []float64{float64(i), float64(i) + 0.5}
		expected = append(expected, v...)
		_, err := buf.Append(v)
		if err != nil {
			t.Fatalf("Append 失败: %v", err)
		}
	}
	if err := buf.Flush(); err != nil {
		t.Fatalf("Flush 失败: %v", err)
	}
	if bb.Len() == 0 {
		t.Fatal("写入后缓冲区为空")
	}

	// 读回
	reader := NewBlockReader(&bb, NewZlibCompressor())
	dt, xRead, rawData, err := reader.ReadBlock()
	if err != nil {
		t.Fatalf("ReadBlock 失败: %v", err)
	}
	if dt != 3 {
		t.Errorf("dt 期望 3，得到 %d", dt)
	}
	if xRead != 2 {
		t.Errorf("x 期望 2，得到 %d", xRead)
	}

	restored, err := unflatten[float64](rawData, dt, xRead)
	if err != nil {
		t.Fatalf("unflatten 失败: %v", err)
	}
	for i := 0; i < dt; i++ {
		for j := 0; j < xRead; j++ {
			idx := i*xRead + j
			if math.Abs(restored[i][j]-expected[idx]) > 1e-9 {
				t.Errorf("数据不一致: 位置 [%d][%d] 期望 %v，得到 %v", i, j, expected[idx], restored[i][j])
			}
		}
	}
}

// TestCompressorRoundtrip ZlibCompressor 压缩-解压往返
func TestCompressorRoundtrip(t *testing.T) {
	original := []byte("hello world, this is a test for zlib compressor roundtrip")
	zc := NewZlibCompressor()

	compressed, err := zc.Compress(original)
	if err != nil {
		t.Fatalf("ZlibCompressor.Compress 失败: %v", err)
	}
	if len(compressed) == 0 {
		t.Fatal("ZlibCompressor.Compress 返回空数据")
	}

	decompressed, err := zc.Decompress(compressed)
	if err != nil {
		t.Fatalf("ZlibCompressor.Decompress 失败: %v", err)
	}

	if string(decompressed) != string(original) {
		t.Errorf("解压数据不匹配: 期望 %q，得到 %q", original, decompressed)
	}
}

// TestDeltaCompressorRoundtrip DeltaZlibCompressor 压缩-解压往返 (float64)
func TestDeltaCompressorRoundtrip(t *testing.T) {
	// 构造 float64 数据的 []byte 表示
	values := []float64{1.0, 2.0, 3.0, 4.0, 5.0, 3.0, 1.0, 100.0}
	raw := flatten[float64](to2D(values, 1))

	dc := NewDeltaZlibCompressor(8)
	compressed, err := dc.Compress(raw)
	if err != nil {
		t.Fatalf("DeltaZlibCompressor.Compress 失败: %v", err)
	}

	decompressed, err := dc.Decompress(compressed)
	if err != nil {
		t.Fatalf("DeltaZlibCompressor.Decompress 失败: %v", err)
	}

	restored, err := unflatten[float64](decompressed, len(values), 1)
	if err != nil {
		t.Fatalf("unflatten 失败: %v", err)
	}
	for i, v := range values {
		if math.Abs(restored[i][0]-v) > 1e-9 {
			t.Errorf("Delta 往返数据不一致: 位置 %d 期望 %v，得到 %v", i, v, restored[i][0])
		}
	}
}

// TestDeltaCompressorFloat32 float32 类型的 delta 压缩往返
func TestDeltaCompressorFloat32(t *testing.T) {
	values := []float32{1.0, 2.5, 3.7, 4.2, 5.0, 0.1}
	raw := flatten[float32](to2D32(values, 1))

	dc := NewDeltaZlibCompressor(4)
	compressed, err := dc.Compress(raw)
	if err != nil {
		t.Fatalf("DeltaZlibCompressor.Compress(float32) 失败: %v", err)
	}

	decompressed, err := dc.Decompress(compressed)
	if err != nil {
		t.Fatalf("DeltaZlibCompressor.Decompress(float32) 失败: %v", err)
	}

	restored, err := unflatten[float32](decompressed, len(values), 1)
	if err != nil {
		t.Fatalf("unflatten 失败: %v", err)
	}
	for i, v := range values {
		if math.Abs(float64(restored[i][0]-v)) > 1e-6 {
			t.Errorf("Delta(float32) 往返数据不一致: 位置 %d 期望 %v，得到 %v", i, v, restored[i][0])
		}
	}
}

// TestDeltaCompressorFloat32Even4 4 个 float32（8 字节）的往返测试
// 验证不会因字节数能被 8 整除而被错误识别为 float64
func TestDeltaCompressorFloat32Even4(t *testing.T) {
	values := []float32{1.0, 2.5, 3.7, 4.2}
	raw := flatten[float32](to2D32(values, 1))

	dc := NewDeltaZlibCompressor(4)
	compressed, err := dc.Compress(raw)
	if err != nil {
		t.Fatalf("DeltaZlibCompressor.Compress(float32 even4) 失败: %v", err)
	}

	decompressed, err := dc.Decompress(compressed)
	if err != nil {
		t.Fatalf("DeltaZlibCompressor.Decompress(float32 even4) 失败: %v", err)
	}

	restored, err := unflatten[float32](decompressed, len(values), 1)
	if err != nil {
		t.Fatalf("unflatten 失败: %v", err)
	}
	for i, v := range values {
		if math.Abs(float64(restored[i][0]-v)) > 1e-6 {
			t.Errorf("Delta(float32 even4) 往返数据不一致: 位置 %d 期望 %v，得到 %v", i, v, restored[i][0])
		}
	}
}

// TestDeltaCompressorFloat32Even8 8 个 float32（16 字节）的往返测试
func TestDeltaCompressorFloat32Even8(t *testing.T) {
	values := []float32{1.0, 2.5, 3.7, 4.2, 5.0, 6.3, 7.1, 8.9}
	raw := flatten[float32](to2D32(values, 1))

	dc := NewDeltaZlibCompressor(4)
	compressed, err := dc.Compress(raw)
	if err != nil {
		t.Fatalf("DeltaZlibCompressor.Compress(float32 even8) 失败: %v", err)
	}

	decompressed, err := dc.Decompress(compressed)
	if err != nil {
		t.Fatalf("DeltaZlibCompressor.Decompress(float32 even8) 失败: %v", err)
	}

	restored, err := unflatten[float32](decompressed, len(values), 1)
	if err != nil {
		t.Fatalf("unflatten 失败: %v", err)
	}
	for i, v := range values {
		if math.Abs(float64(restored[i][0]-v)) > 1e-6 {
			t.Errorf("Delta(float32 even8) 往返数据不一致: 位置 %d 期望 %v，得到 %v", i, v, restored[i][0])
		}
	}
}

// TestAppendInvalidLength Append 长度不等于 x 时返回错误
func TestAppendInvalidLength(t *testing.T) {
	buf := NewBuffer[float64](10, 3)
	_, err := buf.Append([]float64{1.0, 2.0})
	if err == nil {
		t.Error("期望 Append 返回错误，但成功执行")
	}
	if err != ErrInvalidLength {
		t.Errorf("期望 ErrInvalidLength，得到 %v", err)
	}
}

// TestReset Reset 后 ActiveLen 归零
func TestReset(t *testing.T) {
	buf := NewBuffer[float64](10, 3)
	values := []float64{1.0, 2.0, 3.0}
	for i := 0; i < 5; i++ {
		_, err := buf.Append(values)
		if err != nil {
			t.Fatalf("Append 失败: %v", err)
		}
	}
	if buf.ActiveLen() != 5 {
		t.Errorf("Append 后 ActiveLen 期望 5，得到 %d", buf.ActiveLen())
	}
	buf.Reset()
	if buf.ActiveLen() != 0 {
		t.Errorf("Reset 后 ActiveLen 期望 0，得到 %d", buf.ActiveLen())
	}
}

// TestFlushWithoutWriter 未设置 writer 时 Flush 应返回错误
func TestFlushWithoutWriter(t *testing.T) {
	buf := NewBuffer[float64](10, 3)
	_, err := buf.Append([]float64{1.0, 2.0, 3.0})
	if err != nil {
		t.Fatalf("Append 失败: %v", err)
	}
	err = buf.Flush()
	if err != ErrWriterNotSet {
		t.Errorf("期望 ErrWriterNotSet，得到 %v", err)
	}
}

// TestMultipleBlocks 多次写入、多次读取
func TestMultipleBlocks(t *testing.T) {
	n := 3
	x := 2
	buf := NewBuffer[float64](n, x)
	var bb bytes.Buffer
	buf.SetWriter(&bb)
	buf.SetCompressor(NewZlibCompressor())

	// 写入 10 行，每 3 行切换一次
	totalRows := 10
	for i := 0; i < totalRows; i++ {
		v := []float64{float64(i), float64(i) + 0.1}
		_, err := buf.Append(v)
		if err != nil {
			t.Fatalf("Append 失败: %v", err)
		}
	}
	// Flush 残留
	if err := buf.Flush(); err != nil {
		t.Fatalf("Flush 失败: %v", err)
	}

	// 读回所有块
	reader := NewBlockReader(&bb, NewZlibCompressor())
	rowsRead := 0
	for {
		dt, xRead, rawData, err := reader.ReadBlock()
		if err != nil {
			break // io.EOF 或 io.ErrUnexpectedEOF
		}
		if xRead != x {
			t.Errorf("x 期望 %d，得到 %d", x, xRead)
		}
		restored, err := unflatten[float64](rawData, dt, xRead)
		if err != nil {
			t.Fatalf("unflatten 失败: %v", err)
		}
		for i := 0; i < dt; i++ {
			for j := 0; j < xRead; j++ {
				expected := float64(rowsRead) + float64(j)*0.1
				if math.Abs(restored[i][j]-expected) > 1e-9 {
					t.Errorf("块数据不一致: 全局行 %d, 列 %d, 期望 %v，得到 %v", rowsRead, j, expected, restored[i][j])
				}
			}
			rowsRead++
		}
	}
	if rowsRead != totalRows {
		t.Errorf("总行数期望 %d，得到 %d", totalRows, rowsRead)
	}
}

// TestNamedFloatType 验证命名 float64 类型正确序列化
func TestNamedFloatType(t *testing.T) {
	original := [][]myFloat{
		{1.5, 2.5},
		{3.5, 4.5},
	}
	raw := flatten(original)
	expectedLen := 2 * 2 * 8
	if len(raw) != expectedLen {
		t.Fatalf("flatten 长度期望 %d，实际 %d", expectedLen, len(raw))
	}
	restored, err := unflatten[myFloat](raw, 2, 2)
	if err != nil {
		t.Fatal("unflatten 失败:", err)
	}
	for i := range original {
		for j := range original[i] {
			if restored[i][j] != original[i][j] {
				t.Errorf("位置 [%d][%d] 数据不一致: 期望 %v，实际 %v", i, j, original[i][j], restored[i][j])
			}
		}
	}
}

// TestNamedFloat32Type 验证命名 float32 类型正确序列化
func TestNamedFloat32Type(t *testing.T) {
	original := [][]myFloat32{
		{1.5, 2.5, 3.5},
		{4.5, 5.5, 6.5},
	}
	raw := flatten(original)
	expectedLen := 2 * 3 * 4
	if len(raw) != expectedLen {
		t.Fatalf("flatten 长度期望 %d，实际 %d", expectedLen, len(raw))
	}
	restored, err := unflatten[myFloat32](raw, 2, 3)
	if err != nil {
		t.Fatal("unflatten 失败:", err)
	}
	for i := range original {
		for j := range original[i] {
			if restored[i][j] != original[i][j] {
				t.Errorf("位置 [%d][%d] 数据不一致: 期望 %v，实际 %v", i, j, original[i][j], restored[i][j])
			}
		}
	}
}

// to2D 将一维切片转为单列二维切片（float64）
func to2D(values []float64, cols int) [][]float64 {
	rows := len(values) / cols
	result := make([][]float64, rows)
	for i := 0; i < rows; i++ {
		result[i] = make([]float64, cols)
		for j := 0; j < cols; j++ {
			result[i][j] = values[i*cols+j]
		}
	}
	return result
}

func to2D32(values []float32, cols int) [][]float32 {
	rows := len(values) / cols
	result := make([][]float32, rows)
	for i := 0; i < rows; i++ {
		result[i] = make([]float32, cols)
		for j := 0; j < cols; j++ {
			result[i][j] = values[i*cols+j]
		}
	}
	return result
}

// TestBufferReader 验证 BufferReader 正确读取已写入的块
func TestBufferReader(t *testing.T) {
	var bb bytes.Buffer

	// 写入端
	buf := NewBuffer[float64](5, 2)
	buf.SetWriter(&bb)
	buf.SetCompressor(NewZlibCompressor())

	// 写满一个块（5行）
	for i := 0; i < 5; i++ {
		buf.Append([]float64{float64(i), float64(i * 10)})
	}
	buf.Flush()

	// 读取端
	reader := NewBufferReader[float64](&bb, NewZlibCompressor(), nil)
	data, header, err := reader.ReadBlock()
	if err != nil {
		t.Fatal("ReadBlock 失败:", err)
	}

	if header.Dt != 5 || header.X != 2 {
		t.Fatalf("块头不正确: dt=%d, x=%d", header.Dt, header.X)
	}
	if len(data) != 5 || len(data[0]) != 2 {
		t.Fatalf("数据维度不正确: %dx%d", len(data), len(data[0]))
	}

	// 验证数据正确
	for i := 0; i < 5; i++ {
		if data[i][0] != float64(i) || data[i][1] != float64(i*10) {
			t.Errorf("数据不一致 [%d]: 期望 %.0f,%.0f 实际 %.0f,%.0f",
				i, float64(i), float64(i*10), data[i][0], data[i][1])
		}
	}
}

// TestBufferReaderMultipleBlocks 验证多块顺序读取
func TestBufferReaderMultipleBlocks(t *testing.T) {
	var bb bytes.Buffer

	buf := NewBuffer[float64](3, 1)
	buf.SetWriter(&bb)
	buf.SetCompressor(NewZlibCompressor())

	// 写入 8 行，会触发 2 次切换 = 2 个满块 + 1 个残块
	for i := 0; i < 8; i++ {
		buf.Append([]float64{float64(i)})
	}
	buf.Flush()

	// 读取
	reader := NewBufferReader[float64](&bb, NewZlibCompressor(), nil)
	totalRows := 0
	for {
		data, header, err := reader.ReadBlock()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal("ReadBlock 失败:", err)
		}
		totalRows += header.Dt
		for _, row := range data {
			_ = row
		}
	}
	if totalRows != 8 {
		t.Errorf("总行数不正确: 期望 8，实际 %d", totalRows)
	}
}

// TestBufferNewReader 验证 Buffer.NewReader 方法
func TestBufferNewReader(t *testing.T) {
	var bb bytes.Buffer

	buf := NewBuffer[float64](3, 1)
	buf.SetWriter(&bb)
	buf.SetCompressor(NewDeltaZlibCompressor(8))

	buf.Append([]float64{1.5})
	buf.Append([]float64{2.5})
	buf.Append([]float64{3.5})
	buf.Flush()

	// 使用 Buffer.NewReader 创建读取器
	reader := buf.NewReader(&bb)
	data, header, err := reader.ReadBlock()
	if err != nil {
		t.Fatal("ReadBlock 失败:", err)
	}
	if header.Dt != 3 {
		t.Fatalf("期望 dt=3，实际 %d", header.Dt)
	}
	if data[0][0] != 1.5 || data[1][0] != 2.5 || data[2][0] != 3.5 {
		t.Fatal("数据不一致")
	}
}


// TestRLECodecRoundtrip 验证 RLE 编解码往返正确性
func TestRLECodecRoundtrip(t *testing.T) {
	codec := NewRLECompressor[float64]()

	// 模拟 RC 充电数据：节点0不变，节点1指数变化
	data := make([][]float64, 100)
	for i := 0; i < 100; i++ {
		data[i] = []float64{5.0, 5.0 * (1 - math.Exp(-float64(i)*0.01))}
	}

	encoded, err := codec.Encode(data)
	if err != nil {
		t.Fatal("Encode 失败:", err)
	}

	decoded, err := codec.Decode(encoded)
	if err != nil {
		t.Fatal("Decode 失败:", err)
	}

	if len(decoded) != 100 || len(decoded[0]) != 2 {
		t.Fatalf("维度错误: %dx%d", len(decoded), len(decoded[0]))
	}

	// 验证每行每列误差 < 1e-10
	for i := 0; i < 100; i++ {
		for j := 0; j < 2; j++ {
			if math.Abs(float64(data[i][j]-decoded[i][j])) > 1e-10 {
				t.Errorf("[%d][%d] 不一致: %.10f vs %.10f", i, j, data[i][j], decoded[i][j])
			}
		}
	}

	// 压缩率：原始 100*2*8=1600, 编码后应该 < 200
	t.Logf("RLE 编码: %d 字节 (原始 %d 字节, 压缩率 %.1f%%)",
		len(encoded), 1600, float64(len(encoded))/16.0)
}

// TestRLECodecStableSignal 验证全稳定信号的压缩率
func TestRLECodecStableSignal(t *testing.T) {
	codec := NewRLECompressor[float64]()

	// 全部不变的信号
	data := make([][]float64, 500)
	for i := 0; i < 500; i++ {
		data[i] = []float64{3.3, 0.0}
	}

	encoded, err := codec.Encode(data)
	if err != nil {
		t.Fatal("Encode 失败:", err)
	}

	decoded, err := codec.Decode(encoded)
	if err != nil {
		t.Fatal("Decode 失败:", err)
	}

	if len(decoded) != 500 {
		t.Fatalf("行数错误: %d", len(decoded))
	}

	t.Logf("全稳定信号 RLE: %d 字节 (原始 %d 字节, 压缩率 %.1f%%)",
		len(encoded), 500*2*8, float64(len(encoded))/float64(500*2*8)*100)

	// 全稳定信号压缩率应 < 5%
	ratio := float64(len(encoded)) / float64(500*2*8) * 100
	if ratio > 5 {
		t.Errorf("全稳定信号压缩率过高: %.1f%%", ratio)
	}
}

// TestRLEWithCodecBuffer 验证 RLE 通过 Buffer+SetCodec 的完整往返
func TestRLEWithCodecBuffer(t *testing.T) {
	n := 5
	x := 2
	buf := NewBuffer[float64](n, x)
	var bb bytes.Buffer
	buf.SetWriter(&bb)
	buf.SetCodec(NewRLECompressor[float64]())

	expected := make([]float64, 0)
	for i := 0; i < 7; i++ {
		v := []float64{float64(i), float64(i) + 0.5}
		expected = append(expected, v...)
		_, err := buf.Append(v)
		if err != nil {
			t.Fatalf("Append 失败: %v", err)
		}
	}
	if err := buf.Flush(); err != nil {
		t.Fatalf("Flush 失败: %v", err)
	}

	// 读回
	reader := buf.NewReader(&bb)
	totalRows := 0
	for {
		data, _, err := reader.ReadBlock()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal("ReadBlock 失败:", err)
		}
		for i := 0; i < len(data); i++ {
			for j := 0; j < len(data[i]); j++ {
				idx := totalRows*x + j
				if math.Abs(data[i][j]-expected[idx]) > 1e-10 {
					t.Errorf("数据不一致: 位置 [%d][%d] 期望 %v，得到 %v",
						totalRows+i, j, expected[idx], data[i][j])
				}
			}
			totalRows++
		}
	}
	if totalRows != 7 {
		t.Errorf("总行数期望 7，得到 %d", totalRows)
	}
}

// TestRLECodecFloat32 验证 RLE 对 float32 的编解码往返
func TestRLECodecFloat32(t *testing.T) {
	codec := NewRLECompressor[float32]()

	data := make([][]float32, 50)
	for i := 0; i < 50; i++ {
		data[i] = []float32{1.5, float32(i) * 0.5}
	}

	encoded, err := codec.Encode(data)
	if err != nil {
		t.Fatal("Encode 失败:", err)
	}

	decoded, err := codec.Decode(encoded)
	if err != nil {
		t.Fatal("Decode 失败:", err)
	}

	if len(decoded) != 50 || len(decoded[0]) != 2 {
		t.Fatalf("维度错误: %dx%d", len(decoded), len(decoded[0]))
	}

	for i := 0; i < 50; i++ {
		for j := 0; j < 2; j++ {
			if math.Abs(float64(data[i][j]-decoded[i][j])) > 1e-6 {
				t.Errorf("[%d][%d] 不一致: %.6f vs %.6f", i, j, data[i][j], decoded[i][j])
			}
		}
	}

	t.Logf("RLE(float32) 编码: %d 字节 (原始 %d 字节, 压缩率 %.1f%%)",
		len(encoded), 50*2*4, float64(len(encoded))/float64(50*2*4)*100)
}
