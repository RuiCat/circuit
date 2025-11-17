package utils

// BitmapFlag 位图标记
type BitmapFlag uint64

// Bitmap 位图标记实现
// @ 通过位图标记实现对状态的管理
type Bitmap interface {
	Set(bit BitmapFlag, flag bool)  // 设置标记
	Get(bit BitmapFlag) (flag bool) // 获取标记
	Size() int                      // 位图大小
	FlagCount(flag bool) int        // 标记数量
}

// bitmapImpl 实现Bitmap接口
type bitmapImpl struct {
	bits   []uint64
	length int
}

// NewBitmap 创建新的位图实例
func NewBitmap(size int) Bitmap {
	bitCount := (size + 63) / 64 // 计算需要的uint64数量
	return &bitmapImpl{
		bits:   make([]uint64, bitCount),
		length: size,
	}
}

func (b *bitmapImpl) Set(bit BitmapFlag, flag bool) {
	if int(bit) >= b.length {
		return
	}
	index := int(bit) / 64
	offset := uint(int(bit) % 64)
	if flag {
		b.bits[index] |= (1 << offset)
	} else {
		b.bits[index] &^= (1 << offset)
	}
}

func (b *bitmapImpl) Get(bit BitmapFlag) bool {
	if int(bit) >= b.length {
		return false
	}
	index := int(bit) / 64
	offset := uint(int(bit) % 64)
	return (b.bits[index] & (1 << offset)) != 0
}

func (b *bitmapImpl) Size() int {
	return b.length
}

func (b *bitmapImpl) FlagCount(flag bool) int {
	count := 0
	for i := 0; i < b.length; i++ {
		index := i / 64
		offset := uint(i % 64)
		bitSet := (b.bits[index] & (1 << offset)) != 0
		if bitSet == flag {
			count++
		}
	}
	return count
}
