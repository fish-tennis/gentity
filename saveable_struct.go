package gentity

import (
	"reflect"
	"slices"
	"strings"
	"sync"
)

var _saveableStructsMap = newSaveableStructsMap()

// 有需要保存字段的结构
type SaveableStruct struct {
	// 单个db字段
	Field *SaveableField
	// 多个child字段
	Children []*SaveableField
}

// 是否是单个db字段
func (this *SaveableStruct) IsSingleField() bool {
	return this.Field != nil
}

// 字段
type SaveableField struct {
	StructField reflect.StructField
	FieldIndex  int
	// 是否明文保存
	IsPlain bool
	// 保存的字段名
	Name string
}

// 如果字段为nil,根据类型进行初始化
func (this *SaveableField) InitNilField(val reflect.Value) bool {
	if val.IsNil() {
		if !val.CanSet() {
			GetLogger().Error("%v CanSet false", this.Name)
			return false
		}
		if this.StructField.Type.Kind() == reflect.Slice {
			newElem := reflect.MakeSlice(this.StructField.Type, 0, 0)
			val.Set(newElem)
			GetLogger().Debug("%v MakeSlice", this.Name)
		} else if this.StructField.Type.Kind() == reflect.Map {
			newElem := reflect.MakeMap(this.StructField.Type)
			val.Set(newElem)
			GetLogger().Debug("%v MakeMap", this.Name)
		} else {
			newElem := reflect.New(this.StructField.Type)
			val.Set(newElem)
		}
	}
	return true
}

type safeSaveableStructsMap struct {
	// 是否使用全小写
	// gserver使用mongodb,默认使用全小写,以便于redis和mongodb一致
	useLowerName bool
	m            map[reflect.Type]*SaveableStruct
	// 如果在初始化的时候把所有结构缓存的话,这个读写锁是可以去掉的
	l *sync.RWMutex
}

func (s *safeSaveableStructsMap) Set(key reflect.Type, value *SaveableStruct) {
	s.l.Lock()
	defer s.l.Unlock()
	s.m[key] = value
	if value != nil {
		GetLogger().Debug("SaveableStruct: %v", key)
	}
}

func (s *safeSaveableStructsMap) Get(key reflect.Type) (*SaveableStruct, bool) {
	s.l.RLock()
	defer s.l.RUnlock()
	v, ok := s.m[key]
	return v, ok
}

func newSaveableStructsMap() *safeSaveableStructsMap {
	return &safeSaveableStructsMap{
		useLowerName: true, // 默认使用全小写
		l:            new(sync.RWMutex),
		m:            make(map[reflect.Type]*SaveableStruct),
	}
}

// 获取对象的结构描述
// 如果缓存过,则直接从缓存中获取
func GetSaveableStruct(reflectType reflect.Type) *SaveableStruct {
	if reflectType.Kind() == reflect.Ptr {
		reflectType = reflectType.Elem()
	}
	if reflectType.Kind() != reflect.Struct {
		return nil
	}
	if cacheStruct, ok := _saveableStructsMap.Get(reflectType); ok {
		return cacheStruct
	}
	newStruct := &SaveableStruct{}
	// 检查db字段
	for i := 0; i < reflectType.NumField(); i++ {
		fieldStruct := reflectType.Field(i)
		if len(fieldStruct.Tag) == 0 {
			continue
		}
		isPlain := false
		dbSetting, ok := fieldStruct.Tag.Lookup("db")
		if !ok {
			continue
		}
		// db字段只能有一个
		if newStruct.Field != nil {
			GetLogger().Error("%v.%v db field count error", reflectType.Name(), fieldStruct.Name)
			continue
		}
		dbSettings := strings.Split(dbSetting, ";")
		if slices.Contains(dbSettings, "plain") {
			isPlain = true
		}
		// 保存db的字段必须导出
		if ([]byte(fieldStruct.Name))[0] != ([]byte(strings.ToUpper(fieldStruct.Name)))[0] {
			GetLogger().Error("%v.%v field must export(start with upper char)", reflectType.Name(), fieldStruct.Name)
			continue
		}
		name := fieldStruct.Name
		if _saveableStructsMap.useLowerName {
			name = strings.ToLower(fieldStruct.Name)
		}
		for _, n := range dbSettings {
			if n != "" && n != "plain" {
				// 自动转全小写
				if _saveableStructsMap.useLowerName {
					name = strings.ToLower(n)
				} else {
					name = n
				}
				break
			}
		}
		fieldCache := &SaveableField{
			StructField: fieldStruct,
			FieldIndex:  i,
			IsPlain:     isPlain,
			Name:        name,
		}
		newStruct.Field = fieldCache
		GetLogger().Debug("db %v.%v plain:%v", reflectType.Name(), name, isPlain)
	}
	newStruct.Children = make([]*SaveableField, 0)
	// 检查child字段
	for i := 0; i < reflectType.NumField(); i++ {
		fieldStruct := reflectType.Field(i)
		if len(fieldStruct.Tag) == 0 {
			continue
		}
		dbSetting, ok := fieldStruct.Tag.Lookup("child")
		if !ok {
			continue
		}
		// db字段和child字段不共存
		if newStruct.Field != nil {
			GetLogger().Error("%v already have db field,%v cant work", reflectType.Name(), fieldStruct.Name)
			continue
		}
		// 保存db的字段必须导出
		if ([]byte(fieldStruct.Name))[0] != ([]byte(strings.ToUpper(fieldStruct.Name)))[0] {
			GetLogger().Error("%v.%v field must export(start with upper char)", reflectType.Name(), fieldStruct.Name)
			continue
		}
		// 默认使用字段名的全小写
		name := fieldStruct.Name
		if _saveableStructsMap.useLowerName {
			name = strings.ToLower(fieldStruct.Name)
		}
		dbSettings := strings.Split(dbSetting, ";")
		for _, n := range dbSettings {
			if n != "" {
				// 自动转全小写
				if _saveableStructsMap.useLowerName {
					name = strings.ToLower(n)
				} else {
					name = n
				}
				break
			}
		}
		fieldCache := &SaveableField{
			StructField: fieldStruct,
			FieldIndex:  i,
			Name:        name,
		}
		newStruct.Children = append(newStruct.Children, fieldCache)
		GetLogger().Debug("child %v.%v", reflectType.Name(), name)
		GetSaveableStruct(fieldCache.StructField.Type)
	}
	if newStruct.Field == nil && len(newStruct.Children) == 0 {
		_saveableStructsMap.Set(reflectType, nil)
		return nil
	}
	_saveableStructsMap.Set(reflectType, newStruct)
	return newStruct
}

func GetEntitySaveableStruct(entity Entity) {
	entity.RangeComponent(func(component Component) bool {
		GetSaveableStruct(reflect.TypeOf(component))
		return true
	})
}
