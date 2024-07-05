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

// 把组件的修改数据保存到缓存
func SaveComponentChangedDataToCache(kvCache KvCache, cacheKeyPrefix string, entityKey interface{}, component Component) {
	structCache := GetSaveableStruct(reflect.TypeOf(component))
	if structCache == nil {
		return
	}
	if structCache.IsSingleField() {
		cacheKey := GetEntityComponentCacheKey(cacheKeyPrefix, entityKey, component.GetName())
		SaveChangedDataToCache(kvCache, component, cacheKey)
	} else {
		reflectVal := reflect.ValueOf(component).Elem()
		for _, fieldCache := range structCache.Children {
			cacheKey := GetEntityComponentChildCacheKey(cacheKeyPrefix, entityKey, component.GetName(), fieldCache.Name)
			val := reflectVal.Field(fieldCache.FieldIndex)
			if util.IsValueNil(val) {
				_, err := kvCache.Del(cacheKey)
				if IsRedisError(err) {
					GetLogger().Error("%v cache err:%v", cacheKey, err.Error())
				}
				continue
			}
			fieldInterface := val.Interface()
			SaveChangedDataToCache(kvCache, fieldInterface, cacheKey)
		}
	}
}

// 把修改数据保存到缓存
func SaveChangedDataToCache(kvCache KvCache, obj interface{}, cacheKeyName string) {
	structCache := GetSaveableStruct(reflect.TypeOf(obj))
	if structCache == nil {
		return
	}
	if !structCache.IsSingleField() {
		return
	}
	fieldCache := structCache.Field
	// 缓存数据作为一个整体的
	if dirtyMark, ok := obj.(DirtyMark); ok {
		if !dirtyMark.IsDirty() {
			return
		}
		reflectVal := reflect.ValueOf(obj).Elem()
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
		return
	}
	// map格式的
	if dirtyMark, ok := obj.(MapDirtyMark); ok {
		if !dirtyMark.IsDirty() {
			return
		}
		reflectVal := reflect.ValueOf(obj).Elem()
		val := reflectVal.Field(fieldCache.FieldIndex)
		if util.IsValueNil(val) {
			_, err := kvCache.Del(cacheKeyName)
			if IsRedisError(err) {
				GetLogger().Error("%v cache err:%v", cacheKeyName, err.Error())
				return
			}
		} else {
			if val.Kind() != reflect.Map {
				// 特殊结构体需要特殊处理,如MapData[K comparable, V any]
				switch val.Kind() {
				case reflect.Ptr:
					if _, ok := val.Interface().(Saveable); ok {
						SaveChangedDataToCache(kvCache, val.Interface(), cacheKeyName)
						return
					}
				case reflect.Struct:
					if valInterface := convertStructToInterface(val); valInterface != nil {
						if _, ok := valInterface.(Saveable); ok {
							SaveChangedDataToCache(kvCache, valInterface, cacheKeyName)
							return
						}
					}
				}
				GetLogger().Error("%v unsupport kind:%v", cacheKeyName, val.Kind())
				return
			}
			SaveMapValueToCache(kvCache, cacheKeyName, val, dirtyMark)
		}
		dirtyMark.ResetDirty()
		GetLogger().Debug("SaveCache %v", cacheKeyName)
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
			//switch realData := cacheData.(type) {
			//case proto.Message:
			//	// proto.Message -> []byte
			//	err := kvCache.Set(cacheKeyName, realData, 0)
			//	if err != nil {
			//		GetLogger().Error("%v cache err:%v", cacheKeyName, err.Error())
			//		return
			//	}
			//default:
			//	GetLogger().Error("%v cache err:unsupport type:%v", cacheKeyName, reflect.TypeOf(realData))
			//	return
			//}
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
		// TODO: elem类型?
		cacheData := val.Interface()
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

// Entity的变化数据保存到数据库
//
//	指定key
func SaveEntityChangedDataToDbByKey(entityDb EntityDb, entity Entity, entityKey interface{}, kvCache KvCache, removeCacheAfterSaveDb bool, cachePrefix string) error {
	changedDatas := make(map[string]interface{})
	var saved []Saveable
	var delKeys []string
	entity.RangeComponent(func(component Component) bool {
		structCache := GetSaveableStruct(reflect.TypeOf(component))
		if structCache == nil {
			return true
		}
		if structCache.IsSingleField() {
			if saveable, ok := component.(Saveable); ok {
				// 如果某个组件数据没改变过,就无需保存
				if !saveable.IsChanged() {
					GetLogger().Debug("%v ignore %v", entityKey, component.GetName())
					return true
				}
				saveData, err := GetComponentSaveData(component)
				if err != nil {
					GetLogger().Error("%v Save %v err:%v", entityKey, component.GetName(), err.Error())
					return true
				}
				// 使用protobuf存mongodb时,mongodb默认会把字段名转成小写,因为protobuf没设置bson tag
				changedDatas[GetComponentSaveName(component)] = saveData
				if removeCacheAfterSaveDb {
					delKeys = append(delKeys, GetEntityComponentCacheKey(cachePrefix, entityKey, component.GetName()))
				}
				saved = append(saved, saveable)
				GetLogger().Debug("SaveDb %v %v", entityKey, component.GetName())
			}
		} else {
			reflectVal := reflect.ValueOf(component).Elem()
			for _, fieldCache := range structCache.Children {
				childName := GetComponentSaveName(component) + "." + fieldCache.Name
				val := reflectVal.Field(fieldCache.FieldIndex)
				if util.IsValueNil(val) {
					changedDatas[childName] = nil
					continue
				}
				fieldInterface := val.Interface()
				if saveable, ok := fieldInterface.(Saveable); ok {
					// 如果某个组件数据没改变过,就无需保存
					if !saveable.IsChanged() {
						GetLogger().Debug("%v ignore %v.%v", entityKey, component.GetName(), childName)
						continue
					}
					childSaveData, err := GetSaveData(fieldInterface, childName)
					if err != nil {
						GetLogger().Error("%v Save %v.%v err:%v", entityKey, component.GetName(), childName, err.Error())
						continue
					}
					changedDatas[childName] = childSaveData
					if removeCacheAfterSaveDb {
						delKeys = append(delKeys, GetEntityComponentChildCacheKey(cachePrefix, entityKey, component.GetName(), fieldCache.Name))
					}
					saved = append(saved, saveable)
					GetLogger().Debug("SaveDb %v %v.%v", entityKey, component.GetName(), childName)
				}
			}
		}
		return true
	})
	if len(changedDatas) == 0 {
		GetLogger().Debug("ignore unchange data %v", entityKey)
		return nil
	}
	// NOTE: 明文保存的proto字段,字段名会被mongodb自动转为小写 Q:有办法解决吗?
	// 如examples里的baseInfoComponent的pb.BaseInfo的LongFieldNameTest字段在mongodb中会被转成longfieldnametest
	saveDbErr := entityDb.SaveComponents(entityKey, changedDatas)
	if saveDbErr != nil {
		GetLogger().Error("SaveDb %v err:%v", entityKey, saveDbErr)
		GetLogger().Error("%v", changedDatas)
	} else {
		GetLogger().Debug("SaveDb %v", entityKey)
	}
	if saveDbErr == nil {
		// 保存数据库成功后,重置修改标记
		for _, saveable := range saved {
			saveable.ResetChanged()
		}
		if len(delKeys) > 0 {
			// 保存数据库成功后,才删除缓存
			kvCache.Del(delKeys...)
			GetLogger().Debug("RemoveCache %v %v", entityKey, delKeys)
		}
	}
	return saveDbErr
}

// 获取实体需要保存到数据库的完整数据
func GetEntitySaveData(entity Entity, componentDatas map[string]interface{}) {
	entity.RangeComponent(func(component Component) bool {
		structCache := GetSaveableStruct(reflect.TypeOf(component))
		if structCache == nil {
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

// 获取对象的保存数据
func GetSaveData(obj interface{}, parentName string) (interface{}, error) {
	structCache := GetSaveableStruct(reflect.TypeOf(obj))
	if structCache == nil {
		GetLogger().Error("not saveable %v type:%v", parentName, reflect.TypeOf(obj))
		return nil, nil
	}
	objVal := reflect.ValueOf(obj).Elem()
	if structCache.IsSingleField() {
		fieldStruct := structCache.Field
		field := objVal.Field(fieldStruct.FieldIndex)
		if util.IsValueNil(field) {
			return nil, nil
		}
		// 明文保存的数据
		if fieldStruct.IsPlain {
			// 明文保存的特殊结构体需要特殊处理,如MapData[K comparable, V any]
			switch field.Kind() {
			case reflect.Ptr:
				if field.CanInterface() {
					fieldInterface := field.Interface()
					if _, ok := fieldInterface.(Saveable); ok {
						return getSaveableSaveData(fieldInterface, parentName, fieldStruct)
					}
				}
			case reflect.Struct:
				if fieldInterface := convertStructToInterface(field); fieldInterface != nil {
					if _, ok := fieldInterface.(Saveable); ok {
						return getSaveableSaveData(fieldInterface, parentName, fieldStruct)
					}
				}
			}
			fieldInterface := field.Interface()
			if _, ok := fieldInterface.(Saveable); ok {
				return getSaveableSaveData(field.Interface(), parentName, fieldStruct)
			}
			// 明文保存的普通数据,直接返回原始数据
			return fieldInterface, nil
		}
		// 非明文保存的数据,一般用于对proto进行序列化
		switch field.Kind() {
		case reflect.Map:
			return saveFieldMap(obj, field, parentName, fieldStruct)
		case reflect.Slice:
			return saveFieldSlice(obj, field, parentName, fieldStruct)
		case reflect.Ptr:
			return saveFieldPtr(obj, field, parentName, fieldStruct)
		case reflect.Struct:
			return saveFieldStruct(obj, field, parentName, fieldStruct)
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
			GetLogger().Error("%v.%v unsupported type:%v", parentName, fieldStruct.Name, field.Kind())
			return nil, ErrUnsupportedKeyType
		}
	} else {
		// 多个child子模块的组合
		compositeSaveData := make(map[string]interface{})
		for _, fieldCache := range structCache.Children {
			childName := parentName + "." + fieldCache.Name
			val := objVal.Field(fieldCache.FieldIndex)
			if util.IsValueNil(val) {
				compositeSaveData[fieldCache.Name] = nil
				continue
			}
			fieldInterface := val.Interface()
			childSaveData, err := GetSaveData(fieldInterface, childName)
			if err != nil {
				return nil, err
			}
			compositeSaveData[fieldCache.Name] = childSaveData
		}
		return compositeSaveData, nil
	}
}
