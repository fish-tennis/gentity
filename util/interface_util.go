package util

import "reflect"

// interface{}判断是否为空 不能简单的==nil
func IsNil(i any) bool {
	if i == nil {
		return true
	}
	return IsValueNil(reflect.ValueOf(i))
}

func IsValueNil(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.UnsafePointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}
