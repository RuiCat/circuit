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
}

// checkBounds 检查边界
func (r *Read) checkBounds(required int) error {
	if r.Offset < 0 || r.Offset+required > len(r.Byte) {
		return ErrOutOfBounds
	}
	return nil
}

// Bool 逻辑型
func (r *Read) Bool() (v bool, err error) {
	if err = r.checkBounds(1); err != nil {
		return false, err
	}
	v = r.Byte[r.Offset] != 0
	r.Offset++
	return v, nil
}

// Int8 单字节整数
func (r *Read) Int8() (v int8, err error) {
	if err = r.checkBounds(1); err != nil {
		return 0, err
	}
	v = int8(r.Byte[r.Offset])
	r.Offset++
	return v, nil
}

// Uint8  单字节正整数
func (r *Read) Uint8() (v uint8, err error) {
	if err = r.checkBounds(1); err != nil {
		return 0, err
	}
	v = r.Byte[r.Offset]
	r.Offset++
	return v, nil
}

// Int16 双字节整数
func (r *Read) Int16() (v int16, err error) {
	if err = r.checkBounds(2); err != nil {
		return 0, err
	}
	v = int16(r.Order.Uint16(r.Byte[r.Offset:]))
	r.Offset += 2
	return v, nil
}

// Uint16 双字节正整数
func (r *Read) Uint16() (v uint16, err error) {
	if err = r.checkBounds(2); err != nil {
		return 0, err
	}
	v = r.Order.Uint16(r.Byte[r.Offset:])
	r.Offset += 2
	return v, nil
}

// Int32 四字节整数
func (r *Read) Int32() (v int32, err error) {
	if err = r.checkBounds(4); err != nil {
		return 0, err
	}
	v = int32(r.Order.Uint32(r.Byte[r.Offset:]))
	r.Offset += 4
	return v, nil
}

// Uint32 四字节正整数
func (r *Read) Uint32() (v uint32, err error) {
	if err = r.checkBounds(4); err != nil {
		return 0, err
	}
	v = r.Order.Uint32(r.Byte[r.Offset:])
	r.Offset += 4
	return v, nil
}

// Int64 八字节整数
func (r *Read) Int64() (v int64, err error) {
	if err = r.checkBounds(8); err != nil {
		return 0, err
	}
	v = int64(r.Order.Uint64(r.Byte[r.Offset:]))
	r.Offset += 8
	return v, nil
}

// Uint64 八字节正整数
func (r *Read) Uint64() (v uint64, err error) {
	if err = r.checkBounds(8); err != nil {
		return 0, err
	}
	v = r.Order.Uint64(r.Byte[r.Offset:])
	r.Offset += 8
	return v, nil
}

// Int 平台相关整数
func (r *Read) Int() (v int, err error) {
	if bit64 {
		v64, err := r.Int64()
		if err != nil {
			return 0, err
		}
		v = int(v64)
	} else {
		v32, err := r.Int32()
		if err != nil {
			return 0, err
		}
		v = int(v32)
	}
	return v, nil
}

// Uint 平台相关正整数
func (r *Read) Uint() (v uint, err error) {
	if bit64 {
		v64, err := r.Uint64()
		if err != nil {
			return 0, err
		}
		v = uint(v64)
	} else {
		v32, err := r.Uint32()
		if err != nil {
			return 0, err
		}
		v = uint(v32)
	}
	return v, nil
}

// Float32 浮点数
func (r *Read) Float32() (v float32, err error) {
	bits, err := r.Uint32()
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(bits), nil
}

// Float64 浮点数
func (r *Read) Float64() (v float64, err error) {
	bits, err := r.Uint64()
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(bits), nil
}

// Complex64 复数
func (r *Read) Complex64() (v complex64, err error) {
	real, err := r.Float32()
	if err != nil {
		return 0, err
	}
	imag, err := r.Float32()
	if err != nil {
		return 0, err
	}
	return complex(real, imag), nil
}

// Complex128 复数
func (r *Read) Complex128() (v complex128, err error) {
	real, err := r.Float64()
	if err != nil {
		return 0, err
	}
	imag, err := r.Float64()
	if err != nil {
		return 0, err
	}
	return complex(real, imag), nil
}

// Bytes 字节集
func (r *Read) Bytes() (v []byte, err error) {
	length, err := r.Int()
	if err != nil {
		return nil, err
	}
	if err = r.checkBounds(length); err != nil {
		return nil, err
	}
	v = make([]byte, length)
	copy(v, r.Byte[r.Offset:r.Offset+length])
	r.Offset += length
	return v, nil
}

// GetByte 得到字节集
func (r *Read) GetByte(i int) (v []byte, err error) {
	if err = r.checkBounds(i); err != nil {
		return nil, err
	}
	v = make([]byte, i)
	copy(v, r.Byte[r.Offset:r.Offset+i])
	r.Offset += i
	return v, nil
}
