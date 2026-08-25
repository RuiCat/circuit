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

// SeparationPrick 从 Value 字符串中分离类型前缀和数字 ID。
// 例如 "R1" → typeName="R", id=1; "CAPACITOR3" → typeName="CAPACITOR", id=3。
// 若字符串不含数字后缀，则 id=0，typeName 为整个字符串的大写形式。
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

// ParseBool 将 Value 字符串解析为 bool 类型。
// 成功时返回解析结果，失败时返回 defaultValue。
func (value Value) ParseBool(defaultValue bool) bool {
	if val, err := strconv.ParseBool(value.Value); err == nil {
		return val
	}
	return defaultValue
}

// ParseInt 将 Value 字符串解析为 int 类型整数。
// 成功时返回解析结果，失败时返回 defaultValue。
func (value Value) ParseInt(defaultValue int) int {
	if val, err := strconv.Atoi(value.Value); err == nil {
		return val
	}
	return defaultValue
}

// ParseInt8 将 Value 字符串解析为 int8 类型。
// 成功时返回解析结果，失败时返回 defaultValue。
func (value Value) ParseInt8(defaultValue int8) int8 {
	if val, err := strconv.ParseInt(value.Value, 10, 8); err == nil {
		return int8(val)
	}
	return defaultValue
}

// ParseInt16 将 Value 字符串解析为 int16 类型。
// 成功时返回解析结果，失败时返回 defaultValue。
func (value Value) ParseInt16(defaultValue int16) int16 {
	if val, err := strconv.ParseInt(value.Value, 10, 16); err == nil {
		return int16(val)
	}
	return defaultValue
}

// ParseInt32 将 Value 字符串解析为 int32 类型。
// 成功时返回解析结果，失败时返回 defaultValue。
func (value Value) ParseInt32(defaultValue int32) int32 {
	if val, err := strconv.ParseInt(value.Value, 10, 32); err == nil {
		return int32(val)
	}
	return defaultValue
}

// ParseInt64 将 Value 字符串解析为 int64 类型。
// 成功时返回解析结果，失败时返回 defaultValue。
func (value Value) ParseInt64(defaultValue int64) int64 {
	if val, err := strconv.ParseInt(value.Value, 10, 64); err == nil {
		return val
	}
	return defaultValue
}

// ParseUint 将 Value 字符串解析为 uint 类型。
// 成功时返回解析结果，失败时返回 defaultValue。
func (value Value) ParseUint(defaultValue uint) uint {
	if val, err := strconv.ParseUint(value.Value, 10, 0); err == nil {
		return uint(val)
	}
	return defaultValue
}

// ParseUint8 将 Value 字符串解析为 uint8 类型。
// 成功时返回解析结果，失败时返回 defaultValue。
func (value Value) ParseUint8(defaultValue uint8) uint8 {
	if val, err := strconv.ParseUint(value.Value, 10, 8); err == nil {
		return uint8(val)
	}
	return defaultValue
}

// ParseUint16 将 Value 字符串解析为 uint16 类型。
// 成功时返回解析结果，失败时返回 defaultValue。
func (value Value) ParseUint16(defaultValue uint16) uint16 {
	if val, err := strconv.ParseUint(value.Value, 10, 16); err == nil {
		return uint16(val)
	}
	return defaultValue
}

// ParseUint32 将 Value 字符串解析为 uint32 类型。
// 成功时返回解析结果，失败时返回 defaultValue。
func (value Value) ParseUint32(defaultValue uint32) uint32 {
	if val, err := strconv.ParseUint(value.Value, 10, 32); err == nil {
		return uint32(val)
	}
	return defaultValue
}

// ParseUint64 将 Value 字符串解析为 uint64 类型。
// 成功时返回解析结果，失败时返回 defaultValue。
func (value Value) ParseUint64(defaultValue uint64) uint64 {
	if val, err := strconv.ParseUint(value.Value, 10, 64); err == nil {
		return val
	}
	return defaultValue
}

// ParseFloat32 将 Value 字符串解析为 float32 类型。
// 成功时返回解析结果，失败时返回 defaultValue。
func (value Value) ParseFloat32(defaultValue float32) float32 {
	if val, err := strconv.ParseFloat(value.Value, 32); err == nil {
		return float32(val)
	}
	return defaultValue
}

// ParseFloat64 将 Value 字符串解析为 float64 类型。
// 成功时返回解析结果，失败时返回 defaultValue。
func (value Value) ParseFloat64(defaultValue float64) float64 {
	if val, err := strconv.ParseFloat(value.Value, 64); err == nil {
		return val
	}
	return defaultValue
}

// ParseString 安全获取字符串值。若 Value 为空字符串则返回 str，否则返回原始字符串值。
// 若 Value 为双引号包裹的字符串字面量（如 "642C0916"），剥离首尾引号返回内容。
// 成功时返回解析结果，失败时返回 defaultValue。
func (value Value) ParseString(str string) string {
	if value.Value != "" {
		v := value.Value
		if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
			return v[1 : len(v)-1]
		}
		return v
	}
	return str
}

// ParseDuration 将 Value 字符串解析为 time.Duration 类型。
// 成功时返回解析结果，失败时返回 defaultValue。
func (value Value) ParseDuration(defaultValue time.Duration) time.Duration {
	if val, err := time.ParseDuration(value.Value); err == nil {
		return val
	}
	return defaultValue
}

// ParseComplex128 将 Value 字符串解析为 complex128 类型。
// 成功时返回解析结果，失败时返回 defaultValue。
func (value Value) ParseComplex128(defaultValue complex128) complex128 {
	if val, err := strconv.ParseComplex(value.Value, 128); err == nil {
		return val
	}
	return defaultValue
}

// ParseComplex64 将 Value 字符串解析为 complex64 类型。
// 成功时返回解析结果，失败时返回 defaultValue。
func (value Value) ParseComplex64(defaultValue complex64) complex64 {
	if val, err := strconv.ParseComplex(value.Value, 64); err == nil {
		return complex64(val)
	}
	return defaultValue
}
