package ast

import (
	"fmt"
	"strconv"
	"time"
)

// AnyToString 将任意基础类型转换为字符串表示。
// 支持 Go 基础数值类型（包含 uint8-uint64、int8-int64、float32/64、complex128）、
// bool、string、time.Duration 以及实现了 fmt.Stringer 接口的类型。
func AnyToString(v any) string {
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
	case uint8:
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

// StringToAny 将 Value 对象中的字符串还原为与 v 相同类型的值。
// 支持类型包括 Go 基础数值类型（含 uint8-uint64、int8-int64、float32/64、
// complex64/128）、bool、string、time.Duration 和 fmt.Stringer。
func StringToAny(valueStrs Value, v any) any {
	switch v := (v).(type) {
	case string:
		return valueStrs.ParseString(v)
	case bool:
		return valueStrs.ParseBool(v)
	case int:
		return valueStrs.ParseInt(v)
	case int8:
		return valueStrs.ParseInt8(v)
	case int16:
		return valueStrs.ParseInt16(v)
	case int32:
		return valueStrs.ParseInt32(v)
	case int64:
		return valueStrs.ParseInt64(v)
	case uint:
		return valueStrs.ParseUint(v)
	case uint8:
		return valueStrs.ParseUint8(v)
	case uint16:
		return valueStrs.ParseUint16(v)
	case uint32:
		return valueStrs.ParseUint32(v)
	case uint64:
		return valueStrs.ParseUint64(v)
	case float32:
		return valueStrs.ParseFloat32(v)
	case float64:
		return valueStrs.ParseFloat64(v)
	case complex64:
		return valueStrs.ParseComplex64(v)
	case complex128:
		return valueStrs.ParseComplex128(v)
	case time.Duration:
		return valueStrs.ParseDuration(v)
	case fmt.Stringer:
		return valueStrs.ParseString(v.String())
	default:
		return valueStrs.ParseString(fmt.Sprint(v))
	}
}
