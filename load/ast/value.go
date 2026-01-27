package ast

import (
	"strconv"
	"strings"
	"time"
)

// Value 表示一个值，可以是数字、变量名或表达式
type Value struct {
	Value string // 原始值
	IsVar bool   // 是否为变量
	Line  int    // 行号
}

// SeparationPrick 分离字符串前错
func (value Value) SeparationPrick() (typeName string, id int) {
	nameStr := strings.ToUpper(value.Value)
	for i, char := range nameStr {
		if char >= '0' && char <= '9' {
			typeName = nameStr[:i]
			id, _ = strconv.Atoi(nameStr[i:])
			break
		}
	}
	if typeName == "" {
		typeName = nameStr
	}
	return typeName, id
}

// ParseBool 解析布尔值
func (value Value) ParseBool(defaultValue bool) bool {
	if val, err := strconv.ParseBool(value.Value); err == nil {
		return val
	}
	return defaultValue
}

// ParseInt 解析整数
func (value Value) ParseInt(defaultValue int) int {
	if val, err := strconv.Atoi(value.Value); err == nil {
		return val
	}
	return defaultValue
}

// ParseInt8 解析8位有符号整数
func (value Value) ParseInt8(defaultValue int8) int8 {
	if val, err := strconv.ParseInt(value.Value, 10, 8); err == nil {
		return int8(val)
	}
	return defaultValue
}

// ParseInt16 解析16位有符号整数
func (value Value) ParseInt16(defaultValue int16) int16 {
	if val, err := strconv.ParseInt(value.Value, 10, 16); err == nil {
		return int16(val)
	}
	return defaultValue
}

// ParseInt32 解析32位有符号整数
func (value Value) ParseInt32(defaultValue int32) int32 {
	if val, err := strconv.ParseInt(value.Value, 10, 32); err == nil {
		return int32(val)
	}
	return defaultValue
}

// ParseInt64 解析64位有符号整数
func (value Value) ParseInt64(defaultValue int64) int64 {
	if val, err := strconv.ParseInt(value.Value, 10, 64); err == nil {
		return val
	}
	return defaultValue
}

// ParseUint 解析无符号整数
func (value Value) ParseUint(defaultValue uint) uint {
	if val, err := strconv.ParseUint(value.Value, 10, 0); err == nil {
		return uint(val)
	}
	return defaultValue
}

// ParseUint8 解析8位无符号整数
func (value Value) ParseUint8(defaultValue uint8) uint8 {
	if val, err := strconv.ParseUint(value.Value, 10, 8); err == nil {
		return uint8(val)
	}
	return defaultValue
}

// ParseUint16 解析16位无符号整数
func (value Value) ParseUint16(defaultValue uint16) uint16 {
	if val, err := strconv.ParseUint(value.Value, 10, 16); err == nil {
		return uint16(val)
	}
	return defaultValue
}

// ParseUint32 解析32位无符号整数
func (value Value) ParseUint32(defaultValue uint32) uint32 {
	if val, err := strconv.ParseUint(value.Value, 10, 32); err == nil {
		return uint32(val)
	}
	return defaultValue
}

// ParseUint64 解析64位无符号整数
func (value Value) ParseUint64(defaultValue uint64) uint64 {
	if val, err := strconv.ParseUint(value.Value, 10, 64); err == nil {
		return val
	}
	return defaultValue
}

// ParseFloat32 解析32位浮点数
func (value Value) ParseFloat32(defaultValue float32) float32 {
	if val, err := strconv.ParseFloat(value.Value, 32); err == nil {
		return float32(val)
	}
	return defaultValue
}

// ParseFloat64 解析64位浮点数
func (value Value) ParseFloat64(defaultValue float64) float64 {
	if val, err := strconv.ParseFloat(value.Value, 64); err == nil {
		return val
	}
	return defaultValue
}

// ParseString 安全获取字符串
func (value Value) ParseString(str string) string {
	if value.Value != "" {
		return value.Value
	}
	return str
}

// ParseDuration 解析时间间隔
func (value Value) ParseDuration(defaultValue time.Duration) time.Duration {
	if val, err := time.ParseDuration(value.Value); err == nil {
		return val
	}
	return defaultValue
}

// ParseComplex128 解析128位复数
func (value Value) ParseComplex128(defaultValue complex128) complex128 {
	if val, err := strconv.ParseComplex(value.Value, 128); err == nil {
		return val
	}
	return defaultValue
}

// ParseComplex64 解析64位复数
func (value Value) ParseComplex64(defaultValue complex64) complex64 {
	if val, err := strconv.ParseComplex(value.Value, 64); err == nil {
		return complex64(val)
	}
	return defaultValue
}
