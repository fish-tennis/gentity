package gentity

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fish-tennis/gentity/util"
	"google.golang.org/protobuf/proto"
	"reflect"
	"strings"
)

// 获取组件的保存数据
func GetComponentSaveData(component Component) (interface{}, error) {
	return GetSaveData(component, GetComponentSaveName(component))
}

func GetComponentSaveName(component Component) string {
	if _saveableStructsMap.useLowerName {
		return strings.ToLower(component.GetName())
	}
	return component.GetName()
}

func SaveObjectChangedDataToCache(kvCache KvCache, parentCacheKey string, obj any) {
	objStruct := GetObjSaveableStruct(obj)
	if objStruct == nil {
		return
	}
	if objStruct.IsSingleField() {
		cacheKey := parentCacheKey
		fieldObj, saveableField := objStruct.GetSingleSaveable(obj)
		if fieldObj == nil {
			GetLogger().Error("cache %v err", cacheKey)
			return
		}
		//depth := 0
		//fieldObj, _ := GetSingleSaveableField(obj, objStruct.Field, &depth)
		//if fieldObj == nil {
		//	GetLogger().Error("cache %v err depth:%v", cacheKey, depth)
		//	return
		//}
		//GetLogger().Debug("GetSingleSaveableField %v fieldName:%v", cacheKey, objStruct.Field.Name)
		SaveChangedDataToCache(kvCache, fieldObj, cacheKey, saveableField)
	} else {
		objVal := reflect.ValueOf(obj)
		if objVal.Kind() == reflect.Ptr {
			objVal = objVal.Elem()
		}
		for childIndex, childStruct := range objStruct.Children {
			// 子对象用childStruct.Name拼接
			cacheKey := fmt.Sprintf("%v.%v", parentCacheKey, childStruct.Name)
			fieldVal := objVal.Field(childStruct.FieldIndex)
			if util.IsValueNil(fieldVal) {
				_, err := kvCache.Del(cacheKey)
				if IsRedisError(err) {
					GetLogger().Error("cache child err cacheKey:%v fieldName:%v err:%v", cacheKey, childStruct.Name, err.Error())
				}
				continue
			}
			fieldInterface, saveableField := objStruct.GetChildSaveable(obj, childIndex)
			//var fieldInterface any
			//if fieldVal.Kind() == reflect.Struct {
			//	fieldInterface = convertStructToInterface(fieldVal)
			//} else {
			//	fieldInterface = fieldVal.Interface()
			//}
			if fieldInterface == nil {
				GetLogger().Error("cache child err cacheKey:%v", cacheKey)
				continue
			}
			SaveChangedDataToCache(kvCache, fieldInterface, cacheKey, saveableField)
		}
	}
}

// 把组件的修改数据保存到缓存
func SaveComponentChangedDataToCache(kvCache KvCache, cacheKeyPrefix string, entityKey interface{}, component Component) {
	// NOTE: 第一层字段用的组件名,并没有用objStruct.Field.Name
	cacheKey := GetEntityComponentCacheKey(cacheKeyPrefix, entityKey, component.GetName())
	SaveObjectChangedDataToCache(kvCache, cacheKey, component)
}

func saveDirtyMark(kvCache KvCache, obj interface{}, cacheKeyName string, fieldCache *SaveableField) {
	// 缓存数据作为一个整体的
	if dirtyMark, ok := obj.(DirtyMark); ok {
		if !dirtyMark.IsDirty() {
			return
		}
		reflectVal := reflect.ValueOf(obj)
		if reflectVal.Kind() == reflect.Ptr {
			reflectVal = reflectVal.Elem()
		}
		val := reflectVal.Field(fieldCache.FieldIndex)
		if util.IsValueNil(val) {
			_, err := kvCache.Del(cacheKeyName)
			if IsRedisError(err) {
				GetLogger().Error("%v cache err:%v", cacheKeyName, err.Error())
				return
			}
		} else {
			SaveValueToCache(kvCache, cacheKeyName, val)
		}
		dirtyMark.ResetDirty()
		GetLogger().Debug("SaveCache %v", cacheKeyName)
	}
}

func saveMapDirtyMark(kvCache KvCache, obj interface{}, cacheKeyName string, fieldCache *SaveableField) {
	// map格式的
	if dirtyMark, ok := obj.(MapDirtyMark); ok {
		if !dirtyMark.IsDirty() {
			return
		}
		reflectVal := reflect.ValueOf(obj)
		if reflectVal.Kind() == reflect.Ptr {
			reflectVal = reflectVal.Elem()
		}
		val := reflectVal.Field(fieldCache.FieldIndex)
		if util.IsValueNil(val) {
			_, err := kvCache.Del(cacheKeyName)
			if IsRedisError(err) {
				GetLogger().Error("%v cache err:%v", cacheKeyName, err.Error())
				return
			}
		} else {
			SaveMapValueToCache(kvCache, cacheKeyName, val, dirtyMark)
		}
		dirtyMark.ResetDirty()
		GetLogger().Debug("SaveCache %v", cacheKeyName)
	}
}

// 把修改数据保存到缓存
func SaveChangedDataToCache(kvCache KvCache, obj interface{}, cacheKeyName string, saveableField *SaveableField) {
	if saveableField == nil {
		return
	}
	// 缓存数据作为一个整体的
	if _, ok := obj.(DirtyMark); ok {
		saveDirtyMark(kvCache, obj, cacheKeyName, saveableField)
		return
	}
	// map格式的
	if _, ok := obj.(MapDirtyMark); ok {
		saveMapDirtyMark(kvCache, obj, cacheKeyName, saveableField)
		return
	}
}

// 保存单个字段到redis
func SaveValueToCache(kvCache KvCache, cacheKeyName string, val reflect.Value) {
	switch val.Kind() {
	case reflect.Ptr, reflect.Interface:
		cacheData := val.Interface()
		switch realData := cacheData.(type) {
		case proto.Message:
			// proto.Message -> []byte
			err := kvCache.Set(cacheKeyName, realData, 0)
			if err != nil {
				GetLogger().Error("%v cache err:%v", cacheKeyName, err.Error())
				return
			}
		default:
			GetLogger().Error("%v cache err:unsupport type:%v", cacheKeyName, reflect.TypeOf(realData))
			return
		}

	case reflect.Struct:
		if cacheData := convertStructToInterface(val); cacheData != nil {
			SaveValueToCache(kvCache, cacheKeyName, reflect.ValueOf(cacheData))
			return
		}
		GetLogger().Error("%v cache err:unsupport type:%v", cacheKeyName, val)

	case reflect.Map:
		// map格式作为一个整体缓存时,需要先删除之前的数据
		_, err := kvCache.Del(cacheKeyName)
		if IsRedisError(err) {
			GetLogger().Error("%v cache err:%v", cacheKeyName, err.Error())
			return
		}
		cacheData := val.Interface()
		// map -> hash
		err = kvCache.SetMap(cacheKeyName, cacheData)
		if IsRedisError(err) {
			GetLogger().Error("%v cache err:%v", cacheKeyName, err.Error())
			return
		}

	case reflect.Slice, reflect.Array:
		cacheData := val.Interface()
		// slice,用json序列化
		jsonBytes, err := json.Marshal(cacheData)
		if err != nil {
			GetLogger().Error("%v json.Marshal err:%v", cacheKeyName, err.Error())
			return
		}
		// slice -> []byte
		err = kvCache.Set(cacheKeyName, string(jsonBytes), 0)
		if IsRedisError(err) {
			GetLogger().Error("%v cache err:%v", cacheKeyName, err.Error())
			return
		}

	default:
		GetLogger().Error("%v cache err:unsupport kind:%v", cacheKeyName, val.Kind())
	}
}

// 保存map类型字段到redis
func SaveMapValueToCache(kvCache KvCache, cacheKeyName string, val reflect.Value, dirtyMark MapDirtyMark) {
	cacheData := val.Interface()
	if !dirtyMark.HasCached() {
		// 必须把整体数据缓存一次,后面的修改才能增量更新
		if cacheData == nil {
			return
		}
		err := kvCache.SetMap(cacheKeyName, cacheData)
		if IsRedisError(err) {
			GetLogger().Error("%v cache err:%v", cacheKeyName, err.Error())
			return
		}
		dirtyMark.SetCached()
	} else {
		setMap := make(map[interface{}]interface{})
		var delMap []string
		dirtyMark.RangeDirtyMap(func(dirtyKey interface{}, isAddOrUpdate bool) {
			if isAddOrUpdate {
				mapValue := val.MapIndex(reflect.ValueOf(dirtyKey))
				if mapValue.IsValid() {
					// use ConvertValueToInterface()?
					if !mapValue.CanInterface() {
						GetLogger().Error("%v mapValue.CanInterface() false dirtyKey:%v", cacheKeyName, dirtyKey)
						return
					}
					setMap[dirtyKey] = mapValue.Interface()
				} else {
					GetLogger().Debug("%v mapValue.IsValid() false dirtyKey:%v", cacheKeyName, dirtyKey)
				}
			} else {
				// delete
				delMap = append(delMap, util.Itoa(dirtyKey))
			}
		})
		if len(setMap) > 0 {
			// 批量更新
			err := kvCache.SetMap(cacheKeyName, setMap)
			if IsRedisError(err) {
				GetLogger().Error("%v cache %v err:%v", cacheKeyName, setMap, err.Error())
				return
			}
		}
		if len(delMap) > 0 {
			// 批量删除
			_, err := kvCache.HDel(cacheKeyName, delMap...)
			if IsRedisError(err) {
				GetLogger().Error("%v cache %v err:%v", cacheKeyName, delMap, err.Error())
				return
			}
		}
	}
}

// Entity的变化数据保存到数据库
//
//	key为entity.GetId()
func SaveEntityChangedDataToDb(entityDb EntityDb, entity Entity, kvCache KvCache, removeCacheAfterSaveDb bool, cachePrefix string) error {
	return SaveEntityChangedDataToDbByKey(entityDb, entity, entity.GetId(), kvCache, removeCacheAfterSaveDb, cachePrefix)
}

type saveDataRecord struct {
	changedData map[string]any
	saved       []Saveable
	delKeys     []string
}

func saveObjectChangedDataToDbByKey(entityDb EntityDb, obj any, entityKey interface{}, kvCache KvCache,
	removeCacheAfterSaveDb bool, objName string, parentCacheKey string, record *saveDataRecord) {
	objStruct := GetObjSaveableStruct(obj)
	if objStruct == nil {
		// 组件可以没有保存字段
		return
	}
	if objStruct.IsSingleField() {
		saveable, saveableField := objStruct.GetSingleSaveable(obj)
		if saveable == nil {
			GetLogger().Error("%v Save %v Err:obj not a saveable", entityKey, objStruct.Field.Name)
			return
		}
		//depth := 0
		//// 找到实际需要保存的字段(叶子节点)
		//fieldObj, _ := GetSingleSaveableField(obj, objStruct.Field, &depth)
		//if fieldObj == nil {
		//	GetLogger().Error("%v %v.%v not find saveable field,depth:%v", entityKey, objName, objStruct.Field.Name, depth)
		//	return
		//}
		//GetLogger().Debug("GetSingleSaveableField %v %v.%v depth:%v", entityKey, objName, objStruct.Field.Name, depth)
		// 如果某个组件数据没改变过,就无需保存
		if !saveable.IsChanged() {
			GetLogger().Debug("%v ignore %v", entityKey, saveableField.Name)
			return
		}
		saveData, err := getSaveDataOfSaveable(saveable, saveableField, objName)
		if err != nil {
			GetLogger().Error("%v Save %v err:%v", entityKey, saveableField.Name, err.Error())
			return
		}
		// 使用protobuf存mongodb时,mongodb默认会把字段名转成小写,因为protobuf没设置bson tag
		record.changedData[objName] = saveData
		if removeCacheAfterSaveDb {
			record.delKeys = append(record.delKeys, fmt.Sprintf("%v.%v", parentCacheKey, saveableField.Name))
		}
		record.saved = append(record.saved, saveable)
		GetLogger().Debug("SaveDb %v %v", entityKey, saveableField.Name)
	} else {
		objVal := reflect.ValueOf(obj)
		if objVal.Kind() == reflect.Pointer {
			objVal = objVal.Elem()
		}
		for childIndex, childStruct := range objStruct.Children {
			saveable, saveableField := objStruct.GetChildSaveable(obj, childIndex)
			if saveable == nil {
				GetLogger().Error("%v SaveChild %v Err:field not a saveable", entityKey, childStruct.Name)
				continue
			}
			// 如果某个组件数据没改变过,就无需保存
			if !saveable.IsChanged() {
				GetLogger().Debug("%v ignore child %v", entityKey, saveableField.Name)
				continue
			}
			saveData, err := getSaveDataOfSaveable(saveable, saveableField, objName)
			if err != nil {
				GetLogger().Error("%v SaveChild %v err:%v", entityKey, saveableField.Name, err.Error())
				continue
			}
			// 使用protobuf存mongodb时,mongodb默认会把字段名转成小写,因为protobuf没设置bson tag
			childName := ""
			if _saveableStructsMap.useLowerName {
				childName = objName + "." + strings.ToLower(childStruct.Name)
			} else {
				childName = objName + "." + childStruct.Name
			}
			record.changedData[childName] = saveData
			if removeCacheAfterSaveDb {
				record.delKeys = append(record.delKeys, fmt.Sprintf("%v.%v", parentCacheKey, childName))
			}
			record.saved = append(record.saved, saveable)
			GetLogger().Debug("SaveDb Child %v %v", entityKey, childName)
			//childCacheName := parentCacheKey + "." + saveableField.Name
			//saveObjectChangedDataToDbByKey(entityDb, saveable, entityKey, kvCache, removeCacheAfterSaveDb, saveableField.Name, childCacheName, record)

			//childName := ""
			//if _saveableStructsMap.useLowerName {
			//	childName = objName + "." + strings.ToLower(childStruct.Name)
			//} else {
			//	childName = objName + "." + childStruct.Name
			//}
			//fieldVal := objVal.Field(childStruct.FieldIndex)
			//if util.IsValueNil(fieldVal) {
			//	record.changedData[childName] = nil
			//	continue
			//}
			//var fieldInterface any
			//if fieldVal.Kind() == reflect.Struct {
			//	fieldInterface = convertStructToInterface(fieldVal)
			//} else {
			//	fieldInterface = fieldVal.Interface()
			//}
			//if fieldInterface == nil {
			//	GetLogger().Error("save %v %v err", entityKey, childName)
			//	continue
			//}
			//childCacheName := ""
			//if _saveableStructsMap.useLowerName {
			//	childCacheName = parentCacheKey + "." + strings.ToLower(childStruct.Name)
			//} else {
			//	childCacheName = parentCacheKey + "." + childStruct.Name
			//}
			//saveObjectChangedDataToDbByKey(entityDb, fieldInterface, entityKey, kvCache, removeCacheAfterSaveDb, childName, childCacheName, record)
		}
	}
}

// Entity的变化数据保存到数据库
//
//	指定key
func SaveEntityChangedDataToDbByKey(entityDb EntityDb, entity Entity, entityKey interface{}, kvCache KvCache, removeCacheAfterSaveDb bool, cachePrefix string) error {
	record := &saveDataRecord{
		changedData: make(map[string]any),
	}
	entity.RangeComponent(func(component Component) bool {
		saveObjectChangedDataToDbByKey(entityDb, component, entityKey, kvCache, removeCacheAfterSaveDb,
			component.GetName(), GetEntityCacheKey(cachePrefix, entityKey), record)
		return true
	})
	if len(record.changedData) == 0 {
		GetLogger().Debug("ignore unchanged data %v", entityKey)
		return nil
	}
	// NOTE: 明文保存的proto字段,字段名会被mongodb自动转为小写 Q:有办法解决吗?
	// 如examples里的baseInfoComponent的pb.BaseInfo的LongFieldNameTest字段在mongodb中会被转成longfieldnametest
	saveDbErr := entityDb.SaveComponents(entityKey, record.changedData)
	if saveDbErr != nil {
		GetLogger().Error("SaveDb %v err:%v", entityKey, saveDbErr)
		GetLogger().Error("%v", record.changedData)
	} else {
		GetLogger().Debug("SaveDb %v", entityKey)
	}
	if saveDbErr == nil {
		// 保存数据库成功后,重置修改标记
		for _, saveable := range record.saved {
			saveable.ResetChanged()
		}
		if len(record.delKeys) > 0 {
			// 保存数据库成功后,才删除缓存
			kvCache.Del(record.delKeys...)
			GetLogger().Debug("RemoveCache %v %v", entityKey, record.delKeys)
		}
	}
	return saveDbErr
}

// 获取实体需要保存到数据库的完整数据
func GetEntitySaveData(entity Entity, componentDatas map[string]interface{}) {
	entity.RangeComponent(func(component Component) bool {
		structCache := GetObjSaveableStruct(component)
		if structCache == nil {
			// 组件可以没有保存字段
			return true
		}
		saveData, err := GetComponentSaveData(component)
		if err != nil {
			GetLogger().Error("%v %v err:%v", entity.GetId(), component.GetName(), err.Error())
			return true
		}
		componentDatas[GetComponentSaveName(component)] = saveData
		GetLogger().Debug("GetEntitySaveData %v %v", entity.GetId(), component.GetName())
		return true
	})
}

func saveFieldMapByKeyType[K comparable](obj interface{}, field reflect.Value, parentName string, fieldStruct *SaveableField, keyFn func(*reflect.MapIter) K) (interface{}, error) {
	// map[K]proto.Message -> map[K][]byte
	// map[K]interface{} -> map[K]interface{}
	newMap := make(map[K]any)
	it := field.MapRange()
	for it.Next() {
		// map的value是proto格式,进行序列化
		key := keyFn(it)
		valueInterface := it.Value().Interface()
		v, err := getInterfaceSaveData(valueInterface, parentName, fieldStruct)
		if err != nil {
			GetLogger().Error("%v.%v convert key:%v err:%v", parentName, fieldStruct.Name, key, err.Error())
			return nil, err
		}
		newMap[key] = v
	}
	return newMap, nil
}

func saveFieldMap(obj interface{}, field reflect.Value, parentName string, fieldStruct *SaveableField) (interface{}, error) {
	typ := field.Type()
	keyType := typ.Key()
	valType := typ.Elem()
	if valType.Kind() == reflect.Interface || valType.Kind() == reflect.Ptr {
		switch keyType.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return saveFieldMapByKeyType(obj, field, parentName, fieldStruct, func(iter *reflect.MapIter) int64 {
				return iter.Key().Int()
			})
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			// map[uint]proto.Message -> map[uint64][]byte
			// map[uint]interface{} -> map[uint64]interface{}
			return saveFieldMapByKeyType(obj, field, parentName, fieldStruct, func(iter *reflect.MapIter) uint64 {
				return iter.Key().Uint()
			})
		case reflect.String:
			// map[string]proto.Message -> map[string][]byte
			// map[string]interface{} -> map[string]interface{}
			return saveFieldMapByKeyType(obj, field, parentName, fieldStruct, func(iter *reflect.MapIter) string {
				return iter.Key().String()
			})
		case reflect.Bool:
			// map[bool]proto.Message -> map[bool][]byte
			// map[bool]interface{} -> map[bool]interface{}
			return saveFieldMapByKeyType(obj, field, parentName, fieldStruct, func(iter *reflect.MapIter) bool {
				return iter.Key().Bool()
			})
		case reflect.Float32, reflect.Float64:
			// map[float]proto.Message -> map[float][]byte
			// map[float]interface{} -> map[float]interface{}
			return saveFieldMapByKeyType(obj, field, parentName, fieldStruct, func(iter *reflect.MapIter) float64 {
				return iter.Key().Float()
			})
		case reflect.Complex64, reflect.Complex128:
			// map[complex]proto.Message -> map[complex][]byte
			// map[complex]interface{} -> map[complex]interface{}
			return saveFieldMapByKeyType(obj, field, parentName, fieldStruct, func(iter *reflect.MapIter) complex128 {
				return iter.Key().Complex()
			})
		default:
			GetLogger().Error("%v.%v unsupported map key type:%v", parentName, fieldStruct.Name, keyType.Kind())
			return nil, ErrUnsupportedKeyType
		}
	} else {
		// map的value是基础类型,无需序列化,直接返回
		return field.Interface(), nil
	}
}

func saveFieldSlice(obj interface{}, field reflect.Value, parentName string, fieldStruct *SaveableField) (interface{}, error) {
	typ := field.Type()
	valType := typ.Elem()
	if valType.Kind() == reflect.Interface || valType.Kind() == reflect.Ptr {
		newSlice := make([]interface{}, 0, field.Len())
		for i := 0; i < field.Len(); i++ {
			sliceElem := field.Index(i)
			valueInterface := sliceElem.Interface()
			v, err := getInterfaceSaveData(valueInterface, parentName, fieldStruct)
			if err != nil {
				GetLogger().Error("%v.%v convert index:%v err:%v", parentName, fieldStruct.Name, i, err.Error())
				return nil, err
			}
			newSlice = append(newSlice, v)
		}
		// proto
		return newSlice, nil
	} else {
		// slice的value是基础类型,无需序列化,直接返回
		return field.Interface(), nil
	}
}

func saveFieldPtr(obj interface{}, field reflect.Value, parentName string, fieldStruct *SaveableField) (interface{}, error) {
	fieldInterface := field.Interface()
	return getInterfaceSaveData(fieldInterface, parentName, fieldStruct)
}

func saveFieldStruct(obj interface{}, field reflect.Value, parentName string, fieldStruct *SaveableField) (interface{}, error) {
	if fieldInterface := convertStructToInterface(field); fieldInterface != nil {
		return getInterfaceSaveData(fieldInterface, parentName, fieldStruct)
	}
	GetLogger().Error("%v.%v not a addr struct type:%v", parentName, fieldStruct.Name, field.Type().String())
	return field.Interface(), nil
}

func convertStructToInterface(field reflect.Value) any {
	if !field.CanAddr() {
		return nil
	}
	fieldAddr := field.Addr()
	if !fieldAddr.CanInterface() {
		return nil
	}
	return fieldAddr.Interface()
}

func convertStructToSaveable(field reflect.Value) Saveable {
	if fieldInterface := convertStructToInterface(field); fieldInterface != nil {
		if saveable, ok := fieldInterface.(Saveable); ok {
			return saveable
		}
	}
	return nil
}

func getInterfaceSaveData(fieldInterface interface{}, parentName string, fieldStruct *SaveableField) (interface{}, error) {
	if protoMessage, ok := fieldInterface.(proto.Message); ok {
		return proto.Marshal(protoMessage)
	} else {
		// Saveable
		// TODO: 移除该支持 以简化代码
		return getSaveableSaveData(fieldInterface, parentName, fieldStruct)
	}
}

func getSaveableSaveData(fieldInterface interface{}, parentName string, fieldStruct *SaveableField) (interface{}, error) {
	// Saveable
	if valueSaveable, ok := fieldInterface.(Saveable); ok {
		valueSaveData, valueSaveErr := GetSaveData(valueSaveable, parentName)
		if valueSaveErr != nil {
			GetLogger().Error("%v.%v Saveable err:%v", parentName, fieldStruct.Name, valueSaveErr.Error())
			return nil, valueSaveErr
		}
		return valueSaveData, nil
	} else {
		// TODO:扩展一个自定义序列化接口 customSerialize()(interface{}, error)
		GetLogger().Error("%v.%v not Saveable type:%v", parentName, fieldStruct.Name, reflect.TypeOf(fieldInterface).String())
		return nil, errors.New(fmt.Sprintf("%v.%v not Saveable type:%v", parentName, fieldStruct.Name, reflect.TypeOf(fieldInterface).String()))
	}
}

func getSaveDataOfSaveable(saveable Saveable, saveableField *SaveableField, parentName string) (interface{}, error) {
	objVal := reflect.ValueOf(saveable)
	if objVal.Kind() == reflect.Ptr {
		objVal = objVal.Elem()
	}
	field := objVal.Field(saveableField.FieldIndex)
	if util.IsValueNil(field) {
		return nil, nil
	}
	// 明文保存的数据
	if saveableField.IsPlain {
		//// 明文保存的特殊结构体需要特殊处理,如MapData[K comparable, V any]
		//switch field.Kind() {
		//case reflect.Ptr:
		//	if field.CanInterface() {
		//		fieldInterface := field.Interface()
		//		if _, ok := fieldInterface.(Saveable); ok {
		//			return getSaveableSaveData(fieldInterface, parentName, saveableField)
		//		}
		//	}
		//case reflect.Struct:
		//	if fieldInterface := convertStructToInterface(field); fieldInterface != nil {
		//		if _, ok := fieldInterface.(Saveable); ok {
		//			return getSaveableSaveData(fieldInterface, parentName, saveableField)
		//		}
		//	}
		//}
		fieldInterface := field.Interface()
		//if _, ok := fieldInterface.(Saveable); ok {
		//	return getSaveableSaveData(field.Interface(), parentName, saveableField)
		//}
		// 明文保存的普通数据,直接返回原始数据
		return fieldInterface, nil
	}
	// 非明文保存的数据,一般用于对proto进行序列化
	switch field.Kind() {
	case reflect.Map:
		return saveFieldMap(saveable, field, parentName, saveableField)
	case reflect.Slice:
		return saveFieldSlice(saveable, field, parentName, saveableField)
	case reflect.Ptr:
		return saveFieldPtr(saveable, field, parentName, saveableField)
	case reflect.Struct:
		return saveFieldStruct(saveable, field, parentName, saveableField)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return field.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return field.Uint(), nil
	case reflect.Bool:
		return field.Bool(), nil
	case reflect.Float32, reflect.Float64:
		return field.Float(), nil
	case reflect.Complex64, reflect.Complex128:
		return field.Complex(), nil
	case reflect.String:
		return field.String(), nil
	default:
		GetLogger().Error("%v %v unsupported fieldKind:%v", parentName, saveableField.Name, field.Kind())
		return nil, ErrUnsupportedKeyType
	}
}

// 获取对象的保存数据
func GetSaveData(obj any, parentName string) (interface{}, error) {
	objStruct := GetObjSaveableStruct(obj)
	if objStruct == nil {
		GetLogger().Error("not saveable %v type:%v", parentName, reflect.TypeOf(obj))
		return nil, nil
	}
	objVal := reflect.ValueOf(obj)
	if objVal.Kind() == reflect.Ptr {
		objVal = objVal.Elem()
	}
	if objStruct.IsSingleField() {
		saveable, saveableField := objStruct.GetSingleSaveable(obj)
		if saveable == nil {
			// return nil, nil
			GetLogger().Error("GetSaveData %v.%v err", parentName, objStruct.Field.Name)
			return nil, ErrUnsupportedType
		}
		return getSaveDataOfSaveable(saveable, saveableField, parentName)
	} else {
		// 多个child子模块的组合
		compositeSaveData := make(map[string]interface{})
		for childIndex, childStruct := range objStruct.Children {
			saveable, saveableField := objStruct.GetChildSaveable(obj, childIndex)
			if saveable == nil {
				GetLogger().Error("GetSaveData %v Err:field not a saveable", childStruct.Name)
				return nil, ErrNotSaveable
			}
			childName := parentName + "." + childStruct.Name
			childSaveData, err := getSaveDataOfSaveable(saveable, saveableField, childName)
			if err != nil {
				GetLogger().Error("GetSaveDataErr %v", childName)
				return nil, err
			}
			compositeSaveData[childStruct.Name] = childSaveData

			//val := objVal.Field(childStruct.FieldIndex)
			//if util.IsValueNil(val) {
			//	compositeSaveData[childStruct.Name] = nil
			//	continue
			//}
			//fieldInterface := val.Interface()
			//childSaveData, err := GetSaveData(fieldInterface, childName)
			//if err != nil {
			//	return nil, err
			//}
			//compositeSaveData[childStruct.Name] = childSaveData
		}
		return compositeSaveData, nil
	}
}
