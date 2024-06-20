package gentity

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fish-tennis/gentity/util"
	"github.com/go-redis/redis/v8"
	"google.golang.org/protobuf/proto"
	"reflect"
)

// map[k]interface{}类型的字段,无法直接反序列化,因为不知道map的value具体是什么类型
//
//	因此提供一个自定义加载接口,由业务层自行实现特殊的反序列化逻辑
//	LoadFromCache处理map[k]interface{}时,会把数据转换成map[k][]byte,传入LoadFromBytesMap
type InterfaceMapLoader interface {
	// bytesMap: map[k][]byte
	LoadFromBytesMap(bytesMap any) error
}

// 加载数据(反序列化)
func LoadData(obj interface{}, sourceData interface{}) error {
	if util.IsNil(sourceData) {
		return nil
	}
	structCache := GetSaveableStruct(reflect.TypeOf(obj))
	if structCache == nil {
		return ErrNotSaveableStruct
	}
	reflectVal := reflect.ValueOf(obj).Elem()
	if structCache.IsSingleField() {
		err := DeserializeField(obj, sourceData, structCache.Field)
		if err != nil {
			GetLogger().Error("DeserializeFieldError:%v fieldName:%v", err.Error(), structCache.Field.Name)
		}
		return err
	} else {
		sourceTyp := reflect.TypeOf(sourceData)
		// 如果是proto,先转换成map
		if sourceTyp.Kind() == reflect.Ptr {
			protoMessage, ok := sourceData.(proto.Message)
			if !ok {
				GetLogger().Error("unsupported type:%v", sourceTyp.Kind())
				return errors.New(fmt.Sprintf("unsupported type:%v", sourceTyp.Kind()))
			}
			// mongodb中读出来是proto.Message格式,转换成map[string]interface{}
			sourceData = ConvertProtoToMap(protoMessage)
			sourceTyp = reflect.TypeOf(sourceData)
		}
		if sourceTyp.Kind() != reflect.Map {
			GetLogger().Error("unsupported type:%v", sourceTyp.Kind())
			return ErrSourceDataType
		}
		sourceVal := reflect.ValueOf(sourceData)
		for _, fieldCache := range structCache.Children {
			sourceFieldVal := sourceVal.MapIndex(reflect.ValueOf(fieldCache.Name))
			if !sourceFieldVal.IsValid() {
				GetLogger().Debug("saveable not exists:%v", fieldCache.Name)
				continue
			}
			val := reflectVal.Field(fieldCache.FieldIndex)
			if !fieldCache.InitNilField(val) {
				GetLogger().Error("child nil %v", fieldCache.Name)
				continue
			}
			fieldInterface := val.Interface()
			childLoadErr := LoadData(fieldInterface, sourceFieldVal.Interface())
			if childLoadErr != nil {
				GetLogger().Error("child load error field:%v", fieldCache.Name)
				return childLoadErr
			}
		}
	}
	return nil
}

// 基础类型的字段赋值(int,float,bool,string,complex)
func DeserializeFieldBaseType(obj any, field reflect.Value, data any, fieldStruct *SaveableField) error {
	dataVal := reflect.ValueOf(data)
	switch fieldStruct.StructField.Type.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch dataVal.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			field.SetInt(dataVal.Int())
			return nil
		default:
			GetLogger().Error("data not a int,fieldName:%v", fieldStruct.Name)
			return errors.New("type not match")
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		switch dataVal.Kind() {
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			field.SetUint(dataVal.Uint())
			return nil
		default:
			GetLogger().Error("data not a uint,fieldName:%v", fieldStruct.Name)
			return errors.New("type not match")
		}

	case reflect.Bool:
		switch dataVal.Kind() {
		case reflect.Bool:
			field.SetBool(dataVal.Bool())
			return nil
		default:
			GetLogger().Error("data not a bool,fieldName:%v", fieldStruct.Name)
			return errors.New("type not match")
		}

	case reflect.Float32, reflect.Float64:
		switch dataVal.Kind() {
		case reflect.Float32, reflect.Float64:
			field.SetFloat(dataVal.Float())
			return nil
		default:
			GetLogger().Error("data not a float,fieldName:%v", fieldStruct.Name)
			return errors.New("type not match")
		}

	case reflect.Complex64, reflect.Complex128:
		switch dataVal.Kind() {
		case reflect.Complex64, reflect.Complex128:
			field.SetComplex(dataVal.Complex())
			return nil
		default:
			GetLogger().Error("data not a complex,fieldName:%v", fieldStruct.Name)
			return errors.New("type not match")
		}

	case reflect.String:
		switch dataVal.Kind() {
		case reflect.String:
			field.SetString(dataVal.String())
			return nil
		default:
			GetLogger().Error("data not a string,fieldName:%v", fieldStruct.Name)
			return errors.New("type not match")
		}

	default:
		return ErrUnsupportedType
	}
}

// protobuf类型的字段赋值
func DeserializeFieldProto(obj any, field reflect.Value, data any, fieldStruct *SaveableField) error {
	if fieldProtoMessage, ok := field.Interface().(proto.Message); ok {
		dataTyp := reflect.TypeOf(data)
		switch dataTyp.Kind() {
		case reflect.Slice, reflect.Array:
			// []byte -> proto.Message
			//if fieldStruct.IsPlain {
			//	return errors.New("plain proto field cant deserialize from bytes")
			//}
			if dataTyp.Elem().Kind() != reflect.Uint8 {
				GetLogger().Error("data not []byte,fieldName:%v", fieldStruct.Name)
				return errors.New(fmt.Sprintf("data not []byte,fieldName:%v", fieldStruct.Name))
			}
			if bytes, ok := data.([]byte); ok {
				if len(bytes) == 0 {
					return nil
				}
				err := proto.Unmarshal(bytes, fieldProtoMessage)
				if err != nil {
					GetLogger().Error("%v proto.Unmarshal err:%v", fieldStruct.Name, err.Error())
					return err
				}
				return nil
			} else {
				GetLogger().Error("data cant convert to []byte,fieldName:%v", fieldStruct.Name)
				return errors.New(fmt.Sprintf("data cant convert to []byte,fieldName:%v", fieldStruct.Name))
			}

		case reflect.Ptr, reflect.Interface:
			// proto.Merge()
			dataVal := reflect.ValueOf(data)
			if dataProtoMessage, ok := dataVal.Interface().(proto.Message); ok {
				if dataProtoMessage.ProtoReflect().Descriptor() != fieldProtoMessage.ProtoReflect().Descriptor() {
					GetLogger().Error("descriptor not match,fieldName:%v", fieldStruct.Name)
					return errors.New(fmt.Sprintf("descriptor not match,fieldName:%v", fieldStruct.Name))
				}
				proto.Merge(fieldProtoMessage, dataProtoMessage)
				return nil
			} else {
				GetLogger().Error("data not a proto.Message,fieldName:%v", fieldStruct.Name)
				return errors.New(fmt.Sprintf("data not a proto.Message,fieldName:%v", fieldStruct.Name))
			}

		default:
			GetLogger().Error("unsupported type,fieldName:%v dataType:%v", fieldStruct.Name, dataTyp.Kind())
			return ErrUnsupportedType
		}
	}
	return ErrUnsupportedType
}

func DeserializeFieldSlice(obj any, field reflect.Value, data any, fieldStruct *SaveableField) error {
	dataTyp := reflect.TypeOf(data)
	switch dataTyp.Kind() {
	case reflect.Slice, reflect.Array:
		fieldElemType := fieldStruct.StructField.Type.Elem()
		dataItemType := dataTyp.Elem()
		switch fieldElemType.Kind() {
		case reflect.Ptr:
			// []proto.Message
			field.Clear()
			dataVal := reflect.ValueOf(data)
			for i := 0; i < dataVal.Len(); i++ {
				dataItem := dataVal.Index(i)
				// fieldElemType需要是一个具体的proto类型
				// dataItemType可能是proto.Message或者是[]byte
				dataItemInterface := ConvertValueToInterface(dataItemType, fieldElemType, dataItem)
				// append
				field.Set(reflect.Append(field, reflect.ValueOf(dataItemInterface)))
				GetLogger().Debug("%v append, fieldElemType:%v dataItemType:%v", fieldStruct.Name, fieldElemType, dataItemType)
			}
			return nil

		default:
			// 基础类型
			if dataTyp.Elem().Kind() != fieldStruct.StructField.Type.Elem().Kind() {
				GetLogger().Error("unsupported type,fieldName:%v dataElemType:%v", fieldStruct.Name, dataTyp.Elem().Kind())
				// 类型不一致,暂时返回错误
				return ErrSliceElemType
			}
			dataVal := reflect.ValueOf(data)
			// 如果是数组,长度必须一致
			if fieldStruct.StructField.Type.Kind() == reflect.Array && dataVal.Len() != field.Len() {
				GetLogger().Error("array len not match,fieldName:%v dataLen:%v", fieldStruct.Name, dataVal.Len())
				return ErrArrayLen
			}
			if fieldStruct.StructField.Type.Kind() == reflect.Slice {
				if field.Cap() < dataVal.Len() {
					field.Grow(dataVal.Len() - field.Cap())
				}
				field.SetLen(dataVal.Len())
			}
			reflect.Copy(field, dataVal)
			return nil
		}

	default:
		GetLogger().Error("unsupported type,fieldName:%v dataType:%v", fieldStruct.Name, dataTyp.Kind())
		return ErrUnsupportedType
	}
}

func DeserializeFieldMap(obj any, field reflect.Value, data any, fieldStruct *SaveableField) error {
	dataTyp := reflect.TypeOf(data)
	if dataTyp.Kind() != reflect.Map {
		GetLogger().Error("unsupported type,fieldName:%v dataType:%v", fieldStruct.Name, dataTyp.Kind())
		return errors.New(fmt.Sprintf("data not a map,fieldName:%v", fieldStruct.Name))
	}
	dataVal := reflect.ValueOf(data)
	dataKeyType := dataTyp.Key()
	dataValType := dataTyp.Elem()
	keyType := fieldStruct.StructField.Type.Key()
	valType := fieldStruct.StructField.Type.Elem()
	sourceIt := dataVal.MapRange()
	for sourceIt.Next() {
		k := ConvertValueToInterface(dataKeyType, keyType, sourceIt.Key())
		v := ConvertValueToInterface(dataValType, valType, sourceIt.Value())
		field.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(v))
	}
	return nil
}

func DeserializeFieldStruct(obj any, field reflect.Value, data any, fieldStruct *SaveableField) error {
	dataTyp := reflect.TypeOf(data)
	if bytes, ok := data.([]byte); ok {
		// []byte -> proto.Message
		if field.CanAddr() {
			fieldAddr := field.Addr()
			if fieldAddr.CanInterface() {
				fieldInterface := fieldAddr.Interface()
				if protoMessage, ok := fieldInterface.(proto.Message); ok {
					err := proto.Unmarshal(bytes, protoMessage)
					if err != nil {
						GetLogger().Error("proto.Unmarshal err:%v,fieldName:%v dataType:%v", err, fieldStruct.Name, dataTyp.Kind())
					}
					return err
				}
			}
		}
	}
	if dataTyp.Kind() != reflect.Struct {
		GetLogger().Error("unsupported type,fieldName:%v dataType:%v", fieldStruct.Name, dataTyp.Kind())
		return errors.New(fmt.Sprintf("data not a struct,fieldName:%v", fieldStruct.Name))
	}
	dataVal := reflect.ValueOf(data)
	for i := 0; i < field.Type().NumField(); i++ {
		sf := field.Type().Field(i)
		if !sf.IsExported() {
			continue
		}
		dataFieldVal := dataVal.FieldByName(sf.Name)
		if !dataFieldVal.IsValid() {
			GetLogger().Debug("dataFieldVal NotValid fieldName:%v.%v", fieldStruct.Name, sf.Name)
			continue
		}
		v := field.Field(i)
		if !v.IsValid() {
			GetLogger().Debug("fieldVal NotValid fieldName:%v.%v", fieldStruct.Name, sf.Name)
			continue
		}
		if !v.CanSet() {
			GetLogger().Debug("fieldVal cant set fieldName:%v.%v", fieldStruct.Name, sf.Name)
			continue
		}
		v.Set(dataFieldVal)
		GetLogger().Debug("fieldVal set fieldName:%v.%v", fieldStruct.Name, sf.Name)
	}
	return nil
}

// 反序列化字段
func DeserializeField(obj any, sourceData any, fieldStruct *SaveableField) error {
	objVal := reflect.ValueOf(obj).Elem()
	// 字段value
	fieldVal := objVal.Field(fieldStruct.FieldIndex)
	// 字段如果是nil,则尝试初始化
	if !fieldStruct.InitNilField(fieldVal) {
		return errors.New("cant init nil field")
	}
	switch fieldStruct.StructField.Type.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.String, reflect.Bool, reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return DeserializeFieldBaseType(obj, fieldVal, sourceData, fieldStruct)

	case reflect.Ptr: // reflect.Interface?
		if _, ok := fieldVal.Interface().(proto.Message); ok {
			return DeserializeFieldProto(obj, fieldVal, sourceData, fieldStruct)
		} else {
			return errors.New(fmt.Sprintf("ptr is not a proto.Message,fieldName:%v", fieldStruct.Name))
		}

	case reflect.Interface:
		return errors.New("not support interface{} field")

	case reflect.Slice, reflect.Array:
		return DeserializeFieldSlice(obj, fieldVal, sourceData, fieldStruct)

	case reflect.Map:
		return DeserializeFieldMap(obj, fieldVal, sourceData, fieldStruct)

	case reflect.Struct:
		return DeserializeFieldStruct(obj, fieldVal, sourceData, fieldStruct)

	default:
		return ErrUnsupportedType
	}

	//sourceTyp := reflect.TypeOf(sourceData)
	//switch sourceTyp.Kind() {
	//case reflect.Slice, reflect.Array:
	//	if !fieldStruct.IsPlain {
	//		// proto反序列化
	//		// []byte -> proto.Message
	//		if sourceTyp.Elem().Kind() == reflect.Uint8 {
	//			if bytes, ok := sourceData.([]byte); ok {
	//				if len(bytes) == 0 {
	//					return nil
	//				}
	//				// []byte -> proto.Message
	//				if protoMessage, ok2 := fieldVal.Interface().(proto.Message); ok2 {
	//					err := proto.Unmarshal(bytes, protoMessage)
	//					if err != nil {
	//						GetLogger().Error("%v proto.Unmarshal err:%v", fieldStruct.Name, err.Error())
	//						return err
	//					}
	//					return nil
	//				}
	//			}
	//		}
	//		// TODO: sourceData如果是[]proto.Message类型
	//	}
	//	// 基础类型的slice
	//	if fieldStruct.StructField.Type.Kind() == reflect.Slice ||
	//		fieldStruct.StructField.Type.Kind() == reflect.Array {
	//		// TODO: fieldStruct如果是[]proto.Message类型
	//		if sourceTyp.Elem().Kind() != fieldStruct.StructField.Type.Elem().Kind() {
	//			// 类型不一致,暂时返回错误
	//			return ErrSliceElemType
	//		}
	//		sourceDataVal := reflect.ValueOf(sourceData)
	//		// 如果是数组,长度必须一致
	//		if fieldStruct.StructField.Type.Kind() == reflect.Array && sourceDataVal.Len() != fieldVal.Len() {
	//			return ErrArrayLen
	//		}
	//		if fieldStruct.StructField.Type.Kind() == reflect.Slice {
	//			if fieldVal.Cap() < sourceDataVal.Len() {
	//				fieldVal.Grow(sourceDataVal.Len() - fieldVal.Cap())
	//			}
	//			fieldVal.SetLen(sourceDataVal.Len())
	//		}
	//		reflect.Copy(fieldVal, sourceDataVal)
	//	}
	//
	//case reflect.Map:
	//	// map[int|string]int|string -> map[int|string]int|string
	//	// map[int|string][]byte -> map[int|string]proto.Message
	//	sourceVal := reflect.ValueOf(sourceData)
	//	sourceKeyType := sourceTyp.Key()
	//	sourceValType := sourceTyp.Elem()
	//	if fieldStruct.StructField.Type.Kind() == reflect.Map {
	//		//fieldVal := reflect.ValueOf(fieldVal)
	//		keyType := fieldStruct.StructField.Type.Key()
	//		valType := fieldStruct.StructField.Type.Elem()
	//		sourceIt := sourceVal.MapRange()
	//		for sourceIt.Next() {
	//			k := ConvertValueToInterface(sourceKeyType, keyType, sourceIt.Key())
	//			v := ConvertValueToInterface(sourceValType, valType, sourceIt.Value())
	//			fieldVal.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(v))
	//		}
	//		return nil
	//	}
	//
	//case reflect.Interface, reflect.Ptr:
	//	sourceVal := reflect.ValueOf(sourceData)
	//	if sourceProtoMessage, ok := sourceVal.Interface().(proto.Message); ok {
	//		if protoMessage, ok2 := fieldVal.Interface().(proto.Message); ok2 {
	//			if sourceProtoMessage.ProtoReflect().Descriptor() == protoMessage.ProtoReflect().Descriptor() {
	//				proto.Merge(protoMessage, sourceProtoMessage)
	//				return nil
	//			}
	//		}
	//	}
	//	// TODO:扩展一个序列化接口
	//	return errors.New(fmt.Sprintf("unsupport type %v", fieldStruct.Name))
	//
	//default:
	//	return errors.New(fmt.Sprintf("unsupport type %v sourceTyp.Kind():%v", fieldStruct.Name, sourceTyp.Kind()))
	//}
	//return nil
}

// 从缓存中恢复数据
//
//	有缓存数据return true,否则return false
//	解析缓存数据错误return error,否则return nil
func LoadFromCache(obj interface{}, kvCache KvCache, cacheKey string) (bool, error) {
	structCache := GetSaveableStruct(reflect.TypeOf(obj))
	if structCache == nil {
		return false, nil
	}
	reflectVal := reflect.ValueOf(obj).Elem()
	if structCache.IsSingleField() {
		cacheType, err := kvCache.Type(cacheKey)
		if err == redis.Nil || cacheType == "" || cacheType == "none" {
			return false, nil
		}
		fieldCache := structCache.Field
		val := reflectVal.Field(fieldCache.FieldIndex)
		if cacheType == "string" {
			if fieldCache.StructField.Type.Kind() == reflect.Ptr || fieldCache.StructField.Type.Kind() == reflect.Interface {
				if !fieldCache.InitNilField(val) {
					GetLogger().Error("nil %v", fieldCache.Name)
					return true, errors.New(fmt.Sprintf("%v nil", fieldCache.Name))
				}
				if !val.CanInterface() {
					return true, errors.New(fmt.Sprintf("%v CanInterface false", fieldCache.Name))
				}
				if protoMessage, ok := val.Interface().(proto.Message); ok {
					// []byte -> proto.Message
					err = kvCache.GetProto(cacheKey, protoMessage)
					if IsRedisError(err) {
						GetLogger().Error("GetProto %v %v err:%v", cacheKey, cacheType, err)
						return true, err
					}
					return true, nil
				}
			} else if fieldCache.StructField.Type.Kind() == reflect.Slice || fieldCache.StructField.Type.Kind() == reflect.Array {
				if !fieldCache.InitNilField(val) {
					GetLogger().Error("nil %v", fieldCache.Name)
					return true, errors.New(fmt.Sprintf("%v nil", fieldCache.Name))
				}
				if !val.CanInterface() {
					return true, errors.New(fmt.Sprintf("%v CanInterface false", fieldCache.Name))
				}
				jsonData, err := kvCache.Get(cacheKey)
				if IsRedisError(err) {
					GetLogger().Error("Get %v %v err:%v", cacheKey, cacheType, err)
					return true, err
				}
				fieldInterface := val.Addr().Interface()
				err = json.Unmarshal([]byte(jsonData), fieldInterface)
				if err != nil {
					GetLogger().Error("json.Unmarshal %v %v err:%v", cacheKey, val.Interface(), err)
					return true, err
				}
				GetLogger().Debug("%v json.Unmarshal", cacheKey)
				return true, nil
			}
			return true, errors.New(fmt.Sprintf("unsupport kind:%v cacheKey:%v cacheType:%v", fieldCache.StructField.Type.Kind(), cacheKey, cacheType))
		} else if cacheType == "hash" {
			if !fieldCache.InitNilField(val) {
				GetLogger().Error("nil %v", fieldCache.Name)
				return true, errors.New(fmt.Sprintf("%v nil", fieldCache.Name))
			}
			if !val.CanInterface() {
				return true, errors.New(fmt.Sprintf("%v CanInterface false", fieldCache.Name))
			}
			if fieldCache.IsInterfaceMap() {
				// hash -> map[k]any
				bytesMap := fieldCache.NewBytesMap()
				err = kvCache.GetMap(cacheKey, bytesMap)
				if err == nil {
					if interfaceMapLoader, ok := obj.(InterfaceMapLoader); ok {
						err = interfaceMapLoader.LoadFromBytesMap(bytesMap)
					}
				}
			} else {
				// hash -> map[k]v
				err = kvCache.GetMap(cacheKey, val.Interface())
			}
			if IsRedisError(err) {
				GetLogger().Error("GetMap %v %v err:%v", cacheKey, cacheType, err)
				return true, err
			}
			return true, nil
		} else {
			GetLogger().Error("%v unsupport cache type:%v", cacheKey, cacheType)
			return true, errors.New(fmt.Sprintf("%v unsupport cache type:%v", cacheKey, cacheType))
		}
	} else {
		for _, fieldCache := range structCache.Children {
			val := reflectVal.Field(fieldCache.FieldIndex)
			if !fieldCache.InitNilField(val) {
				GetLogger().Error("nil %v", fieldCache.Name)
				return true, errors.New(fmt.Sprintf("%v nil", fieldCache.Name))
			}
			fieldInterface := val.Interface()
			hasCache, err := LoadFromCache(fieldInterface, kvCache, cacheKey+"."+fieldCache.Name)
			if !hasCache {
				continue
			}
			if err != nil {
				GetLogger().Error("LoadFromCache %v error:%v", cacheKey, err.Error())
				continue
			}
		}
	}
	return true, nil
}

// 根据缓存数据,修复数据
// 如:服务器crash时,缓存数据没来得及保存到数据库,服务器重启后读取缓存中的数据,保存到数据库,防止数据回档
func FixEntityDataFromCache(entity Entity, db EntityDb, kvCache KvCache, cacheKeyPrefix string, entityKey interface{}) {
	entity.RangeComponent(func(component Component) bool {
		structCache := GetSaveableStruct(reflect.TypeOf(component))
		if structCache == nil {
			return true
		}
		if structCache.IsSingleField() {
			cacheKey := GetEntityComponentCacheKey(cacheKeyPrefix, entityKey, component.GetName())
			hasCache, err := LoadFromCache(component, kvCache, cacheKey)
			if !hasCache {
				return true
			}
			if err != nil {
				GetLogger().Error("LoadFromCache %v error:%v", cacheKey, err.Error())
				return true
			}
			saveData, err := GetComponentSaveData(component)
			if err != nil {
				GetLogger().Error("%v Save %v err %v", entityKey, component.GetName(), err.Error())
				return true
			}
			saveDbErr := db.SaveComponent(entityKey, GetComponentSaveName(component), saveData)
			if saveDbErr != nil {
				GetLogger().Error("%v SaveDb %v err %v", entityKey, GetComponentSaveName(component), saveDbErr.Error())
				return true
			}
			GetLogger().Info("%v -> %v", cacheKey, GetComponentSaveName(component))
			kvCache.Del(cacheKey)
			GetLogger().Info("RemoveCache %v", cacheKey)
		} else {
			reflectVal := reflect.ValueOf(component).Elem()
			for _, fieldCache := range structCache.Children {
				val := reflectVal.Field(fieldCache.FieldIndex)
				if !fieldCache.InitNilField(val) {
					GetLogger().Error("%v nil", fieldCache.Name)
					return true
				}
				fieldInterface := val.Interface()
				cacheKey := GetEntityComponentChildCacheKey(cacheKeyPrefix, entityKey, component.GetName(), fieldCache.Name)
				hasCache, err := LoadFromCache(fieldInterface, kvCache, cacheKey)
				if !hasCache {
					return true
				}
				if err != nil {
					GetLogger().Error("LoadFromCache %v error:%v", cacheKey, err.Error())
					return true
				}
				GetLogger().Debug("%v", fieldInterface)
				saveData, err := GetSaveData(fieldInterface, GetComponentSaveName(component))
				if err != nil {
					GetLogger().Error("%v Save %v.%v err %v", entityKey, component.GetName(), fieldCache.Name, err.Error())
					return true
				}
				GetLogger().Debug("%v", saveData)
				saveDbErr := db.SaveComponentField(entityKey, GetComponentSaveName(component), fieldCache.Name, saveData)
				if saveDbErr != nil {
					GetLogger().Error("%v SaveDb %v.%v err %v", entityKey, GetComponentSaveName(component), fieldCache.Name, saveDbErr.Error())
					return true
				}
				GetLogger().Info("%v -> %v.%v", cacheKey, GetComponentSaveName(component), fieldCache.Name)
				kvCache.Del(cacheKey)
				GetLogger().Info("RemoveCache %v", cacheKey)
			}
		}
		return true
	})
}
