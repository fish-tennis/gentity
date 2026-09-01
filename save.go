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

// 获取组件的完整保存数据
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
			glog.Error("SaveObjectChangedDataToCache fieldObj nil", "cacheKey", cacheKey)
			return
		}
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
					glog.Error("SaveObjectChangedDataToCache child err", "cacheKey", cacheKey, "fieldName", childStruct.Name, "err", err)
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
				glog.Error("SaveObjectChangedDataToCache field err", "cacheKey", cacheKey)
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
				glog.Error("kvCache.Del err", "cacheKey", cacheKeyName, "err", err)
				return
			}
		} else {
			SaveValueToCache(kvCache, cacheKeyName, val)
		}
		dirtyMark.ResetDirty()
		glog.Debug("SaveCache", "cacheKey", cacheKeyName)
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
				glog.Error("kvCache.Del err", "cacheKey", cacheKeyName, "err", err)
				return
			}
		} else {
			SaveMapValueToCache(kvCache, cacheKeyName, val, dirtyMark)
		}
		dirtyMark.ResetDirty()
		glog.Debug("SaveMapCache", "cacheKey", cacheKeyName)
	}
}

// 把修改数据保存到缓存
func SaveChangedDataToCache(kvCache KvCache, obj any, cacheKeyName string, saveableField *SaveableField) {
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
				glog.Error("kvCache.Set err", "cacheKey", cacheKeyName, "err", err)
				return
			}
		default:
			glog.Error("SaveValueToCache err:unsupport type", "cacheKey", cacheKeyName, "type", reflect.TypeOf(realData))
			return
		}

	case reflect.Struct:
		if cacheData := convertStructToInterface(val); cacheData != nil {
			SaveValueToCache(kvCache, cacheKeyName, reflect.ValueOf(cacheData))
			return
		}
		glog.Error("SaveValueToCache err:unsupport type", "cacheKey", cacheKeyName, "type", val)

	case reflect.Map:
		cacheData := val.Interface()
		if scriptCache, ok := kvCache.(ScriptKvCache); ok {
			// Lua脚本原子执行 Del+HSet:分步执行时,Del与写入之间并发读会读到空数据,
			// 进程崩溃则会留下已删未写的丢失状态
			err := scriptCache.AtomicReplaceMap(cacheKeyName, cacheData)
			if err != nil {
				glog.Error("AtomicReplaceMap err", "cacheKey", cacheKeyName, "err", err)
				return
			}
			return
		}
		// 回退:分步执行(不支持脚本的缓存系统)
		// map格式作为一个整体缓存时,需要先删除之前的数据
		_, err := kvCache.Del(cacheKeyName)
		if IsRedisError(err) {
			glog.Error("kvCache.Del err", "cacheKey", cacheKeyName, "err", err)
			return
		}
		// map -> hash
		err = kvCache.SetMap(cacheKeyName, cacheData)
		if IsRedisError(err) {
			glog.Error("cache err", "cacheKey", cacheKeyName, "err", err)
			return
		}

	case reflect.Slice, reflect.Array:
		cacheData := val.Interface()
		// slice,用json序列化
		jsonBytes, err := json.Marshal(cacheData)
		if err != nil {
			glog.Error("json.Marshal err", "cacheKey", cacheKeyName, "err", err)
			return
		}
		// slice -> []byte
		err = kvCache.Set(cacheKeyName, string(jsonBytes), 0)
		if IsRedisError(err) {
			glog.Error("kvCache.Set err", "cacheKey", cacheKeyName, "err", err)
			return
		}

	default:
		glog.Error("SaveValueToCache err:unsupport kind", "cacheKey", cacheKeyName, "kind", val.Kind())
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
			glog.Error("kvCache.SetMap err", "cacheKey", cacheKeyName, "err", err)
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
						glog.Error("mapValue.CanInterface() false", "cacheKey", cacheKeyName, "dirtyKey", dirtyKey)
						return
					}
					setMap[dirtyKey] = mapValue.Interface()
				} else {
					glog.Debug("mapValue.IsValid() false", "cacheKey", cacheKeyName, "dirtyKey", dirtyKey)
				}
			} else {
				// delete
				delMap = append(delMap, util.Itoa(dirtyKey))
			}
		})
		if len(setMap) > 0 || len(delMap) > 0 {
			if scriptCache, ok := kvCache.(ScriptKvCache); ok {
				// Lua脚本原子执行 HSet+HDel:
				// 分步执行时若set成功del失败(或中途崩溃),调用方ResetDirty后不再重试,
				// 内存与缓存将永久不一致,最终把错误数据落库;原子执行则不会出现半成品状态
				err := scriptCache.AtomicUpdateMap(cacheKeyName, setMap, delMap)
				if err != nil {
					glog.Error("AtomicUpdateMap err", "cacheKey", cacheKeyName, "setMap", setMap, "delMap", delMap, "err", err)
					return
				}
				return
			}
			// 回退:分步执行(不支持脚本的缓存系统)
			if len(setMap) > 0 {
				// 批量更新
				err := kvCache.SetMap(cacheKeyName, setMap)
				if IsRedisError(err) {
					glog.Error("kvCache.SetMap err", "cacheKey", cacheKeyName, "setMap", setMap, "err", err)
					return
				}
			}
			if len(delMap) > 0 {
				// 批量删除
				_, err := kvCache.HDel(cacheKeyName, delMap...)
				if IsRedisError(err) {
					glog.Error("kvCache.HDel err", "cacheKey", cacheKeyName, "delMap", delMap, "err", err)
					return
				}
			}
		}
	}
}

// resolveShardKey 检测entityDb+entity的可选分片键支持
// 返回的ShardKeyEntityDb非nil表示已启用(同时返回分片键值),nil表示退化为常规操作
// 条件:entityDb的分片键列已启用(ShardKeyName非空)且entity提供分片键值(ShardKeyProvider)
func resolveShardKey(entityDb EntityDb, entity Entity) (ShardKeyEntityDb, interface{}) {
	if shardDb, ok := entityDb.(ShardKeyEntityDb); ok && shardDb.ShardKeyName() != "" {
		if provider, ok := entity.(ShardKeyProvider); ok {
			return shardDb, provider.GetShardKeyValue()
		}
	}
	return nil, nil
}

// saveComponentsOptionalShardKey 批量保存组件变更数据,按需附加分片键条件直达目标分片
// 未启用分片键时退化为常规SaveComponents(分片集群下广播,不影响正确性,便于渐进式启用)
func saveComponentsOptionalShardKey(entityDb EntityDb, entity Entity, entityKey interface{}, changedData map[string]any) error {
	if shardDb, shardKeyValue := resolveShardKey(entityDb, entity); shardDb != nil {
		return shardDb.SaveComponentsWithShardKey(shardKeyValue, entityKey, changedData)
	}
	return entityDb.SaveComponents(entityKey, changedData)
}

// saveComponentOptionalShardKey 保存单个组件,按需附加分片键条件(见resolveShardKey)
func saveComponentOptionalShardKey(entityDb EntityDb, entity Entity, entityKey interface{}, componentName string, componentData interface{}) error {
	if shardDb, shardKeyValue := resolveShardKey(entityDb, entity); shardDb != nil {
		return shardDb.SaveComponentWithShardKey(shardKeyValue, entityKey, componentName, componentData)
	}
	return entityDb.SaveComponent(entityKey, componentName, componentData)
}

// saveComponentFieldOptionalShardKey 保存组件的单个字段,按需附加分片键条件(见resolveShardKey)
func saveComponentFieldOptionalShardKey(entityDb EntityDb, entity Entity, entityKey interface{}, componentName string, fieldName string, fieldData interface{}) error {
	if shardDb, shardKeyValue := resolveShardKey(entityDb, entity); shardDb != nil {
		return shardDb.SaveComponentFieldWithShardKey(shardKeyValue, entityKey, componentName, fieldName, fieldData)
	}
	return entityDb.SaveComponentField(entityKey, componentName, fieldName, fieldData)
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
			glog.Error("saveObjectChangedDataToDbByKey Err:obj not a saveable", "entityKey", entityKey, "field", objStruct.Field.Name)
			return
		}
		// 如果某个组件数据没改变过,就无需保存
		if !saveable.IsChanged() {
			glog.Debug("saveObjectChangedDataToDbByKey ignore", "entityKey", entityKey, "field", saveableField.Name)
			return
		}
		saveData, err := getSaveDataOfSaveable(saveable, saveableField, objName)
		if err != nil {
			glog.Error("getSaveDataOfSaveable err", "entityKey", entityKey, "field", saveableField.Name, "err", err)
			return
		}
		// 使用protobuf存mongodb时,mongodb默认会把字段名转成小写,因为protobuf没设置bson tag
		record.changedData[objName] = saveData
		if removeCacheAfterSaveDb {
			record.delKeys = append(record.delKeys, fmt.Sprintf("%v.%v", parentCacheKey, saveableField.Name))
		}
		record.saved = append(record.saved, saveable)
		glog.Debug("saveObjectChangedDataToDbByKey", "entityKey", entityKey, "field", saveableField.Name)
	} else {
		objVal := reflect.ValueOf(obj)
		if objVal.Kind() == reflect.Pointer {
			objVal = objVal.Elem()
		}
		for childIndex, childStruct := range objStruct.Children {
			saveable, saveableField := objStruct.GetChildSaveable(obj, childIndex)
			if saveable == nil {
				glog.Error("saveObjectChangedDataToDbByKey Err:field not a saveable", "entityKey", entityKey, "field", childStruct.Name)
				continue
			}
			// 如果某个组件数据没改变过,就无需保存
			if !saveable.IsChanged() {
				glog.Debug("saveObjectChangedDataToDbByKey ignore child", "entityKey", entityKey, "field", saveableField.Name)
				continue
			}
			saveData, err := getSaveDataOfSaveable(saveable, saveableField, objName)
			if err != nil {
				glog.Error("getSaveDataOfSaveable err", "entityKey", entityKey, "field", saveableField.Name, "err", err)
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
			glog.Debug("saveObjectChangedDataToDbByKey Child", "entityKey", entityKey, "child", childName)
		}
	}
}

// Entity的变化数据保存到数据库,只保存有数据变化的组件数据,但组件的数据不会分割,只要一个组件有数据变化,组件的数据就是全量覆盖
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
		glog.Debug("SaveEntityChangedDataToDbByKey ignore unchanged data", "entityKey", entityKey)
		return nil
	}
	// NOTE: 明文保存的proto字段,字段名会被mongodb自动转为小写 Q:有办法解决吗?
	// 如examples里的baseInfoComponent的pb.BaseInfo的LongFieldNameTest字段在mongodb中会被转成longfieldnametest
	// entityDb支持分片键且entity实现ShardKeyProvider时,自动附加分片键条件直达分片(见saveComponentsOptionalShardKey)
	saveDbErr := saveComponentsOptionalShardKey(entityDb, entity, entityKey, record.changedData)
	if saveDbErr != nil {
		glog.Error("SaveEntityChangedDataToDbByKey", "entityKey", entityKey, "err", saveDbErr)
		glog.Error("SaveEntityChangedDataToDbByKeyErr", "data", record.changedData)
	} else {
		glog.Debug("SaveEntityChangedDataToDbByKey", "entityKey", entityKey)
	}
	if saveDbErr == nil {
		// 保存数据库成功后,重置修改标记
		for _, saveable := range record.saved {
			saveable.ResetChanged()
		}
		if len(record.delKeys) > 0 {
			// 保存数据库成功后,才删除缓存
			kvCache.Del(record.delKeys...)
			glog.Debug("RemoveCache", "entityKey", entityKey, "delKeys", record.delKeys)
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
			glog.Error("GetEntitySaveData err", "entityKey", entity.GetId(), "component", component.GetName(), "err", err)
			return true
		}
		componentDatas[GetComponentSaveName(component)] = saveData
		glog.Debug("GetEntitySaveData", "entityKey", entity.GetId(), "component", component.GetName())
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
			glog.Error("getInterfaceSaveDataErr", "parent", parentName, "field", fieldStruct.Name, "key", key, "err", err)
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
			glog.Error("unsupported map key type", "parent", parentName, "field", fieldStruct.Name, "type", keyType.Kind())
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
				glog.Error("getInterfaceSaveDataErr", "parent", parentName, "field", fieldStruct.Name, "index", i, "err", err)
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
	glog.Error("not a addr struct", "parent", parentName, "field", fieldStruct.Name, "type", field.Type().String())
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

func getInterfaceSaveData(fieldInterface any, parentName string, fieldStruct *SaveableField) (any, error) {
	if protoMessage, ok := fieldInterface.(proto.Message); ok {
		return proto.Marshal(protoMessage)
	} else {
		// 支持map[key]Saveable的特殊动态结构
		return getSaveableSaveData(fieldInterface, parentName, fieldStruct)
	}
}

func getSaveableSaveData(fieldInterface any, parentName string, fieldStruct *SaveableField) (any, error) {
	// 支持map[key]Saveable的特殊动态结构
	if valueSaveable, ok := fieldInterface.(Saveable); ok {
		valueSaveData, valueSaveErr := GetSaveData(valueSaveable, parentName)
		if valueSaveErr != nil {
			glog.Error("Saveable err", "parent", parentName, "field", fieldStruct.Name, "err", valueSaveErr)
			return nil, valueSaveErr
		}
		return valueSaveData, nil
	} else {
		// TODO:扩展一个自定义序列化接口 customSerialize()(interface{}, error)
		glog.Error("not Saveable", "parent", parentName, "field", fieldStruct.Name, "type", reflect.TypeOf(fieldInterface).String())
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
		fieldInterface := field.Interface()
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
		glog.Error("unsupported fieldKind", "parent", parentName, "field", saveableField.Name, "kind", field.Kind())
		return nil, ErrUnsupportedKeyType
	}
}

// 获取对象的完整保存数据
func GetSaveData(obj any, parentName string) (interface{}, error) {
	objStruct := GetObjSaveableStruct(obj)
	if objStruct == nil {
		glog.Error("not saveable", "parent", parentName, "type", reflect.TypeOf(obj))
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
			glog.Error("GetSaveData err", "parent", parentName, "field", objStruct.Field.Name)
			return nil, ErrUnsupportedType
		}
		return getSaveDataOfSaveable(saveable, saveableField, parentName)
	} else {
		// 多个child子模块的组合
		compositeSaveData := make(map[string]interface{})
		for childIndex, childStruct := range objStruct.Children {
			saveable, saveableField := objStruct.GetChildSaveable(obj, childIndex)
			if saveable == nil {
				glog.Error("GetSaveData Err:field not a saveable", "field", childStruct.Name)
				return nil, ErrNotSaveable
			}
			childName := parentName + "." + childStruct.Name
			childSaveData, err := getSaveDataOfSaveable(saveable, saveableField, childName)
			if err != nil {
				glog.Error("GetSaveDataErr", "childName", childName)
				return nil, err
			}
			compositeSaveData[childStruct.Name] = childSaveData
		}
		return compositeSaveData, nil
	}
}
