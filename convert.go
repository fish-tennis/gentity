package gentity

import (
	"errors"
	"fmt"
	"github.com/fish-tennis/gentity/util"
	"google.golang.org/protobuf/proto"
	"reflect"
	"strconv"
	"strings"
)

// reflect.Value -> interface{}
func ConvertValueToInterface(srcType, dstType reflect.Type, srcValue reflect.Value) interface{} {
	switch srcType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return ConvertInterfaceToRealType(dstType, srcValue.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return ConvertInterfaceToRealType(dstType, srcValue.Uint())
	case reflect.Float32, reflect.Float64:
		return ConvertInterfaceToRealType(dstType, srcValue.Float())
	case reflect.Complex64, reflect.Complex128:
		return ConvertInterfaceToRealType(dstType, srcValue.Complex())
	case reflect.String:
		return ConvertInterfaceToRealType(dstType, srcValue.String())
	case reflect.Bool:
		return ConvertInterfaceToRealType(dstType, srcValue.Bool())
	case reflect.Interface, reflect.Ptr:
		return ConvertInterfaceToRealType(dstType, srcValue.Interface())
	case reflect.Slice:
		// dstType是proto.Message, []byte -> proto.Message
		if dstType.Implements(reflect.TypeOf((*proto.Message)(nil)).Elem()) {
			return ConvertInterfaceToRealType(dstType, srcValue.Bytes())
		} else {
			return ConvertInterfaceToRealType(dstType, srcValue.Interface())
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
	case reflect.Complex64:
		return v.(complex64)
	case reflect.Complex128:
		return v.(complex128)
	case reflect.String:
		return v
	case reflect.Bool:
		return v.(bool)
	case reflect.Ptr:
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
		if protoMessage, ok := v.(proto.Message); ok {
			return protoMessage
		}
	case reflect.Slice:
		return v
		//if bytes, ok := v.([]byte); ok {
		//	if typ.Elem().Kind() == reflect.Uint8 {
		//		return v
		//	}
		//	newProto := reflect.New(typ.Elem())
		//	if protoMessage, ok2 := newProto.Interface().(proto.Message); ok2 {
		//		protoErr := proto.Unmarshal(bytes, protoMessage)
		//		if protoErr != nil {
		//			return protoErr
		//		}
		//		return protoMessage
		//	}
		//}
	}
	GetLogger().Error("unsupported type:%v", typ.Kind())
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
		case reflect.Complex64, reflect.Complex128:
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
		if _saveableStructsMap.useLowerName {
			stringMap[strings.ToLower(sf.Name)] = v
		} else {
			stringMap[sf.Name] = v
		}
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
	case reflect.Complex64:
		c, _ := strconv.ParseComplex(v, 64)
		return c
	case reflect.Complex128:
		c, _ := strconv.ParseComplex(v, 128)
		return c
	case reflect.String:
		return v
	case reflect.Bool:
		return v == "true" || v == "1"
	case reflect.Slice:
		// []byte
		if typ.Elem().Kind() == reflect.Uint8 {
			return []byte(v)
		}
	case reflect.Ptr:
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

func convertValueToString(val reflect.Value) (string, error) {
	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.Itoa(int(val.Int())), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(val.Uint(), 10), nil
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(val.Float(), 'f', 2, 64), nil
	case reflect.Complex64, reflect.Complex128:
		return strconv.FormatComplex(val.Complex(), 'f', 2, 128), nil
	case reflect.String:
		return val.String(), nil
	case reflect.Interface:
		if !val.CanInterface() {
			GetLogger().Error("unsupport type:%v", val.Kind())
			return "", errors.New(fmt.Sprintf("unsupport type:%v", val.Kind()))
		}
		return util.ToString(val.Interface())
	default:
		GetLogger().Error("unsupport type:%v", val.Kind())
		return "", errors.New(fmt.Sprintf("unsupport type:%v", val.Kind()))
	}
}

func convertValueToStringOrInterface(val reflect.Value) (interface{}, error) {
	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128,
		reflect.String:
		return convertValueToString(val)
	case reflect.Interface, reflect.Ptr:
		if !util.IsValueNil(val) {
			if !val.CanInterface() {
				GetLogger().Error("unsupport type:%v", val.Kind())
				return nil, errors.New(fmt.Sprintf("unsupport type:%v", val.Kind()))
			}
			i := val.Interface()
			// protobuf格式
			if protoMessage, ok := i.(proto.Message); ok {
				bytes, protoErr := proto.Marshal(protoMessage)
				if protoErr != nil {
					GetLogger().Error("convert proto err:%v", protoErr.Error())
					return nil, protoErr
				}
				return bytes, nil
			}
			// Saveable格式
			if valueSaveable, ok := i.(Saveable); ok {
				valueSaveData, valueSaveErr := GetSaveData(valueSaveable, "")
				if valueSaveErr != nil {
					GetLogger().Error("convert Saveabl err:%v", valueSaveErr.Error())
					return nil, valueSaveErr
				}
				return valueSaveData, nil
			}
			return i, nil
		}
	default:
		GetLogger().Error("unsupport type:%v", val.Kind())
		return nil, errors.New(fmt.Sprintf("unsupport type:%v", val.Kind()))
	}
	GetLogger().Error("unsupport type:%v", val.Kind())
	return nil, errors.New(fmt.Sprintf("unsupport type:%v", val.Kind()))
}
