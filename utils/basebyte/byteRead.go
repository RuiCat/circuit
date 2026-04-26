package basebyte

import (
	"encoding/binary"
	"errors"
	"math"
)

var (
	// ErrOutOfBounds 读取偏移量越界错误
	ErrOutOfBounds = errors.New("read offset out of bounds")
	// ErrInvalidData 无效数据长度错误
	ErrInvalidData = errors.New("invalid data length")
)

// Read 字节读取器，从字节数组中顺序读取各种基本数据类型
// 支持大端/小端字节序，自动维护读取偏移量，遇到错误时记录并返回零值
type Read struct {
	Byte   []byte            // 待读取的字节数组
	Offset int               // 当前读取偏移位置
	Order  binary.ByteOrder  // 字节序（大端/小端）
	Error  error             // 读取过程中发生的错误
}

// CheckBounds 检查当前偏移量后是否还有足够的字节可供读取
// 参数required: 需要读取的字节数
// 如果偏移量越界或剩余字节不足，返回ErrOutOfBounds
func (r *Read) CheckBounds(required int) error {
	switch {
	case r.Offset < 0:
		return ErrOutOfBounds
	case r.Offset+required > len(r.Byte):
		return ErrOutOfBounds
	case r.Offset == len(r.Byte):
		return ErrOutOfBounds
	}
	return nil
}

// Bool 从字节数组中读取一个布尔值
// 非零字节为true，零字节为false，偏移量前进1字节
func (r *Read) Bool() (v bool) {
	if err := r.CheckBounds(1); err != nil {
		r.Error = err
		return false
	}
	v = r.Byte[r.Offset] != 0
	r.Offset++
	return v
}

// Int8 从字节数组中读取一个int8值，偏移量前进1字节
func (r *Read) Int8() (v int8) {
	if err := r.CheckBounds(1); err != nil {
		r.Error = err
		return 0
	}
	v = int8(r.Byte[r.Offset])
	r.Offset++
	return v
}

// Uint8 从字节数组中读取一个uint8值，偏移量前进1字节
func (r *Read) Uint8() (v uint8) {
	if err := r.CheckBounds(1); err != nil {
		r.Error = err
		return 0
	}
	v = r.Byte[r.Offset]
	r.Offset++
	return v
}

// Int16 从字节数组中按字节序读取一个int16值，偏移量前进2字节
func (r *Read) Int16() (v int16) {
	if err := r.CheckBounds(2); err != nil {
		r.Error = err
		return 0
	}
	v = int16(r.Order.Uint16(r.Byte[r.Offset:]))
	r.Offset += 2
	return v
}

// Uint16 从字节数组中按字节序读取一个uint16值，偏移量前进2字节
func (r *Read) Uint16() (v uint16) {
	if err := r.CheckBounds(2); err != nil {
		r.Error = err
		return 0
	}
	v = r.Order.Uint16(r.Byte[r.Offset:])
	r.Offset += 2
	return v
}

// Int32 从字节数组中按字节序读取一个int32值，偏移量前进4字节
func (r *Read) Int32() (v int32) {
	if err := r.CheckBounds(4); err != nil {
		r.Error = err
		return 0
	}
	v = int32(r.Order.Uint32(r.Byte[r.Offset:]))
	r.Offset += 4
	return v
}

// Uint32 从字节数组中按字节序读取一个uint32值，偏移量前进4字节
func (r *Read) Uint32() (v uint32) {
	if err := r.CheckBounds(4); err != nil {
		r.Error = err
		return 0
	}
	v = r.Order.Uint32(r.Byte[r.Offset:])
	r.Offset += 4
	return v
}

// Int64 从字节数组中按字节序读取一个int64值，偏移量前进8字节
func (r *Read) Int64() (v int64) {
	if err := r.CheckBounds(8); err != nil {
		r.Error = err
		return 0
	}
	v = int64(r.Order.Uint64(r.Byte[r.Offset:]))
	r.Offset += 8
	return v
}

// Uint64 从字节数组中按字节序读取一个uint64值，偏移量前进8字节
func (r *Read) Uint64() (v uint64) {
	if err := r.CheckBounds(8); err != nil {
		r.Error = err
		return 0
	}
	v = r.Order.Uint64(r.Byte[r.Offset:])
	r.Offset += 8
	return v
}

// Int 从字节数组中读取一个平台相关位数的int值（64位平台读8字节，32位平台读4字节）
// 偏移量前进相应字节数
func (r *Read) Int() (v int) {
	if bit64 {
		v64 := r.Int64()
		v = int(v64)
	} else {
		v32 := r.Int32()
		v = int(v32)
	}
	return v
}

// Uint 从字节数组中读取一个平台相关位数的uint值（64位平台读8字节，32位平台读4字节）
// 偏移量前进相应字节数
func (r *Read) Uint() (v uint) {
	if bit64 {
		v64 := r.Uint64()
		v = uint(v64)
	} else {
		v32 := r.Uint32()
		v = uint(v32)
	}
	return v
}

// Float32 从字节数组中按字节序读取一个float32值，偏移量前进4字节
func (r *Read) Float32() (v float32) {
	bits := r.Uint32()
	return math.Float32frombits(bits)
}

// Float64 从字节数组中按字节序读取一个float64值，偏移量前进8字节
func (r *Read) Float64() (v float64) {
	bits := r.Uint64()
	return math.Float64frombits(bits)
}

// Complex64 从字节数组中读取一个complex64值（由两个float32构成），偏移量前进8字节
func (r *Read) Complex64() (v complex64) {
	real := r.Float32()
	imag := r.Float32()
	return complex(real, imag)
}

// Complex128 从字节数组中读取一个complex128值（由两个float64构成），偏移量前进16字节
func (r *Read) Complex128() (v complex128) {
	real := r.Float64()
	imag := r.Float64()
	return complex(real, imag)
}

// Bytes 从字节数组中读取一个字节切片（先读长度，再读数据），偏移量前进len+4/8字节
func (r *Read) Bytes() (v []byte) {
	length := r.Int()
	if err := r.CheckBounds(length); err != nil {
		r.Error = err
		return nil
	}
	v = make([]byte, length)
	copy(v, r.Byte[r.Offset:r.Offset+length])
	r.Offset += length
	return v
}

// GetByte 从字节数组中读取指定长度的字节切片，偏移量前进i字节
// 参数i: 需要读取的字节数。若i<=0，返回空切片
func (r *Read) GetByte(i int) (v []byte) {
	if i > 0 {
		v = make([]byte, i)
		if err := r.CheckBounds(i); err != nil {
			r.Error = err
			return nil
		}
		copy(v, r.Byte[r.Offset:r.Offset+i])
		r.Offset += i
	} else {
		v = make([]byte, 0)
	}
	return v
}

// ReadChunk8 读取一个8位长度前缀的数据块，返回新的子读取器
// 数据长度为读取到的int8值减去2（扣除长度字段自身占用）
func (r *Read) ReadChunk8() (read *Read) {
	return &Read{
		Byte:  r.GetByte(int(r.Int8() - 2)),
		Order: r.Order,
	}
}

// ReadChunk32 读取一个32位长度前缀的数据块，返回新的子读取器
// 数据长度为读取到的int32值减去4（扣除长度字段自身占用）
func (r *Read) ReadChunk32() (read *Read) {
	return &Read{
		Byte:  r.GetByte(int(r.Int32() - 4)),
		Order: r.Order,
	}
}

// ReadChunk64 读取一个64位长度前缀的数据块，返回新的子读取器
// 数据长度为读取到的int64值减去8（扣除长度字段自身占用）
func (r *Read) ReadChunk64() (read *Read) {
	return &Read{
		Byte:  r.GetByte(int(r.Int64() - 8)),
		Order: r.Order,
	}
}
