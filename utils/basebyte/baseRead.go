package basebyte

import (
	"reflect"
)

// BaseRead 基本类型读
func BaseRead(r *Read, v interface{}) error {
	return baseRead(r, reflect.ValueOf(v))
}

// baseRead 基本类型读实现
func baseRead(r *Read, v reflect.Value) error {
	if !v.IsValid() || !v.CanSet() {
		return ErrInvalidData
	}

	switch v.Kind() {
	case reflect.Bool:
		val, err := r.Bool()
		if err != nil {
			return err
		}
		v.SetBool(val)
	case reflect.Int:
		val, err := r.Int()
		if err != nil {
			return err
		}
		v.SetInt(int64(val))
	case reflect.Int8:
		val, err := r.Int8()
		if err != nil {
			return err
		}
		v.SetInt(int64(val))
	case reflect.Int16:
		val, err := r.Int16()
		if err != nil {
			return err
		}
		v.SetInt(int64(val))
	case reflect.Int32:
		val, err := r.Int32()
		if err != nil {
			return err
		}
		v.SetInt(int64(val))
	case reflect.Int64:
		val, err := r.Int64()
		if err != nil {
			return err
		}
		v.SetInt(val)
	case reflect.Uint:
		val, err := r.Uint()
		if err != nil {
			return err
		}
		v.SetUint(uint64(val))
	case reflect.Uint8:
		val, err := r.Uint8()
		if err != nil {
			return err
		}
		v.SetUint(uint64(val))
	case reflect.Uint16:
		val, err := r.Uint16()
		if err != nil {
			return err
		}
		v.SetUint(uint64(val))
	case reflect.Uint32:
		val, err := r.Uint32()
		if err != nil {
			return err
		}
		v.SetUint(uint64(val))
	case reflect.Uint64:
		val, err := r.Uint64()
		if err != nil {
			return err
		}
		v.SetUint(val)
	case reflect.Float32:
		val, err := r.Float32()
		if err != nil {
			return err
		}
		v.SetFloat(float64(val))
	case reflect.Float64:
		val, err := r.Float64()
		if err != nil {
			return err
		}
		v.SetFloat(val)
	case reflect.Complex64:
		val, err := r.Complex64()
		if err != nil {
			return err
		}
		v.SetComplex(complex128(val))
	case reflect.Complex128:
		val, err := r.Complex128()
		if err != nil {
			return err
		}
		v.SetComplex(val)
	case reflect.Ptr:
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		return baseRead(r, v.Elem())
	case reflect.Map:
		t := v.Type()
		if v.IsNil() {
			v.Set(reflect.MakeMap(t))
		}

		count, err := r.Int()
		if err != nil {
			return err
		}

		key := reflect.New(t.Key()).Elem()
		value := reflect.New(t.Elem()).Elem()

		for i := 0; i < count; i++ {
			if err := baseRead(r, key); err != nil {
				return err
			}
			if err := baseRead(r, value); err != nil {
				return err
			}
			v.SetMapIndex(key, value)
		}
	case reflect.String:
		bytes, err := r.Bytes()
		if err != nil {
			return err
		}
		v.SetString(string(bytes))
	case reflect.Array:
		n := v.Len()
		for i := 0; i < n; i++ {
			if err := baseRead(r, v.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Slice:
		n, err := r.Int()
		if err != nil {
			return err
		}
		v.Set(reflect.MakeSlice(v.Type(), n, n))
		for i := 0; i < n; i++ {
			if err := baseRead(r, v.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Interface:
		kindVal, err := r.Uint()
		if err != nil {
			return err
		}
		k := reflect.Kind(kindVal)

		elem, ok := typetoValue(k)
		if !ok {
			return ErrInvalidData
		}

		if err := baseRead(r, elem); err != nil {
			return err
		}
		v.Set(elem)
	case reflect.Struct:
		n := v.NumField()
		for i := 0; i < n; i++ {
			if err := baseRead(r, v.Field(i)); err != nil {
				return err
			}
		}
	default:
		return ErrInvalidData
	}
	return nil
}

// typetoValue 类型到值
func typetoValue(t reflect.Kind) (v reflect.Value, _ bool) {
	switch t {
	case reflect.Bool:
		v = reflect.ValueOf(new(bool)).Elem()
	case reflect.Int:
		v = reflect.ValueOf(new(int)).Elem()
	case reflect.Int8:
		v = reflect.ValueOf(new(int8)).Elem()
	case reflect.Int16:
		v = reflect.ValueOf(new(int16)).Elem()
	case reflect.Int32:
		v = reflect.ValueOf(new(int32)).Elem()
	case reflect.Int64:
		v = reflect.ValueOf(new(int64)).Elem()
	case reflect.Uint:
		v = reflect.ValueOf(new(uint)).Elem()
	case reflect.Uint8:
		v = reflect.ValueOf(new(uint8)).Elem()
	case reflect.Uint16:
		v = reflect.ValueOf(new(uint16)).Elem()
	case reflect.Uint32:
		v = reflect.ValueOf(new(uint32)).Elem()
	case reflect.Uint64:
		v = reflect.ValueOf(new(uint64)).Elem()
	case reflect.Float32:
		v = reflect.ValueOf(new(float32)).Elem()
	case reflect.Float64:
		v = reflect.ValueOf(new(float64)).Elem()
	case reflect.Complex64:
		v = reflect.ValueOf(new(complex64)).Elem()
	case reflect.Complex128:
		v = reflect.ValueOf(new(complex128)).Elem()
	case reflect.String:
		v = reflect.ValueOf(new(string)).Elem()
	case reflect.Interface:
		v = reflect.ValueOf(new(interface{})).Elem()
	default:
		return v, false
	}
	return v, true
}
