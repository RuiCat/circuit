package basebyte

import (
	"encoding/binary"
	"math"
)

// bit64 int是否为64
var bit64 = (^uint(0) >> 64) == 0

// Write 写
type Write struct {
	Byte  []byte
	Order binary.ByteOrder
}

// Bool 逻辑型
func (w *Write) Bool(v bool) {
	if v {
		w.Byte = append(w.Byte, 1)
	} else {
		w.Byte = append(w.Byte, 0)
	}
}

// Int8 单字节整数
func (w *Write) Int8(v int8) {
	w.Byte = append(w.Byte, byte(v))
}

// Uint8  单字节正整数
func (w *Write) Uint8(v uint8) {
	w.Byte = append(w.Byte, v)
}

// Int16 双字节整数
func (w *Write) Int16(v int16) {
	w.Uint16(uint16(v))
}

// Uint16 双字节正整数
func (w *Write) Uint16(v uint16) {
	b := make([]byte, 2)
	w.Order.PutUint16(b, v)
	w.Byte = append(w.Byte, b...)
}

// Int32 四字节整数
func (w *Write) Int32(v int32) {
	w.Uint32(uint32(v))
}

// Uint32 四字节正整数
func (w *Write) Uint32(v uint32) {
	b := make([]byte, 4)
	w.Order.PutUint32(b, v)
	w.Byte = append(w.Byte, b...)
}

// Int64 八字节整数
func (w *Write) Int64(v int64) {
	w.Uint64(uint64(v))
}

// Uint64 八字节正整数
func (w *Write) Uint64(v uint64) {
	b := make([]byte, 8)
	w.Order.PutUint64(b, v)
	w.Byte = append(w.Byte, b...)
}

// Int 平台相关整数
func (w *Write) Int(v int) {
	if bit64 {
		w.Int64(int64(v))
	} else {
		w.Int32(int32(v))
	}
}

// Uint 平台相关正整数
func (w *Write) Uint(v uint) {
	if bit64 {
		w.Uint64(uint64(v))
	} else {
		w.Uint32(uint32(v))
	}
}

// Float32 浮点数
func (w *Write) Float32(v float32) {
	w.Uint32(math.Float32bits(v))
}

// Float64 浮点数
func (w *Write) Float64(v float64) {
	w.Uint64(math.Float64bits(v))
}

// Complex64 复数
func (w *Write) Complex64(v complex64) {
	w.Float32(float32(real(v)))
	w.Float32(float32(imag(v)))
}

// Complex128 复数
func (w *Write) Complex128(v complex128) {
	w.Float64(float64(real(v)))
	w.Float64(float64(imag(v)))
}

// Bytes 字节集
func (w *Write) Bytes(v []byte) {
	w.Int(len(v))
	w.Byte = append(w.Byte, v...)
}

// SetByte 字节
func (w *Write) SetByte(v []byte) {
	w.Byte = append(w.Byte, v...)
}
