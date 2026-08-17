package utils

import "math/bits"

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
	if size < 0 {
		size = 0
	}
	bitCount := int((uint64(size) + 63) / 64) // 用 uint64 计算，避免 size+63 溢出
	return &bitmapImpl{
		bits:   make([]uint64, bitCount),
		length: size,
	}
}

func (b *bitmapImpl) Set(bit BitmapFlag, flag bool) {
	// 用无符号比较，避免 bit>=2^63 时 int(bit) 变负数绕过上界检查。
	if bit >= BitmapFlag(b.length) {
		return
	}
	index := int(bit >> 6)
	offset := uint(bit & 63)
	if flag {
		b.bits[index] |= (1 << offset)
	} else {
		b.bits[index] &^= (1 << offset)
	}
}

func (b *bitmapImpl) Get(bit BitmapFlag) bool {
	// 用无符号比较，避免 bit>=2^63 时 int(bit) 变负数绕过上界检查。
	if bit >= BitmapFlag(b.length) {
		return false
	}
	index := int(bit >> 6)
	offset := uint(bit & 63)
	return (b.bits[index] & (1 << offset)) != 0
}

func (b *bitmapImpl) Size() int {
	return b.length
}

func (b *bitmapImpl) FlagCount(flag bool) int {
	if b.length == 0 {
		return 0
	}

	numWords := len(b.bits)
	setCount := 0

	// 处理所有完整的字
	for _, word := range b.bits[:numWords-1] {
		setCount += bits.OnesCount64(word)
	}

	// 处理最后一个字，屏蔽掉未使用的位
	lastWord := b.bits[numWords-1]
	remainingBits := b.length % 64
	if remainingBits > 0 {
		// 创建一个掩码来清除超出位图长度的位
		mask := (uint64(1) << remainingBits) - 1
		setCount += bits.OnesCount64(lastWord & mask)
	} else { // 最后一个字是完整的（或者长度是64的倍数）
		setCount += bits.OnesCount64(lastWord)
	}

	if flag {
		return setCount
	}

	return b.length - setCount
}
