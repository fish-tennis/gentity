package gentity

import (
	"encoding/json"
	"errors"
	"github.com/fish-tennis/gentity/util"
	"google.golang.org/protobuf/proto"
	"reflect"
)

// 获取组件的保存数据
func GetComponentSaveData(component Component) (interface{}, error) {
	return GetSaveData(component, component.GetNameLower())
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
			if val.IsNil() {
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
		if val.IsNil() {
			_, err := kvCache.Del(cacheKeyName)
			if IsRedisError(err) {
				GetLogger().Error("%v cache err:%v", cacheKeyName, err.Error())
				return
			}
		} else {
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

			case reflect.Map:
				// map格式作为一个整体缓存时,需要先删除之前的数据
				_, err := kvCache.Del(cacheKeyName)
				if IsRedisError(err) {
					GetLogger().Error("%v cache err:%v", cacheKeyName, err.Error())
					return
				}
				cacheData := val.Interface()
				err = kvCache.SetMap(cacheKeyName, cacheData)
				if IsRedisError(err) {
					GetLogger().Error("%v cache err:%v", cacheKeyName, err.Error())
					return
				}

			case reflect.Slice:
				cacheData := val.Interface()
				jsonBytes, err := json.Marshal(cacheData)
				if err != nil {
					GetLogger().Error("%v json.Marshal err:%v", cacheKeyName, err.Error())
					return
				}
				err = kvCache.Set(cacheKeyName, string(jsonBytes), 0)
				if IsRedisError(err) {
					GetLogger().Error("%v cache err:%v", cacheKeyName, err.Error())
					return
				}
				GetLogger().Debug("%v json.Marshal", cacheKeyName)

			default:
				GetLogger().Error("%v cache err:unsupport kind:%v", cacheKeyName, val.Kind())
			}
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
		if val.IsNil() {
			_, err := kvCache.Del(cacheKeyName)
			if IsRedisError(err) {
				GetLogger().Error("%v cache err:%v", cacheKeyName, err.Error())
				return
			}
		} else {
			if val.Kind() != reflect.Map {
				GetLogger().Error("%v unsupport kind:%v", cacheKeyName, val.Kind())
				return
			}
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
		dirtyMark.ResetDirty()
		GetLogger().Debug("SaveCache %v", cacheKeyName)
		return
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
				changedDatas[component.GetNameLower()] = saveData
				if removeCacheAfterSaveDb {
					delKeys = append(delKeys, GetEntityComponentCacheKey(cachePrefix, entityKey, component.GetName()))
				}
				saved = append(saved, saveable)
				GetLogger().Debug("SaveDb %v %v", entityKey, component.GetName())
			}
		} else {
			reflectVal := reflect.ValueOf(component).Elem()
			for _, fieldCache := range structCache.Children {
				childName := component.GetNameLower() + "." + fieldCache.Name
				val := reflectVal.Field(fieldCache.FieldIndex)
				if val.IsNil() {
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
		componentDatas[component.GetNameLower()] = saveData
		GetLogger().Debug("GetEntitySaveData %v %v", entity.GetId(), component.GetName())
		return true
	})
}

// 获取对象的保存数据
func GetSaveData(obj interface{}, parentName string) (interface{}, error) {
	structCache := GetSaveableStruct(reflect.TypeOf(obj))
	if structCache == nil {
		GetLogger().Debug("not saveable %v", parentName)
		return nil, nil
	}
	reflectVal := reflect.ValueOf(obj).Elem()
	if structCache.IsSingleField() {
		fieldCache := structCache.Field
		val := reflectVal.Field(fieldCache.FieldIndex)
		if val.IsNil() {
			return nil, nil
		}
		fieldInterface := val.Interface()
		// 明文保存数据
		if fieldCache.IsPlain {
			return fieldInterface, nil
		}
		// 非明文保存的数据,一般用于对proto进行序列化
		switch val.Kind() {
		case reflect.Map:
			// 保存数据是一个map
			typ := reflect.TypeOf(fieldInterface)
			keyType := typ.Key()
			valType := typ.Elem()
			if valType.Kind() == reflect.Interface || valType.Kind() == reflect.Ptr {
				switch keyType.Kind() {
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					// map[int]proto.Message -> map[int64][]byte
					// map[int]interface{} -> map[int64]interface{}
					newMap := make(map[int64]interface{})
					it := val.MapRange()
					for it.Next() {
						// map的value是proto格式,进行序列化
						valueInterface := it.Value().Interface()
						if protoMessage, ok := valueInterface.(proto.Message); ok {
							bytes, err := proto.Marshal(protoMessage)
							if err != nil {
								GetLogger().Error("%v.%v proto %v err:%v", parentName, fieldCache.Name, it.Key().Int(), err.Error())
								return nil, err
							}
							newMap[it.Key().Int()] = bytes
						} else {
							// map[int]Saveable
							if valueSaveable, ok := valueInterface.(Saveable); ok {
								valueSaveData, valueSaveErr := GetSaveData(valueSaveable, parentName)
								if valueSaveErr != nil {
									GetLogger().Error("%v.%v Saveable %v err:%v", parentName, fieldCache.Name, it.Key().Int(), valueSaveErr.Error())
									return nil, valueSaveErr
								}
								newMap[it.Key().Int()] = valueSaveData
							} else {
								newMap[it.Key().Int()] = valueInterface
							}
						}
					}
					return newMap, nil
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					// map[uint]proto.Message -> map[uint64][]byte
					// map[uint]interface{} -> map[uint64]interface{}
					newMap := make(map[uint64]interface{})
					it := val.MapRange()
					for it.Next() {
						// map的value是proto格式,进行序列化
						valueInterface := it.Value().Interface()
						if protoMessage, ok := valueInterface.(proto.Message); ok {
							bytes, err := proto.Marshal(protoMessage)
							if err != nil {
								GetLogger().Error("%v.%v proto %v err:%v", parentName, fieldCache.Name, it.Key().Uint(), err.Error())
								return nil, err
							}
							newMap[it.Key().Uint()] = bytes
						} else {
							// map[uint]Saveable
							if valueSaveable, ok := valueInterface.(Saveable); ok {
								valueSaveData, valueSaveErr := GetSaveData(valueSaveable, parentName)
								if valueSaveErr != nil {
									GetLogger().Error("%v.%v Saveable %v err:%v", parentName, fieldCache.Name, it.Key().Int(), valueSaveErr.Error())
									return nil, valueSaveErr
								}
								newMap[it.Key().Uint()] = valueSaveData
							} else {
								newMap[it.Key().Uint()] = valueInterface
							}
						}
					}
					return newMap, nil
				case reflect.String:
					// map[string]proto.Message -> map[string][]byte
					// map[string]interface{} -> map[string]interface{}
					newMap := make(map[string]interface{}, val.Len())
					it := val.MapRange()
					for it.Next() {
						// map的value是proto格式,进行序列化
						valueInterface := it.Value().Interface()
						if protoMessage, ok := valueInterface.(proto.Message); ok {
							bytes, err := proto.Marshal(protoMessage)
							if err != nil {
								GetLogger().Error("%v.%v proto %v err:%v", parentName, fieldCache.Name, it.Key().String(), err.Error())
								return nil, err
							}
							newMap[it.Key().String()] = bytes
						} else {
							// map[string]Saveable
							if valueSaveable, ok := valueInterface.(Saveable); ok {
								valueSaveData, valueSaveErr := GetSaveData(valueSaveable, parentName)
								if valueSaveErr != nil {
									GetLogger().Error("%v.%v Saveable %v err:%v", parentName, fieldCache.Name, it.Key().Int(), valueSaveErr.Error())
									return nil, valueSaveErr
								}
								newMap[it.Key().String()] = valueSaveData
							} else {
								newMap[it.Key().String()] = valueInterface
							}
						}
					}
					return newMap, nil
				default:
					GetLogger().Error("%v.%v unsupport key type:%v", parentName, fieldCache.Name, keyType.Kind())
					return nil, errors.New("unsupport key type")
				}
			} else {
				// map的value是基础类型,无需序列化,直接返回
				return fieldInterface, nil
			}

		case reflect.Slice:
			typ := reflect.TypeOf(fieldInterface)
			valType := typ.Elem()
			if valType.Kind() == reflect.Interface || valType.Kind() == reflect.Ptr {
				newSlice := make([]interface{}, 0, val.Len())
				for i := 0; i < val.Len(); i++ {
					sliceElem := val.Index(i)
					valueInterface := sliceElem.Interface()
					if protoMessage, ok := valueInterface.(proto.Message); ok {
						bytes, err := proto.Marshal(protoMessage)
						if err != nil {
							GetLogger().Error("%v.%v proto %v err:%v", parentName, fieldCache.Name, i, err.Error())
							return nil, err
						}
						newSlice = append(newSlice, bytes)
					} else {
						// []Saveable
						if valueSaveable, ok := valueInterface.(Saveable); ok {
							valueSaveData, valueSaveErr := GetSaveData(valueSaveable, parentName)
							if valueSaveErr != nil {
								GetLogger().Error("%v.%v Saveable %v err:%v", parentName, fieldCache.Name, i, valueSaveErr.Error())
								return nil, valueSaveErr
							}
							newSlice = append(newSlice, valueSaveData)
						} else {
							newSlice = append(newSlice, valueInterface)
						}
					}
				}
				// proto
				return newSlice, nil
			} else {
				// slice的value是基础类型,无需序列化,直接返回
				return fieldInterface, nil
			}

		case reflect.Ptr:
			// 模块的保存数据是一个proto.Message
			// proto.Message -> []byte
			if protoMessage, ok := fieldInterface.(proto.Message); ok {
				return proto.Marshal(protoMessage)
			} else {
				// Saveable
				if valueSaveable, ok := fieldInterface.(Saveable); ok {
					valueSaveData, valueSaveErr := GetSaveData(valueSaveable, parentName)
					if valueSaveErr != nil {
						GetLogger().Error("%v.%v Saveable err:%v", parentName, fieldCache.Name, valueSaveErr.Error())
						return nil, valueSaveErr
					}
					return valueSaveData, nil
				} else {
					// TODO:扩展一个自定义序列化接口
				}
			}

		default:
			return nil, errors.New("unsupport key type")
		}
	} else {
		// 多个child子模块的组合
		compositeSaveData := make(map[string]interface{})
		for _, fieldCache := range structCache.Children {
			childName := parentName + "." + fieldCache.Name
			val := reflectVal.Field(fieldCache.FieldIndex)
			if val.IsNil() {
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
	return nil, errors.New("unsupport type")
}
