package gentity

import (
	"github.com/fish-tennis/gentity/util"
	"google.golang.org/protobuf/proto"
	"reflect"
	"strconv"
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
	case reflect.Bool:
		return ConvertInterfaceToRealType(dstType, v.Bool())
	case reflect.Interface, reflect.Ptr:
		return ConvertInterfaceToRealType(dstType, v.Interface())
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return ConvertInterfaceToRealType(dstType, v.Bytes())
		} else {
			return ConvertInterfaceToRealType(dstType, v.Interface())
		}
	default:
		GetLogger().Error("unsupported type:%v", srcType.Kind())
	}
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
	default:
		GetLogger().Error("unsupported type:%v", srcType.Kind())
	}
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
	case reflect.Bool:
		return v.(bool)
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
	default:
		GetLogger().Error("unsupported type:%v", typ.Kind())
	}
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
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			v = fieldVal.Interface()
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			v = fieldVal.Interface()
		case reflect.Float32, reflect.Float64:
			v = fieldVal.Interface()
		case reflect.String:
			v = fieldVal.Interface()
		case reflect.Bool:
			v = fieldVal.Interface()
		default:
			GetLogger().Error("unsupported type:%v", fieldVal.Kind())
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

func ConvertStringToRealType(typ reflect.Type, v string) interface{} {
	switch typ.Kind() {
	case reflect.Int:
		return util.Atoi(v)
	case reflect.Int8:
		return int8(util.Atoi(v))
	case reflect.Int16:
		return int16(util.Atoi(v))
	case reflect.Int32:
		return int32(util.Atoi(v))
	case reflect.Int64:
		return util.Atoi64(v)
	case reflect.Uint:
		return uint(util.Atou(v))
	case reflect.Uint8:
		return uint8(util.Atou(v))
	case reflect.Uint16:
		return uint16(util.Atou(v))
	case reflect.Uint32:
		return uint32(util.Atou(v))
	case reflect.Uint64:
		return util.Atou(v)
	case reflect.Float32:
		f, _ := strconv.ParseFloat(v, 32)
		return float32(f)
	case reflect.Float64:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	case reflect.String:
		return v
	case reflect.Bool:
		return v == "true" || v == "1"
	case reflect.Interface, reflect.Ptr:
		newProto := reflect.New(typ.Elem())
		if protoMessage, ok := newProto.Interface().(proto.Message); ok {
			protoErr := proto.Unmarshal([]byte(v), protoMessage)
			if protoErr != nil {
				GetLogger().Error("proto err:%v", protoErr.Error())
				return protoErr
			}
			return protoMessage
		}
	default:
		GetLogger().Error("unsupported type:%v", typ.Kind())
	}
	return nil
}
