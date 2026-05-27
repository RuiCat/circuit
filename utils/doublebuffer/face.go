package doublebuffer

import "io"

// Number 电压值类型约束，只支持浮点类型
type Number interface {
	~float32 | ~float64
}

// Buffer 双缓冲接口
// 非并发安全，调用方负责同步
// 用于存储连续的多维数组值 [dt+1][x]T
// 两个缓冲区交替工作，当一个缓冲区写满( dt >= n )时自动切换并刷新到文件
type Buffer[T Number] interface {
	// Append 追加一行数据到活跃缓冲区
	// 若 dt >= n 则自动切换缓冲并将非活跃缓冲压缩写入文件
	// 返回 switched 表示是否发生了缓冲切换
	Append(values []T) (switched bool, err error)

	// ActiveLen 返回当前活跃缓冲已写入的行数 (dt)
	ActiveLen() int

	// Cap 返回单个缓冲区的最大行数 (n)
	Cap() int

	// Cols 返回每行的节点数 (x)
	Cols() int

	// Flush 手动刷新所有缓冲中的残留数据到文件
	// 先刷新非活跃缓冲（若有），再刷新当前活跃缓冲（若有）
	// 用于仿真结束时处理未写满的残留数据
	Flush() error

	// SetWriter 设置写入目标
	SetWriter(w io.Writer)

	// SetCompressor 设置压缩器（默认为 ZlibCompressor）
	SetCompressor(c Compressor)

	// SetCodec 设置块编解码器（替代 Compressor，在结构化层面压缩）
	SetCodec(c BlockCodec[T])

	// Reset 重置缓冲区状态（清空活跃缓冲但不影响文件）
	Reset()

	// NewReader 基于当前压缩器配置创建一个泛型块读取器
	// 用于从文件中读取之前写入的块数据
	NewReader(r io.Reader) *BufferReader[T]
}

// Compressor 压缩器接口
type Compressor interface {
	// Compress 压缩字节数据
	Compress(data []byte) ([]byte, error)
	// Decompress 解压字节数据
	Decompress(data []byte) ([]byte, error)
}

// BlockHeader 文件块头部信息
type BlockHeader struct {
	Dt int // 该块中的行数
	X  int // 每行的列数
}

// BlockCodec 块编解码器接口
// 在 [][]T 层面进行列感知压缩，可直接操作结构化数据
// Encode 产生的字节流自带 dt/x 维度信息，Decoder 无需额外参数
type BlockCodec[T Number] interface {
	// Encode 将二维数据编码为字节流
	Encode(data [][]T) ([]byte, error)
	// Decode 将编码字节流解码为二维数据
	Decode(encoded []byte) ([][]T, error)
}
