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

// map[k]any类型的字段,无法直接反序列化,因为不知道map的value具体是什么类型
//
// 因此提供一个自定义加载接口,由业务层自行实现特殊的反序列化逻辑
// LoadFromCache处理map[k]any时,会把数据转换成map[k][]byte,传入LoadFromBytesMap
// 保存数据时,由于知道具体的value类型,所以无需特殊保存接口
type InterfaceMapLoader interface {
	// bytesMap: map[k][]byte
	LoadFromBytesMap(bytesMap any) error
}

func LoadEntityData(entity Entity, entityData interface{}) error {
	var err error
	entity.RangeComponent(func(component Component) bool {
		objStruct := GetObjSaveableStruct(component)
		if objStruct == nil {
			// 组件可以没有保存字段
			return true
		}
		entityDataVal := reflect.ValueOf(entityData)
		if entityDataVal.Kind() == reflect.Ptr {
			entityDataVal = entityDataVal.Elem()
		}
		if util.IsValueNil(entityDataVal) {
			return true
		}
		dataVal := GetFieldValue(entityDataVal, component.GetName())
		if util.IsValueNil(dataVal) {
			return true
		}
		if !dataVal.CanInterface() {
			GetLogger().Error("LoadEntityData %v %v entityData's field CantInterface", entity.GetId(), component.GetName())
			return false
		}
		err = LoadObjData(component, dataVal.Interface())
		if err != nil {
			GetLogger().Error("LoadEntityData %v %v err:%v", entity.GetId(), component.GetName(), err.Error())
			return false
		}
		return true
	})
	return err
}

func LoadObjData(obj any, sourceData interface{}) error {
	if util.IsNil(sourceData) {
		return nil
	}
	objStruct := GetObjSaveableStruct(obj)
	if objStruct == nil {
		return ErrNotSaveableStruct
	}
	if objStruct.IsSingleField() {
		// InterfaceMap特殊处理
		if objStruct.Field.IsInterfaceMap() {
			// InterfaceMapLoader的特殊加载接口放在obj上(一般是组件上),而不是field上
			if interfaceMapLoader, ok := obj.(InterfaceMapLoader); ok {
				GetLogger().Debug("InterfaceMapLoader %v", objStruct.Field.Name)
				return interfaceMapLoader.LoadFromBytesMap(sourceData)
			}
		}
		saveable, saveableField := objStruct.GetSingleSaveable(obj)
		if saveable == nil {
			GetLogger().Error("LoadObjData %v Err:obj not a saveable", objStruct.Field.Name)
			return ErrNotSaveable
		}
		err := loadField(saveable, sourceData, saveableField)
		if err != nil {
			GetLogger().Error("loadFieldError:%v fieldName:%v", err.Error(), saveableField.Name)
		}
		return err
	} else {
		sourceVal := reflect.ValueOf(sourceData)
		objVal := reflect.ValueOf(obj)
		if objVal.Kind() == reflect.Ptr {
			objVal = objVal.Elem()
		}
		for childIndex, childStruct := range objStruct.Children {
			sourceFieldVal := GetFieldValue(sourceVal, childStruct.Name)
			if !sourceFieldVal.IsValid() {
				GetLogger().Debug("sourceFieldVal not exists:%v", childStruct.Name)
				continue
			}
			// child InterfaceMap特殊处理
			if childStruct.IsInterfaceMap() {
				childObj := objVal.Field(childStruct.FieldIndex).Interface()
				if interfaceMapLoader, ok := childObj.(InterfaceMapLoader); ok {
					GetLogger().Debug("InterfaceMapLoader %v", childStruct.Name)
					childLoadErr := interfaceMapLoader.LoadFromBytesMap(sourceData)
					if childLoadErr != nil {
						GetLogger().Error("LoadObjData error field:%v", childStruct.Name)
						return childLoadErr
					}
				}
			}
			saveable, saveableField := objStruct.GetChildSaveable(obj, childIndex)
			if saveable == nil {
				GetLogger().Error("LoadObjData %v Err:field not a saveable", childStruct.Name)
				return ErrNotSaveable
			}
			childLoadErr := loadField(saveable, sourceFieldVal.Interface(), saveableField)
			if childLoadErr != nil {
				GetLogger().Error("LoadObjData error field:%v", saveableField.Name)
				return childLoadErr
			}
		}
	}
	return nil
}

// 基础类型的字段赋值(int,float,bool,string,complex)
func loadFieldBaseType(obj any, field reflect.Value, data any, fieldStruct *SaveableField) error {
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
func loadFieldProto(obj any, field reflect.Value, data any, fieldStruct *SaveableField) error {
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

func loadFieldSlice(obj any, field reflect.Value, data any, fieldStruct *SaveableField) error {
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

func loadFieldMap(obj any, field reflect.Value, data any, fieldStruct *SaveableField) error {
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

func loadFieldStruct(obj any, field reflect.Value, data any, fieldStruct *SaveableField) error {
	dataTyp := reflect.TypeOf(data)
	if bytes, ok := data.([]byte); ok {
		// []byte -> proto.Message
		if fieldInterface := convertStructToInterface(field); fieldInterface != nil {
			if protoMessage, ok := fieldInterface.(proto.Message); ok {
				err := proto.Unmarshal(bytes, protoMessage)
				if err != nil {
					GetLogger().Error("proto.Unmarshal err:%v,fieldName:%v dataType:%v", err, fieldStruct.Name, dataTyp.Kind())
				}
				return err
			}
			if _, ok := fieldInterface.(Saveable); ok {
				GetLogger().Error("saveableField load err,fieldName:%v dataType:%v", fieldStruct.Name, dataTyp.Kind())
			}
		}
	}
	if dataTyp.Kind() != reflect.Struct {
		//// 特殊结构体的赋值,如MapData[K comparable, V any]
		//if fieldInterface := convertStructToInterface(field); fieldInterface != nil {
		//	if _, ok := fieldInterface.(Saveable); ok {
		//		return LoadData(fieldInterface, data)
		//	}
		//}
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
func loadField(obj any, sourceData any, fieldStruct *SaveableField) error {
	objVal := reflect.ValueOf(obj)
	if objVal.Kind() == reflect.Ptr {
		objVal = objVal.Elem()
	}
	// 字段value
	field := objVal.Field(fieldStruct.FieldIndex)
	// 字段如果是nil,则尝试初始化
	if !fieldStruct.InitNilField(field) {
		return errors.New("cant init nil field")
	}
	switch fieldStruct.StructField.Type.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.String, reflect.Bool, reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return loadFieldBaseType(obj, field, sourceData, fieldStruct)

	case reflect.Ptr: // reflect.Interface?
		fieldInterface := field.Interface()
		if fieldInterface == nil {
			return errors.New(fmt.Sprintf("ptr field cant convert to interface{},fieldName:%v", fieldStruct.Name))
		}
		if _, ok := fieldInterface.(proto.Message); ok {
			return loadFieldProto(obj, field, sourceData, fieldStruct)
		}
		//if _, ok := fieldInterface.(Saveable); ok {
		//	return LoadData(fieldInterface, sourceData)
		//}
		return errors.New(fmt.Sprintf("ptr is not a proto.Message,fieldName:%v", fieldStruct.Name))

	case reflect.Interface:
		return errors.New("not support interface{} field")

	case reflect.Slice, reflect.Array:
		return loadFieldSlice(obj, field, sourceData, fieldStruct)

	case reflect.Map:
		return loadFieldMap(obj, field, sourceData, fieldStruct)

	case reflect.Struct:
		return loadFieldStruct(obj, field, sourceData, fieldStruct)

	default:
		return ErrUnsupportedType
	}
}

//// 返回obj的map字段
//func getMapField(obj Saveable) (any, error) {
//	structCache := GetSaveableStruct(reflect.TypeOf(obj))
//	if structCache == nil {
//		return nil, ErrNotSaveableStruct
//	}
//	if structCache.IsSingleField() {
//		fieldStruct := structCache.Field
//		objVal := reflect.ValueOf(obj)
//		if objVal.Kind() == reflect.Ptr {
//			objVal = objVal.Elem()
//		}
//		// 字段value
//		field := objVal.Field(fieldStruct.FieldIndex)
//		switch fieldStruct.StructField.Type.Kind() {
//		case reflect.Map: // 不支持 *map
//			return field.Interface(), nil
//
//		case reflect.Ptr:
//			if saveableField, ok := field.Interface().(Saveable); ok {
//				return getMapField(saveableField)
//			}
//
//		case reflect.Struct:
//			if fieldInterface := convertStructToInterface(field); fieldInterface != nil {
//				if saveableField, ok := fieldInterface.(Saveable); ok {
//					return getMapField(saveableField)
//				}
//			}
//		}
//	}
//	return nil, ErrUnsupportedType
//}

// 从缓存加载字段
func loadFieldFromCache(obj any, kvCache KvCache, cacheKey string, fieldStruct *SaveableField, parentObj any) (bool, error) {
	cacheType, err := kvCache.Type(cacheKey)
	if err == redis.Nil || cacheType == "" || cacheType == "none" {
		return false, nil
	}
	objVal := reflect.ValueOf(obj)
	if objVal.Kind() == reflect.Ptr {
		objVal = objVal.Elem()
	}
	field := objVal.Field(fieldStruct.FieldIndex)
	// 字段如果是nil,则尝试初始化
	if !fieldStruct.InitNilField(field) {
		return true, errors.New("cant init nil field")
	}
	fieldType := fieldStruct.StructField.Type
	switch cacheType {
	case "string":
		// string类型的缓存,支持明文保存的基础类型,protobuf,作为整体保存的二进制类型
		cacheData, err := kvCache.Get(cacheKey)
		if IsRedisError(err) {
			GetLogger().Error("Get %v %v err:%v", cacheKey, cacheType, err)
			return true, err
		}
		// 把缓存中的值转换成sourceData
		var sourceData any
		switch fieldType.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.String, reflect.Bool, reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
			sourceData = ConvertStringToRealType(fieldType, cacheData)

		case reflect.Slice, reflect.Array:
			// json.Unmarshal的参数需要传入&field
			if field.CanAddr() {
				field = field.Addr()
			}
			fieldInterface := field.Interface()
			if fieldInterface == nil {
				return true, errors.New(fmt.Sprintf("%v convertStructToInterfaceErr", fieldStruct.Name))
			}
			// 用json序列化
			err = json.Unmarshal([]byte(cacheData), fieldInterface)
			if err != nil {
				GetLogger().Error("slice json.Unmarshal %v %v err:%v", cacheKey, field.Interface(), err)
				return true, err
			}
			GetLogger().Debug("load slice %v field:%v", cacheKey, fieldStruct.Name)
			return true, nil

		default:
			// 除了基础类型,其他类型在string缓存中都是[]byte
			sourceData = []byte(cacheData)
		}
		// 转换成[]byte后,就和从数据库加载数据一致了
		return true, loadField(obj, sourceData, fieldStruct)

	case "hash":
		// hash类型的缓存支持map类型
		if fieldStruct.IsInterfaceMap() {
			// 特殊类型的map
			// hash -> map[k]any
			bytesMap := fieldStruct.NewBytesMap()
			err = kvCache.GetMap(cacheKey, bytesMap)
			if err == nil {
				if parentObj != nil {
					if interfaceMapLoader, ok := parentObj.(InterfaceMapLoader); ok {
						err = interfaceMapLoader.LoadFromBytesMap(bytesMap)
					}
				}
			}
			if IsRedisError(err) {
				GetLogger().Error("loadInterfaceMapErr %v %v err:%v", cacheKey, cacheType, err)
				return true, err
			}
			GetLogger().Debug("load InterfaceMap %v field:%v", cacheKey, fieldStruct.Name)
		} else {
			var mapField any
			switch fieldType.Kind() {
			case reflect.Map:
				// 普通map
				mapField = field.Interface()

			//case reflect.Ptr:
			//	// 可能是MapData
			//	fieldInterface := field.Interface()
			//	if saveableField, ok := fieldInterface.(Saveable); ok {
			//		mapField, err = getMapField(saveableField)
			//		if err != nil {
			//			GetLogger().Error("getMapFieldErr %v %v err:%v", cacheKey, cacheType, err)
			//			return true, err
			//		}
			//	}
			//
			//case reflect.Struct:
			//	if fieldInterface := convertStructToInterface(field); fieldInterface != nil {
			//		if saveableField, ok := fieldInterface.(Saveable); ok {
			//			mapField, err = getMapField(saveableField)
			//			if err != nil {
			//				GetLogger().Error("getMapFieldErr %v %v err:%v", cacheKey, cacheType, err)
			//				return true, err
			//			}
			//		}
			//	}

			default:
				GetLogger().Error("%v unsupport cache type:%v", cacheKey, cacheType)
				return true, errors.New(fmt.Sprintf("%v unsupport cache type:%v", cacheKey, cacheType))
			}
			if mapField == nil {
				GetLogger().Error("%v mapFieldNil cache type:%v", cacheKey, cacheType)
				return true, errors.New(fmt.Sprintf("%v mapFieldNil cache type:%v", cacheKey, cacheType))
			}
			// hash -> map[k]v
			err = kvCache.GetMap(cacheKey, mapField)
			if IsRedisError(err) {
				GetLogger().Error("GetMap %v %v err:%v", cacheKey, cacheType, err)
				return true, err
			}
			GetLogger().Debug("load map %v field:%v", cacheKey, fieldStruct.Name)
		}
		return true, nil

	default:
		GetLogger().Error("%v unsupport cache type:%v", cacheKey, cacheType)
		return true, errors.New(fmt.Sprintf("%v unsupport cache type:%v", cacheKey, cacheType))
	}
}

// 从缓存中恢复数据
//
//	有缓存数据return true,否则return false
//	解析缓存数据错误return error,否则return nil
func LoadFromCache(obj interface{}, kvCache KvCache, cacheKey string, parentObj any) (bool, error) {
	objStruct := GetObjSaveableStruct(obj)
	if objStruct == nil {
		return false, ErrNotSaveableStruct
	}
	if objStruct.IsSingleField() {
		saveable, saveableField := objStruct.GetSingleSaveable(obj)
		if saveable == nil {
			GetLogger().Error("LoadFromCache %v err", cacheKey)
			return false, ErrUnsupportedType
		}
		return loadFieldFromCache(saveable, kvCache, cacheKey, saveableField, parentObj)
	} else {
		hasData := false
		objVal := reflect.ValueOf(obj)
		if objVal.Kind() == reflect.Ptr {
			objVal = objVal.Elem()
		}
		for childIndex, childStruct := range objStruct.Children {
			saveable, saveableField := objStruct.GetChildSaveable(obj, childIndex)
			if saveable == nil {
				GetLogger().Error("nil %v", childStruct.Name)
				return true, errors.New(fmt.Sprintf("%v nil", childStruct.Name))
			}
			hasCache, err := loadFieldFromCache(saveable, kvCache, cacheKey+"."+childStruct.Name, saveableField, parentObj)
			if !hasCache {
				continue
			}
			if err != nil {
				GetLogger().Error("LoadFromCache child %v error:%v", cacheKey, err.Error())
				continue
			}
			hasData = true
		}
		return hasData, nil
	}
}

// 根据缓存数据,修复数据
// 如:服务器crash时,缓存数据没来得及保存到数据库,服务器重启后读取缓存中的数据,保存到数据库,防止数据回档
func FixEntityDataFromCache(entity Entity, db EntityDb, kvCache KvCache, cacheKeyPrefix string, entityKey interface{}) {
	entity.RangeComponent(func(component Component) bool {
		objStruct := GetObjSaveableStruct(component)
		if objStruct == nil {
			// 组件可以没有保存字段
			return true
		}
		if objStruct.IsSingleField() {
			cacheKey := GetEntityComponentCacheKey(cacheKeyPrefix, entityKey, component.GetName())
			saveable, saveableField := objStruct.GetSingleSaveable(component)
			if saveable == nil {
				GetLogger().Error("%v FixEntityDataFromCache %v Err:obj not a saveable", entityKey, objStruct.Field.Name)
				return true
			}
			hasCache, err := LoadFromCache(saveable, kvCache, cacheKey, component)
			if !hasCache {
				return true
			}
			if err != nil {
				GetLogger().Error("LoadFromCache %v error:%v", cacheKey, err.Error())
				return true
			}
			saveData, err := getSaveDataOfSaveable(saveable, saveableField, GetComponentSaveName(component))
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
			objVal := reflect.ValueOf(component)
			if objVal.Kind() == reflect.Ptr {
				objVal = objVal.Elem()
			}
			for childIndex, childStruct := range objStruct.Children {
				saveable, saveableField := objStruct.GetChildSaveable(component, childIndex)
				if saveable == nil {
					GetLogger().Error("%v FixEntityDataFromCache %v.%v Err:field not a saveable", entityKey, component.GetName(), childStruct.Name)
					return true
				}
				var parentObj any
				if childStruct.IsInterfaceMap() {
					parentObj = objVal.Field(childStruct.FieldIndex).Interface()
				}
				cacheKey := GetEntityComponentChildCacheKey(cacheKeyPrefix, entityKey, component.GetName(), childStruct.Name)
				hasCache, err := LoadFromCache(saveable, kvCache, cacheKey, parentObj)
				if !hasCache {
					return true
				}
				if err != nil {
					GetLogger().Error("LoadFromCache %v error:%v", cacheKey, err.Error())
					return true
				}
				//GetLogger().Debug("%v", fieldInterface)
				saveData, err := getSaveDataOfSaveable(saveable, saveableField, GetComponentSaveName(component))
				if err != nil {
					GetLogger().Error("%v Save %v.%v err %v", entityKey, component.GetName(), childStruct.Name, err.Error())
					return true
				}
				//GetLogger().Debug("%v", saveData)
				saveDbErr := db.SaveComponentField(entityKey, GetComponentSaveName(component), childStruct.Name, saveData)
				if saveDbErr != nil {
					GetLogger().Error("%v SaveDb %v.%v err %v", entityKey, GetComponentSaveName(component), childStruct.Name, saveDbErr.Error())
					return true
				}
				GetLogger().Info("%v -> %v.%v", cacheKey, GetComponentSaveName(component), childStruct.Name)
				kvCache.Del(cacheKey)
				GetLogger().Info("RemoveCacheAfterFix %v", cacheKey)
			}
		}
		return true
	})
}
