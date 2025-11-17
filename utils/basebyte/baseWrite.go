package basebyte

import (
	"reflect"
)

// BaseWrite 基础类型写
func BaseWrite(w *Write, v interface{}) error {
	return baseWrite(w, reflect.ValueOf(v))
}

// baseWrite 基础类型写实现
func baseWrite(w *Write, v reflect.Value) error {
	if !v.IsValid() {
		return ErrInvalidData
	}

	switch v.Kind() {
	case reflect.Bool:
		w.Bool(v.Bool())
	case reflect.Int:
		w.Int(int(v.Int()))
	case reflect.Int8:
		w.Int8(int8(v.Int()))
	case reflect.Int16:
		w.Int16(int16(v.Int()))
	case reflect.Int32:
		w.Int32(int32(v.Int()))
	case reflect.Int64:
		w.Int64(v.Int())
	case reflect.Uint:
		w.Uint(uint(v.Uint()))
	case reflect.Uint8:
		w.Uint8(uint8(v.Uint()))
	case reflect.Uint16:
		w.Uint16(uint16(v.Uint()))
	case reflect.Uint32:
		w.Uint32(uint32(v.Uint()))
	case reflect.Uint64:
		w.Uint64(v.Uint())
	case reflect.Float32:
		w.Float32(float32(v.Float()))
	case reflect.Float64:
		w.Float64(v.Float())
	case reflect.Complex64:
		w.Complex64(complex64(v.Complex()))
	case reflect.Complex128:
		w.Complex128(v.Complex())
	case reflect.String:
		w.Bytes([]byte(v.String()))
	case reflect.Ptr:
		return baseWrite(w, v.Elem())
	case reflect.Map:
		w.Int(v.Len())
		iter := v.MapRange()
		for iter.Next() {
			if err := baseWrite(w, iter.Key()); err != nil {
				return err
			}
			if err := baseWrite(w, iter.Value()); err != nil {
				return err
			}
		}
	case reflect.Array, reflect.Slice:
		count := v.Len()
		w.Int(count)
		for i := 0; i < count; i++ {
			if err := baseWrite(w, v.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Interface:
		e := v.Elem()
		if !e.IsValid() {
			return ErrInvalidData
		}
		w.Uint(uint(e.Kind()))
		if err := baseWrite(w, e); err != nil {
			return err
		}
	case reflect.Struct:
		n := v.NumField()
		for i := 0; i < n; i++ {
			if err := baseWrite(w, v.Field(i)); err != nil {
				return err
			}
		}
	default:
		return ErrInvalidData
	}
	return nil
}
