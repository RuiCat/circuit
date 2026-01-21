package utils

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// NetList 网表定义
type NetList []string

// FromAnySlice 将 []any 转换为 NetList 类型
// any 只能是基础类型，不考虑结构体的解析
func FromAnySlice(slice []any) NetList {
	if slice == nil {
		return NetList{}
	}
	result := make(NetList, len(slice))
	for i, v := range slice {
		result[i] = anyToString(v)
	}
	return result
}

// anyToString 将任意基础类型转换为字符串
func anyToString(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case bool:
		return strconv.FormatBool(val)
	case int:
		return strconv.FormatInt(int64(val), 10)
	case int8:
		return strconv.FormatInt(int64(val), 10)
	case int16:
		return strconv.FormatInt(int64(val), 10)
	case int32:
		return strconv.FormatInt(int64(val), 10)
	case int64:
		return strconv.FormatInt(val, 10)
	case uint:
		return strconv.FormatUint(uint64(val), 10)
	case uint16:
		return strconv.FormatUint(uint64(val), 10)
	case uint32:
		return strconv.FormatUint(uint64(val), 10)
	case uint64:
		return strconv.FormatUint(val, 10)
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case complex128:
		return strconv.FormatComplex(val, 'f', -1, 128)
	case time.Duration:
		return val.String()
	case fmt.Stringer:
		return val.String()
	default:
		return fmt.Sprint(v)
	}
}

// SeparationPrick 分离字符串前错
func (value NetList) SeparationPrick(i int) (typeName string, id int) {
	nameStr := strings.ToUpper(value[i])
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
func (value NetList) ParseBool(i int, defaultValue bool) bool {
	if i < len(value) {
		if val, err := strconv.ParseBool(value[i]); err == nil {
			return val
		}
	}
	return defaultValue
}

// ParseInt 解析整数
func (value NetList) ParseInt(i int, defaultValue int) int {
	if i < len(value) {
		if val, err := strconv.Atoi(value[i]); err == nil {
			return val
		}
	}
	return defaultValue
}

// ParseInt8 解析8位有符号整数
func (value NetList) ParseInt8(i int, defaultValue int8) int8 {
	if i < len(value) {
		if val, err := strconv.ParseInt(value[i], 10, 8); err == nil {
			return int8(val)
		}
	}
	return defaultValue
}

// ParseInt16 解析16位有符号整数
func (value NetList) ParseInt16(i int, defaultValue int16) int16 {
	if i < len(value) {
		if val, err := strconv.ParseInt(value[i], 10, 16); err == nil {
			return int16(val)
		}
	}
	return defaultValue
}

// ParseInt32 解析32位有符号整数
func (value NetList) ParseInt32(i int, defaultValue int32) int32 {
	if i < len(value) {
		if val, err := strconv.ParseInt(value[i], 10, 32); err == nil {
			return int32(val)
		}
	}
	return defaultValue
}

// ParseInt64 解析64位有符号整数
func (value NetList) ParseInt64(i int, defaultValue int64) int64 {
	if i < len(value) {
		if val, err := strconv.ParseInt(value[i], 10, 64); err == nil {
			return val
		}
	}
	return defaultValue
}

// ParseUint 解析无符号整数
func (value NetList) ParseUint(i int, defaultValue uint) uint {
	if i < len(value) {
		if val, err := strconv.ParseUint(value[i], 10, 0); err == nil {
			return uint(val)
		}
	}
	return defaultValue
}

// ParseUint8 解析8位无符号整数
func (value NetList) ParseUint8(i int, defaultValue uint8) uint8 {
	if i < len(value) {
		if val, err := strconv.ParseUint(value[i], 10, 8); err == nil {
			return uint8(val)
		}
	}
	return defaultValue
}

// ParseUint16 解析16位无符号整数
func (value NetList) ParseUint16(i int, defaultValue uint16) uint16 {
	if i < len(value) {
		if val, err := strconv.ParseUint(value[i], 10, 16); err == nil {
			return uint16(val)
		}
	}
	return defaultValue
}

// ParseUint32 解析32位无符号整数
func (value NetList) ParseUint32(i int, defaultValue uint32) uint32 {
	if i < len(value) {
		if val, err := strconv.ParseUint(value[i], 10, 32); err == nil {
			return uint32(val)
		}
	}
	return defaultValue
}

// ParseUint64 解析64位无符号整数
func (value NetList) ParseUint64(i int, defaultValue uint64) uint64 {
	if i < len(value) {
		if val, err := strconv.ParseUint(value[i], 10, 64); err == nil {
			return val
		}
	}
	return defaultValue
}

// ParseFloat32 解析32位浮点数
func (value NetList) ParseFloat32(i int, defaultValue float32) float32 {
	if i < len(value) {
		if val, err := strconv.ParseFloat(value[i], 32); err == nil {
			return float32(val)
		}
	}
	return defaultValue
}

// ParseFloat64 解析64位浮点数
func (value NetList) ParseFloat64(i int, defaultValue float64) float64 {
	if i < len(value) {
		if val, err := strconv.ParseFloat(value[i], 64); err == nil {
			return val
		}
	}
	return defaultValue
}

// ParseString 安全获取字符串
func (value NetList) ParseString(i int, defaultValue string) string {
	if i < len(value) {
		return value[i]
	}
	return defaultValue
}

// ParseDuration 解析时间间隔
func (value NetList) ParseDuration(i int, defaultValue time.Duration) time.Duration {
	if i < len(value) {
		if val, err := time.ParseDuration(value[i]); err == nil {
			return val
		}
	}
	return defaultValue
}

// ParseComplex128 解析128位复数
func (value NetList) ParseComplex128(i int, defaultValue complex128) complex128 {
	if i < len(value) {
		if val, err := strconv.ParseComplex(value[i], 128); err == nil {
			return val
		}
	}
	return defaultValue
}

// ParseComplex64 解析64位复数
func (value NetList) ParseComplex64(i int, defaultValue complex64) complex64 {
	if i < len(value) {
		if val, err := strconv.ParseComplex(value[i], 64); err == nil {
			return complex64(val)
		}
	}
	return defaultValue
}
