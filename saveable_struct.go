package gentity

import (
	"github.com/fish-tennis/gentity/util"
	"reflect"
	"slices"
	"strings"
	"sync"
)

// 定义数据的关键字,允许应用层自行修改
var (
	// 单个保存字段的关键字
	KeywordDb = "db"
	// 子字段的关键字
	KeywordChild = "child"
	// 明文保存的关键字
	KeywordPlain = "plain"
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
	if util.IsValueNil(val) {
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

// 是否是map[k]any类型的map
//
//	这种类型的map,无法直接使用gentity.LoadData来加载数据,因为不知道map的value具体是什么类型
func (this *SaveableField) IsInterfaceMap() bool {
	if this.StructField.Type.Kind() != reflect.Map {
		return false
	}
	valueType := this.StructField.Type.Elem()
	return valueType.Kind() == reflect.Interface
}

// map[k]any类型的字段,new一个map[k][]byte对象
func (this *SaveableField) NewBytesMap() any {
	if !this.IsInterfaceMap() {
		GetLogger().Error("%v not a interface map", this.Name)
		return nil
	}
	keyType := this.StructField.Type.Key()
	switch keyType.Kind() {
	case reflect.Int:
		return make(map[int][]byte)
	case reflect.Int8:
		return make(map[int8][]byte)
	case reflect.Int16:
		return make(map[int16][]byte)
	case reflect.Int32:
		return make(map[int32][]byte)
	case reflect.Int64:
		return make(map[int64][]byte)
	case reflect.Uint:
		return make(map[uint][]byte)
	case reflect.Uint8:
		return make(map[uint8][]byte)
	case reflect.Uint16:
		return make(map[uint16][]byte)
	case reflect.Uint32:
		return make(map[uint32][]byte)
	case reflect.Uint64:
		return make(map[uint64][]byte)
	case reflect.Float32:
		return make(map[float32][]byte)
	case reflect.Float64:
		return make(map[float64][]byte)
	case reflect.Complex64:
		return make(map[complex64][]byte)
	case reflect.Complex128:
		return make(map[complex128][]byte)
	case reflect.String:
		return make(map[string][]byte)
	default:
		GetLogger().Error("%v unsupported key type:%v", this.Name, keyType.Kind())
		return nil
	}
}

type safeSaveableStructsMap struct {
	// 是否使用全小写,默认false
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
		if len(value.Children) == 0 {
			GetLogger().Debug("SaveableStruct: %v plain:%v", key, value.Field.IsPlain)
		} else {
			var children []string
			for _, child := range value.Children {
				children = append(children, child.Name)
			}
			GetLogger().Debug("SaveableStruct: %v children:%v", key, children)
		}
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
		useLowerName: false,
		l:            new(sync.RWMutex),
		m:            make(map[reflect.Type]*SaveableStruct),
	}
}

func GetSaveableStruct(reflectType reflect.Type) *SaveableStruct {
	return GetSaveableStructChild(reflectType, false)
}

// 获取对象的结构描述
// 如果缓存过,则直接从缓存中获取
func GetSaveableStructChild(reflectType reflect.Type, defaultPlain bool) *SaveableStruct {
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
		isPlain := defaultPlain
		dbSetting, ok := fieldStruct.Tag.Lookup(KeywordDb)
		if !ok {
			continue
		}
		// db字段只能有一个
		if newStruct.Field != nil {
			GetLogger().Error("%v.%v db field count error", reflectType.Name(), fieldStruct.Name)
			continue
		}
		dbSettings := strings.Split(dbSetting, ";")
		if slices.Contains(dbSettings, KeywordPlain) {
			isPlain = true
		}
		// 保存db的字段必须导出
		if ([]byte(fieldStruct.Name))[0] != ([]byte(strings.ToUpper(fieldStruct.Name)))[0] {
			GetLogger().Error("%v.%v field must export(start with upper char)", reflectType.Name(), fieldStruct.Name)
			continue
		}
		switch fieldStruct.Type.Kind() {
		case reflect.Interface, reflect.Func, reflect.Chan, reflect.Uintptr, reflect.UnsafePointer:
			GetLogger().Error("%v.%v db field unsupported type:%v", reflectType.Name(), fieldStruct.Name, fieldStruct.Type.Kind())
			continue
		}
		name := fieldStruct.Name
		if _saveableStructsMap.useLowerName {
			name = strings.ToLower(fieldStruct.Name)
		}
		for _, n := range dbSettings {
			if n != "" && n != KeywordPlain {
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
		dbSetting, ok := fieldStruct.Tag.Lookup(KeywordChild)
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
		name := fieldStruct.Name
		if _saveableStructsMap.useLowerName {
			name = strings.ToLower(fieldStruct.Name)
		}
		dbSettings := strings.Split(dbSetting, ";")
		isChildPlain := slices.Contains(dbSettings, KeywordPlain)
		for _, n := range dbSettings {
			if n != "" && n != KeywordPlain {
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
			IsPlain:     isChildPlain,
		}
		newStruct.Children = append(newStruct.Children, fieldCache)
		GetLogger().Debug("child %v.%v plain:%v", reflectType.Name(), name, isChildPlain)
		GetSaveableStructChild(fieldCache.StructField.Type, isChildPlain)
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
