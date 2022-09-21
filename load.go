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

// 加载数据
// 反序列化
func LoadData(obj interface{}, sourceData interface{}) error {
	if util.IsNil(sourceData) {
		return nil
	}
	structCache := GetSaveableStruct(reflect.TypeOf(obj))
	if structCache == nil {
		return errors.New("not Saveable object")
	}
	reflectVal := reflect.ValueOf(obj).Elem()
	if structCache.IsSingleField() {
		fieldCache := structCache.Field
		val := reflectVal.Field(fieldCache.FieldIndex)
		if val.IsNil() {
			if !val.CanSet() {
				GetLogger().Error("%v CanSet false", fieldCache.Name)
				return nil
			}
			newElem := reflect.New(fieldCache.StructField.Type)
			val.Set(newElem)
		}
		sourceTyp := reflect.TypeOf(sourceData)
		switch sourceTyp.Kind() {
		case reflect.Slice:
			if !fieldCache.IsPlain {
				// proto反序列化
				// []byte -> proto.Message
				if sourceTyp.Elem().Kind() == reflect.Uint8 {
					if bytes, ok := sourceData.([]byte); ok {
						if len(bytes) == 0 {
							return nil
						}
						// []byte -> proto.Message
						if protoMessage, ok2 := val.Interface().(proto.Message); ok2 {
							err := proto.Unmarshal(bytes, protoMessage)
							if err != nil {
								GetLogger().Error("%v proto.Unmarshal err:%v", fieldCache.Name, err.Error())
								return err
							}
							return nil
						}
					}
				}
			}
			// 基础类型的slice
			if fieldCache.StructField.Type.Kind() == reflect.Slice {
				// 数组类型一致,就直接赋值
				if sourceTyp.Elem().Kind() == fieldCache.StructField.Type.Elem().Kind() {
					if val.CanSet() {
						val.Set(reflect.ValueOf(sourceData))
						return nil
					}
				}
				// 类型不一致,暂时返回错误
				return errors.New("slice element type error")
			}

		case reflect.Map:
			// map[intORstring]intORstring -> map[intORstring]intORstring
			// map[intORstring][]byte -> map[intORstring]proto.Message
			sourceVal := reflect.ValueOf(sourceData)
			sourceKeyType := sourceTyp.Key()
			sourceValType := sourceTyp.Elem()
			if fieldCache.StructField.Type.Kind() == reflect.Map {
				//fieldVal := reflect.ValueOf(val)
				keyType := fieldCache.StructField.Type.Key()
				valType := fieldCache.StructField.Type.Elem()
				sourceIt := sourceVal.MapRange()
				for sourceIt.Next() {
					k := ConvertValueToInterface(sourceKeyType, keyType, sourceIt.Key())
					v := ConvertValueToInterface(sourceValType, valType, sourceIt.Value())
					val.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(v))
				}
				return nil
			}

		case reflect.Interface, reflect.Ptr:
			sourceVal := reflect.ValueOf(sourceData)
			//if fieldCache.StructField.Type == sourceTyp {
			//	if val.CanSet() {
			//		val.Set(sourceVal)
			//		return nil
			//	}
			//}
			if sourceProtoMessage, ok := sourceVal.Interface().(proto.Message); ok {
				if protoMessage, ok2 := val.Interface().(proto.Message); ok2 {
					if sourceProtoMessage.ProtoReflect().Descriptor() == protoMessage.ProtoReflect().Descriptor() {
						proto.Merge(protoMessage, sourceProtoMessage)
						return nil
					}
				}
			}
			// TODO:扩展一个序列化接口
			return errors.New(fmt.Sprintf("unsupport type %v", fieldCache.Name))

		default:
			return errors.New(fmt.Sprintf("unsupport type %v sourceTyp.Kind():%v", fieldCache.Name, sourceTyp.Kind()))

		}
	} else {
		sourceTyp := reflect.TypeOf(sourceData)
		// 如果是proto,先转换成map
		if sourceTyp.Kind() == reflect.Ptr {
			protoMessage, ok := sourceData.(proto.Message)
			if !ok {
				GetLogger().Error("unsupport type:%v", sourceTyp.Kind())
				return errors.New(fmt.Sprintf("unsupport type:%v", sourceTyp.Kind()))
			}
			// mongodb中读出来是proto.Message格式,转换成map[string]interface{}
			sourceData = ConvertProtoToMap(protoMessage)
			sourceTyp = reflect.TypeOf(sourceData)
		}
		if sourceTyp.Kind() != reflect.Map {
			GetLogger().Error("unsupport type:%v", sourceTyp.Kind())
			return errors.New("sourceData type error")
		}
		sourceVal := reflect.ValueOf(sourceData)
		for _, fieldCache := range structCache.Children {
			sourceFieldVal := sourceVal.MapIndex(reflect.ValueOf(fieldCache.Name))
			if !sourceFieldVal.IsValid() {
				GetLogger().Debug("saveable not exists:%v", fieldCache.Name)
				continue
			}
			val := reflectVal.Field(fieldCache.FieldIndex)
			if val.IsNil() {
				if !val.CanSet() {
					GetLogger().Error("child cant new field:%v", fieldCache.Name)
					continue
				}
				newElem := reflect.New(fieldCache.StructField.Type)
				val.Set(newElem)
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

// 从缓存中恢复数据
func LoadFromCache(obj interface{}, kvCache KvCache, cacheKey string) (bool, error) {
	structCache := GetSaveableStruct(reflect.TypeOf(obj))
	if structCache == nil {
		return false, nil
	}
	cacheType, err := kvCache.Type(cacheKey)
	if err == redis.Nil || cacheType == "" || cacheType == "none" {
		return false, nil
	}
	reflectVal := reflect.ValueOf(obj).Elem()
	if structCache.IsSingleField() {
		fieldCache := structCache.Field
		val := reflectVal.Field(fieldCache.FieldIndex)
		if cacheType == "string" {
			if fieldCache.StructField.Type.Kind() == reflect.Ptr || fieldCache.StructField.Type.Kind() == reflect.Interface {
				if val.IsNil() {
					if !val.CanSet() {
						GetLogger().Error("%v CanSet false", fieldCache.Name)
						return true, errors.New(fmt.Sprintf("%v CanSet false", fieldCache.Name))
					}
					newElem := reflect.New(fieldCache.StructField.Type)
					val.Set(newElem)
					GetLogger().Debug("cacheKey:%v new %v", cacheKey, fieldCache.Name)
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
			} else if fieldCache.StructField.Type.Kind() == reflect.Slice {
				if val.IsNil() {
					if !val.CanSet() {
						GetLogger().Error("%v CanSet false", fieldCache.Name)
						return true, errors.New(fmt.Sprintf("%v CanSet false", fieldCache.Name))
					}
					newElem := reflect.New(fieldCache.StructField.Type)
					val.Set(newElem)
					GetLogger().Debug("cacheKey:%v new %v", cacheKey, fieldCache.Name)
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
			if val.IsNil() {
				if !val.CanSet() {
					GetLogger().Error("%v CanSet false", fieldCache.Name)
					return true, errors.New(fmt.Sprintf("%v CanSet false", fieldCache.Name))
				}
				newElem := reflect.New(fieldCache.StructField.Type)
				val.Set(newElem)
				GetLogger().Debug("cacheKey:%v new %v", cacheKey, fieldCache.Name)
			}
			if !val.CanInterface() {
				return true, errors.New(fmt.Sprintf("%v CanInterface false", fieldCache.Name))
			}
			// hash -> map
			err = kvCache.GetMap(cacheKey, val.Interface())
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
			if val.IsNil() {
				if !val.CanSet() {
					GetLogger().Error("%v CanSet false", fieldCache.Name)
					return true, errors.New(fmt.Sprintf("%v CanSet false", fieldCache.Name))
				}
				newElem := reflect.New(fieldCache.StructField.Type)
				val.Set(newElem)
				GetLogger().Debug("cacheKey:%v new %v", cacheKey, fieldCache.Name)
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
func FixEntityDataFromCache(entity Entity, db EntityDb, kvCache KvCache, cacheKeyPrefix string) {
	entity.RangeComponent(func(component Component) bool {
		structCache := GetSaveableStruct(reflect.TypeOf(component))
		if structCache == nil {
			return true
		}
		if structCache.IsSingleField() {
			cacheKey := GetEntityComponentCacheKey(cacheKeyPrefix, entity.GetId(), component.GetName())
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
				GetLogger().Error("%v Save %v err %v", entity.GetId(), component.GetName(), err.Error())
				return true
			}
			saveDbErr := db.SaveComponent(entity.GetId(), component.GetNameLower(), saveData)
			if saveDbErr != nil {
				GetLogger().Error("%v SaveDb %v err %v", entity.GetId(), component.GetNameLower(), saveDbErr.Error())
				return true
			}
			GetLogger().Info("%v -> %v", cacheKey, component.GetNameLower())
			kvCache.Del(cacheKey)
			GetLogger().Info("RemoveCache %v", cacheKey)
		} else {
			reflectVal := reflect.ValueOf(component).Elem()
			for _, fieldCache := range structCache.Children {
				val := reflectVal.Field(fieldCache.FieldIndex)
				if val.IsNil() {
					if !val.CanSet() {
						GetLogger().Error("%v CanSet false", fieldCache.Name)
						return true
					}
					newElem := reflect.New(fieldCache.StructField.Type)
					val.Set(newElem)
					GetLogger().Debug("new %v", fieldCache.Name)
				}
				fieldInterface := val.Interface()
				cacheKey := GetPlayerComponentChildCacheKey(entity.GetId(), component.GetName(), fieldCache.Name)
				hasCache, err := LoadFromCache(fieldInterface, kvCache, cacheKey)
				if !hasCache {
					return true
				}
				if err != nil {
					GetLogger().Error("LoadFromCache %v error:%v", cacheKey, err.Error())
					return true
				}
				GetLogger().Debug("%v", fieldInterface)
				saveData, err := GetSaveData(fieldInterface, component.GetNameLower())
				if err != nil {
					GetLogger().Error("%v Save %v.%v err %v", entity.GetId(), component.GetName(), fieldCache.Name, err.Error())
					return true
				}
				GetLogger().Debug("%v", saveData)
				saveDbErr := db.SaveComponentField(entity.GetId(), component.GetNameLower(), fieldCache.Name, saveData)
				if saveDbErr != nil {
					GetLogger().Error("%v SaveDb %v.%v err %v", entity.GetId(), component.GetNameLower(), fieldCache.Name, saveDbErr.Error())
					return true
				}
				GetLogger().Info("%v -> %v.%v", cacheKey, component.GetNameLower(), fieldCache.Name)
				kvCache.Del(cacheKey)
				GetLogger().Info("RemoveCache %v", cacheKey)
			}
		}
		return true
	})
}