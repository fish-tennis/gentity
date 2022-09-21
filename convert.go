package gentity

import (
	"google.golang.org/protobuf/proto"
	"reflect"
	"strings"
)

// reflect.Value -> interface{}
func ConvertValueToInterface(srcType, dstType reflect.Type, v reflect.Value) interface{} {
	switch srcType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return ConvertInterfaceToRealType(dstType, v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return ConvertInterfaceToRealType(dstType, v.Uint())
	case reflect.Float32, reflect.Float64:
		return ConvertInterfaceToRealType(dstType, v.Float())
	case reflect.String:
		return ConvertInterfaceToRealType(dstType, v.String())
	case reflect.Interface, reflect.Ptr:
		return ConvertInterfaceToRealType(dstType, v.Interface())
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return ConvertInterfaceToRealType(dstType, v.Bytes())
		} else {
			return ConvertInterfaceToRealType(dstType, v.Interface())
		}
	}
	GetLogger().Error("unsupport type:%v", srcType.Kind())
	return nil
}

// reflect.Value -> int
func ConvertValueToInt(srcType reflect.Type, v reflect.Value) int64 {
	switch srcType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(v.Uint())
	case reflect.Float32, reflect.Float64:
		// NOTE:有精度问题
		return int64(v.Float())
	}
	GetLogger().Error("unsupport type:%v", srcType.Kind())
	return 0
}

// interface{} -> int or string or proto.Message
func ConvertInterfaceToRealType(typ reflect.Type, v interface{}) interface{} {
	switch typ.Kind() {
	case reflect.Int:
		return int(v.(int64))
	case reflect.Int8:
		return int8(v.(int64))
	case reflect.Int16:
		return int16(v.(int64))
	case reflect.Int32:
		return int32(v.(int64))
	case reflect.Int64:
		return v.(int64)
	case reflect.Uint:
		return uint(v.(uint64))
	case reflect.Uint8:
		return uint8(v.(uint64))
	case reflect.Uint16:
		return uint16(v.(uint64))
	case reflect.Uint32:
		return uint32(v.(uint64))
	case reflect.Uint64:
		return v.(uint64)
	case reflect.Float32:
		return v.(float32)
	case reflect.Float64:
		return v.(float64)
	case reflect.String:
		return v
	case reflect.Ptr, reflect.Slice:
		if bytes, ok := v.([]byte); ok {
			newProto := reflect.New(typ.Elem())
			if protoMessage, ok2 := newProto.Interface().(proto.Message); ok2 {
				protoErr := proto.Unmarshal(bytes, protoMessage)
				if protoErr != nil {
					return protoErr
				}
				return protoMessage
			}
		}
	}
	GetLogger().Error("unsupport type:%v", typ.Kind())
	return nil
}

// proto.Message -> map[string]interface{}
func ConvertProtoToMap(protoMessage proto.Message) map[string]interface{} {
	stringMap := make(map[string]interface{})
	typ := reflect.TypeOf(protoMessage).Elem()
	val := reflect.ValueOf(protoMessage).Elem()
	for i := 0; i < typ.NumField(); i++ {
		sf := typ.Field(i)
		if len(sf.Tag) == 0 {
			continue
		}
		var v interface{}
		fieldVal := val.Field(i)
		switch fieldVal.Kind() {
		case reflect.Slice:
			if fieldVal.Type().Elem().Kind() == reflect.Uint8 {
				v = fieldVal.Bytes()
			} else {
				v = fieldVal.Interface()
			}
		case reflect.Interface, reflect.Ptr, reflect.Map:
			v = fieldVal.Interface()
		}
		if v == nil {
			GetLogger().Debug("%v %v nil", sf.Name, fieldVal.Kind())
			continue
		}
		// 兼容mongodb,字段名小写
		stringMap[strings.ToLower(sf.Name)] = v
	}
	return stringMap
}
