package basebyte

import (
	"encoding/binary"
	"errors"
	"math"
)

var (
	ErrOutOfBounds = errors.New("read offset out of bounds")
	ErrInvalidData = errors.New("invalid data length")
)

// Read 读
type Read struct {
	Byte   []byte
	Offset int
	Order  binary.ByteOrder
	Error  error
}

// CheckBounds 检查边界
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

// Bool 逻辑型
func (r *Read) Bool() (v bool) {
	if err := r.CheckBounds(1); err != nil {
		r.Error = err
		return false
	}
	v = r.Byte[r.Offset] != 0
	r.Offset++
	return v
}

// Int8 单字节整数
func (r *Read) Int8() (v int8) {
	if err := r.CheckBounds(1); err != nil {
		r.Error = err
		return 0
	}
	v = int8(r.Byte[r.Offset])
	r.Offset++
	return v
}

// Uint8  单字节正整数
func (r *Read) Uint8() (v uint8) {
	if err := r.CheckBounds(1); err != nil {
		r.Error = err
		return 0
	}
	v = r.Byte[r.Offset]
	r.Offset++
	return v
}

// Int16 双字节整数
func (r *Read) Int16() (v int16) {
	if err := r.CheckBounds(2); err != nil {
		r.Error = err
		return 0
	}
	v = int16(r.Order.Uint16(r.Byte[r.Offset:]))
	r.Offset += 2
	return v
}

// Uint16 双字节正整数
func (r *Read) Uint16() (v uint16) {
	if err := r.CheckBounds(2); err != nil {
		r.Error = err
		return 0
	}
	v = r.Order.Uint16(r.Byte[r.Offset:])
	r.Offset += 2
	return v
}

// Int32 四字节整数
func (r *Read) Int32() (v int32) {
	if err := r.CheckBounds(4); err != nil {
		r.Error = err
		return 0
	}
	v = int32(r.Order.Uint32(r.Byte[r.Offset:]))
	r.Offset += 4
	return v
}

// Uint32 四字节正整数
func (r *Read) Uint32() (v uint32) {
	if err := r.CheckBounds(4); err != nil {
		r.Error = err
		return 0
	}
	v = r.Order.Uint32(r.Byte[r.Offset:])
	r.Offset += 4
	return v
}

// Int64 八字节整数
func (r *Read) Int64() (v int64) {
	if err := r.CheckBounds(8); err != nil {
		r.Error = err
		return 0
	}
	v = int64(r.Order.Uint64(r.Byte[r.Offset:]))
	r.Offset += 8
	return v
}

// Uint64 八字节正整数
func (r *Read) Uint64() (v uint64) {
	if err := r.CheckBounds(8); err != nil {
		r.Error = err
		return 0
	}
	v = r.Order.Uint64(r.Byte[r.Offset:])
	r.Offset += 8
	return v
}

// Int 平台相关整数
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

// Uint 平台相关正整数
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

// Float32 浮点数
func (r *Read) Float32() (v float32) {
	bits := r.Uint32()
	return math.Float32frombits(bits)
}

// Float64 浮点数
func (r *Read) Float64() (v float64) {
	bits := r.Uint64()
	return math.Float64frombits(bits)
}

// Complex64 复数
func (r *Read) Complex64() (v complex64) {
	real := r.Float32()
	imag := r.Float32()
	return complex(real, imag)
}

// Complex128 复数
func (r *Read) Complex128() (v complex128) {
	real := r.Float64()
	imag := r.Float64()
	return complex(real, imag)
}

// Bytes 字节集
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

// GetByte 得到字节集
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

// ReadChunk8 读取块
func (r *Read) ReadChunk8() (read *Read) {
	return &Read{
		Byte:  r.GetByte(int(r.Int8() - 2)),
		Order: r.Order,
	}
}

// ReadChunk32 读取块
func (r *Read) ReadChunk32() (read *Read) {
	return &Read{
		Byte:  r.GetByte(int(r.Int32() - 4)),
		Order: r.Order,
	}
}

// ReadChunk64 读取块
func (r *Read) ReadChunk64() (read *Read) {
	return &Read{
		Byte:  r.GetByte(int(r.Int64() - 8)),
		Order: r.Order,
	}
}
