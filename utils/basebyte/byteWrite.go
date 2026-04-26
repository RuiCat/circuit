package basebyte

import (
	"encoding/binary"
	"math"
)

// bit64 检测当前平台int类型是否为64位（编译时常量）
var bit64 = (^uint(0) >> 64) == 0

// Write 字节写入器，将各种基本数据类型按顺序写入字节数组
// 支持大端/小端字节序，数据通过追加方式写入Byte切片
type Write struct {
	Byte  []byte           // 已写入的字节数据
	Order binary.ByteOrder // 字节序（大端/小端）
}

// Bool 写入一个布尔值（true写1，false写0），追加1字节
func (w *Write) Bool(v bool) {
	if v {
		w.Byte = append(w.Byte, 1)
	} else {
		w.Byte = append(w.Byte, 0)
	}
}

// Int8 写入一个int8值，追加1字节
func (w *Write) Int8(v int8) {
	w.Byte = append(w.Byte, byte(v))
}

// Uint8 写入一个uint8值，追加1字节
func (w *Write) Uint8(v uint8) {
	w.Byte = append(w.Byte, v)
}

// Int16 写入一个int16值，按字节序追加2字节
func (w *Write) Int16(v int16) {
	w.Uint16(uint16(v))
}

// Uint16 写入一个uint16值，按字节序追加2字节
func (w *Write) Uint16(v uint16) {
	b := make([]byte, 2)
	w.Order.PutUint16(b, v)
	w.Byte = append(w.Byte, b...)
}

// Int32 写入一个int32值，按字节序追加4字节
func (w *Write) Int32(v int32) {
	w.Uint32(uint32(v))
}

// Uint32 写入一个uint32值，按字节序追加4字节
func (w *Write) Uint32(v uint32) {
	b := make([]byte, 4)
	w.Order.PutUint32(b, v)
	w.Byte = append(w.Byte, b...)
}

// Int64 写入一个int64值，按字节序追加8字节
func (w *Write) Int64(v int64) {
	w.Uint64(uint64(v))
}

// Uint64 写入一个uint64值，按字节序追加8字节
func (w *Write) Uint64(v uint64) {
	b := make([]byte, 8)
	w.Order.PutUint64(b, v)
	w.Byte = append(w.Byte, b...)
}

// Int 写入一个平台相关位数的int值（64位平台写8字节，32位平台写4字节）
func (w *Write) Int(v int) {
	if bit64 {
		w.Int64(int64(v))
	} else {
		w.Int32(int32(v))
	}
}

// Uint 写入一个平台相关位数的uint值（64位平台写8字节，32位平台写4字节）
func (w *Write) Uint(v uint) {
	if bit64 {
		w.Uint64(uint64(v))
	} else {
		w.Uint32(uint32(v))
	}
}

// Float32 写入一个float32值，按字节序追加4字节
func (w *Write) Float32(v float32) {
	w.Uint32(math.Float32bits(v))
}

// Float64 写入一个float64值，按字节序追加8字节
func (w *Write) Float64(v float64) {
	w.Uint64(math.Float64bits(v))
}

// Complex64 写入一个complex64值（实部+虚部各4字节），按字节序追加8字节
func (w *Write) Complex64(v complex64) {
	w.Float32(float32(real(v)))
	w.Float32(float32(imag(v)))
}

// Complex128 写入一个complex128值（实部+虚部各8字节），按字节序追加16字节
func (w *Write) Complex128(v complex128) {
	w.Float64(float64(real(v)))
	w.Float64(float64(imag(v)))
}

// Bytes 写入一个字节切片（先写长度，再写数据），长度占用4/8字节
func (w *Write) Bytes(v []byte) {
	w.Int(len(v))
	w.Byte = append(w.Byte, v...)
}

// SetByte 直接将字节切片追加到写入缓冲区（不写长度前缀）
func (w *Write) SetByte(v []byte) {
	w.Byte = append(w.Byte, v...)
}
