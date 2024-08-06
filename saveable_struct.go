package gentity

import (
	"github.com/fish-tennis/gentity/util"
	"google.golang.org/protobuf/proto"
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
// SaveableStruct应该只针对第一层的对象(如Component),并设计为树型结构,在第一次解析结构时,就把层次关系记录下来
type SaveableStruct struct {
	// 单个db字段
	Field *SaveableField
	// 多个child字段
	Children []*SaveableField
	// 父节点
	ParentField *SaveableField
}

// 是否是单个db字段
func (this *SaveableStruct) IsSingleField() bool {
	return this.Field != nil
}

func (this *SaveableStruct) GetSingleSaveable(obj any) (Saveable, *SaveableField) {
	if this.Field == nil {
		return nil, nil
	}
	if this.Field.SaveableStruct == nil {
		/*
			type BaseSaveable struct {
				BaseDirtyMark
				Field *pb.Xxx `db:""`
			}
			type BaseSaveable struct {
				BaseDirtyMark
				*pb.Xxx `db:""`
			}
		*/
		if saveable, ok := obj.(Saveable); ok {
			return saveable, this.Field
		}
		objVal := reflect.ValueOf(obj)
		if objVal.Kind() == reflect.Struct {
			objPtr := convertStructToInterface(objVal)
			if saveable, ok := objPtr.(Saveable); ok {
				return saveable, this.Field
			}
		}
		GetLogger().Error("GetSingleSaveable obj not Saveable:%v", reflect.TypeOf(obj).String())
		return nil, nil
	}
	objVal := reflect.ValueOf(obj)
	if objVal.Kind() == reflect.Ptr {
		objVal = objVal.Elem()
	}
	fieldVal := objVal.Field(this.Field.FieldIndex)
	// TODO: load时,initNilField
	if !fieldVal.CanInterface() {
		GetLogger().Error("GetSingleSaveable field CantInterface:%v", this.Field.Name)
		return nil, nil
	}
	var fieldInterface any
	if fieldVal.Kind() == reflect.Struct {
		fieldInterface = convertStructToInterface(fieldVal)
	} else {
		fieldInterface = fieldVal.Interface()
	}
	if fieldInterface == nil {
		GetLogger().Error("GetSingleSaveable field nil :%v", this.Field.Name)
		return nil, nil
	}
	// 查找下一层
	return this.Field.SaveableStruct.GetSingleSaveable(fieldInterface)
}

func (this *SaveableStruct) GetChildSaveable(obj any, childIndex int) (Saveable, *SaveableField) {
	if childIndex < 0 || childIndex >= len(this.Children) {
		return nil, nil
	}
	saveableField := this.Children[childIndex]
	objVal := reflect.ValueOf(obj)
	if objVal.Kind() == reflect.Ptr {
		objVal = objVal.Elem()
	}
	fieldVal := objVal.Field(saveableField.FieldIndex)
	// TODO: load时,initNilField
	if !fieldVal.CanInterface() {
		GetLogger().Error("GetChildSaveable field CantInterface:%v", saveableField.Name)
		return nil, nil
	}
	var fieldInterface any
	if fieldVal.Kind() == reflect.Struct {
		fieldInterface = convertStructToInterface(fieldVal)
	} else {
		fieldInterface = fieldVal.Interface()
	}
	if fieldInterface == nil {
		GetLogger().Error("GetChildSaveable field nil :%v", saveableField.Name)
		return nil, nil
	}
	if saveableField.SaveableStruct == nil {
		if saveable, ok := fieldInterface.(Saveable); ok {
			return saveable, saveableField
		}
		GetLogger().Error("GetChildSaveable field not Saveable:%v", reflect.TypeOf(fieldInterface).String())
		return nil, nil
	}
	// 查找下一层
	return saveableField.SaveableStruct.GetSingleSaveable(fieldInterface)
}

// 字段
type SaveableField struct {
	StructField reflect.StructField
	// 如果该字段不是叶子节点,则SaveableStruct有值
	SaveableStruct *SaveableStruct
	FieldIndex     int
	// 是否明文保存
	IsPlain bool
	// 保存的字段名
	Name string
	// 节点深度
	Depth int32
	// 是否是map[key]any类型
	isInterfaceMap bool
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

func (this *SaveableField) IsInterfaceMap() bool {
	return this.isInterfaceMap
}

// 是否是map[k]any类型的map
//
//	这种类型的map,无法直接使用gentity.LoadData来加载数据,因为不知道map的value具体是什么类型
func (this *SaveableField) checkInterfaceMap() {
	typ := this.StructField.Type
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Map {
		return
	}
	valueType := typ.Elem()
	this.isInterfaceMap = valueType.Kind() == reflect.Interface
	if this.isInterfaceMap {
		GetLogger().Debug("%v.%v isInterfaceMap depth:%v", this.Name, this.StructField.Name, this.Depth)
	}
}

// 获取最后一层的字段
func (this *SaveableField) getLeafField() *SaveableField {
	field := this
	subStruct := this.SaveableStruct
	// 限制子结构只能是单字段
	for subStruct != nil && subStruct.Field != nil {
		field = subStruct.Field
		subStruct = field.SaveableStruct
	}
	return field
}

func (this *SaveableField) hasSubInterfaceMap() bool {
	return this.getLeafField().isInterfaceMap
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
		if key.Kind() == reflect.Ptr {
			key = key.Elem()
		}
		if len(value.Children) == 0 {
			GetLogger().Info("SaveableStruct: %v.%v plain:%v", key.Name(), value.Field.StructField.Name, value.Field.IsPlain)
		} else {
			var children []string
			for _, child := range value.Children {
				children = append(children, child.Name)
			}
			GetLogger().Info("SaveableStruct: %v children:%v", key.Name(), children)
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

func isSupportedSaveableField(fieldStruct reflect.Type) bool {
	switch fieldStruct.Kind() {
	case reflect.Interface, reflect.Func, reflect.Chan, reflect.Uintptr, reflect.UnsafePointer:
		return false
	default:
		return true
	}
}

func parseField(rootObj any, newStruct *SaveableStruct, fieldStruct reflect.StructField, fieldIndex int,
	tagKeyword string, parentField *SaveableField) *SaveableField {
	if len(fieldStruct.Tag) == 0 {
		return nil
	}
	dbSetting, ok := fieldStruct.Tag.Lookup(tagKeyword)
	if !ok {
		return nil
	}
	// db字段只能有一个
	if newStruct.Field != nil {
		if tagKeyword == KeywordDb {
			GetLogger().Error("%v %v db field count error", getObjOrComponentName(rootObj), fieldStruct.Name)
		} else {
			GetLogger().Error("%v already have db field,%v cant work", getObjOrComponentName(rootObj), fieldStruct.Name)
		}
		return nil
	}
	// 保存db的字段必须导出
	if ([]byte(fieldStruct.Name))[0] != ([]byte(strings.ToUpper(fieldStruct.Name)))[0] {
		GetLogger().Error("%v %v field must export(start with upper char)", getObjOrComponentName(rootObj), fieldStruct.Name)
		return nil
	}
	if !isSupportedSaveableField(fieldStruct.Type) {
		GetLogger().Error("%v %v db field unsupported type:%v", getObjOrComponentName(rootObj), fieldStruct.Name, fieldStruct.Type.Kind())
		return nil
	}
	isPlain := false
	name := ""
	depth := int32(0)
	// 保存名和明文保存方式,只在第一层字段有效
	if parentField == nil {
		dbSettings := strings.Split(dbSetting, ";")
		if slices.Contains(dbSettings, KeywordPlain) {
			isPlain = true
		}
		component, isComponent := rootObj.(Component)
		if tagKeyword == KeywordDb && isComponent {
			// 组件的单保存字段,强制使用组件名
			name = component.GetName()
			if _saveableStructsMap.useLowerName {
				name = strings.ToLower(name)
			}
		} else {
			// child字段或者非组件的db字段,才会使用struct tag里的名字
			name = fieldStruct.Name
			if _saveableStructsMap.useLowerName {
				name = strings.ToLower(name)
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
		}
	} else {
		isPlain = parentField.IsPlain
		name = parentField.Name
		depth = parentField.Depth + 1
	}
	saveableField := &SaveableField{
		StructField: fieldStruct,
		FieldIndex:  fieldIndex,
		IsPlain:     isPlain,
		Name:        name,
		Depth:       depth,
	}
	fieldPtrTyp := fieldStruct.Type
	fieldTyp := fieldStruct.Type
	if fieldTyp.Kind() == reflect.Pointer {
		fieldTyp = fieldTyp.Elem()
	}
	if fieldTyp.Kind() != reflect.Struct {
		if tagKeyword == KeywordDb {
			GetLogger().Debug("parseField %v field:%v fieldType:%v depth:%v", getObjOrComponentName(rootObj), fieldStruct.Name, fieldTyp.String(), depth)
		} else {
			GetLogger().Debug("parseField %v.%v field:%v fieldType:%v depth:%v", getObjOrComponentName(rootObj), name, fieldStruct.Name, fieldTyp.String(), depth)
		}
		saveableField.checkInterfaceMap()
		return saveableField
	}
	// 如果fieldTyp是proto.Message,则直接返回
	if fieldPtrTyp.Implements(reflect.TypeOf((*proto.Message)(nil)).Elem()) {
		if tagKeyword == KeywordDb {
			GetLogger().Debug("parseField %v field:%v fieldType:%v depth:%v", getObjOrComponentName(rootObj), fieldStruct.Name, fieldTyp.String(), depth)
		} else {
			GetLogger().Debug("parseField %v.%v field:%v fieldType:%v depth:%v", getObjOrComponentName(rootObj), name, fieldStruct.Name, fieldTyp.String(), depth)
		}
		saveableField.checkInterfaceMap()
		return saveableField
	}
	// 字段是struct,则继续解析下一层
	subStruct := &SaveableStruct{
		ParentField: parentField,
	}
	subStruct = parseStruct(rootObj, fieldTyp, subStruct, saveableField)
	saveableField.SaveableStruct = subStruct
	saveableField.checkInterfaceMap()
	return saveableField
}

func getObjOrComponentName(obj any) string {
	if component, ok := obj.(Component); ok {
		return component.GetName()
	}
	return reflect.TypeOf(obj).String()
}

func parseStruct(rootObj any, structTyp reflect.Type, newStruct *SaveableStruct, parentField *SaveableField) *SaveableStruct {
	// 检查db字段
	for i := 0; i < structTyp.NumField(); i++ {
		fieldStruct := structTyp.Field(i)
		saveableField := parseField(rootObj, newStruct, fieldStruct, i, KeywordDb, parentField)
		if saveableField == nil {
			continue
		}
		newStruct.Field = saveableField
		if parentField == nil {
			GetLogger().Debug("db %v.%v plain:%v", getObjOrComponentName(rootObj), saveableField.StructField.Name, saveableField.IsPlain)
		}
	}
	// child关键字只能用在第1层字段
	// 第2层开始,只能用db关键字(单保存字段)
	// 防止结构太复杂
	if parentField == nil {
		newStruct.Children = make([]*SaveableField, 0)
		// 检查child字段
		for i := 0; i < structTyp.NumField(); i++ {
			fieldStruct := structTyp.Field(i)
			saveableField := parseField(rootObj, newStruct, fieldStruct, i, KeywordChild, parentField)
			if saveableField == nil {
				continue
			}
			newStruct.Children = append(newStruct.Children, saveableField)
			GetLogger().Debug("child %v.%v plain:%v", structTyp.Name(), saveableField.Name, saveableField.IsPlain)
		}
	}
	if newStruct.Field == nil && len(newStruct.Children) == 0 {
		return nil
	}
	return newStruct
}

func ParseEntitySaveableStruct(entity Entity) {
	entity.RangeComponent(func(component Component) bool {
		GetObjSaveableStruct(component)
		return true
	})
}

// 获取对象的保存结构(一般对组件使用),如果没有保存字段,则返回nil
func GetObjSaveableStruct(obj any) *SaveableStruct {
	objTyp := reflect.TypeOf(obj)
	if objTyp.Kind() == reflect.Ptr {
		objTyp = objTyp.Elem()
	}
	if objTyp.Kind() != reflect.Struct {
		return nil
	}
	if cacheStruct, ok := _saveableStructsMap.Get(objTyp); ok {
		return cacheStruct
	}
	objStruct := &SaveableStruct{}
	objStruct = parseStruct(obj, objTyp, objStruct, nil)
	if objStruct != nil {
		// 如果叶子节点是map[key]any,则把第一层field也标记为InterfaceMap
		if objStruct.Field != nil {
			if objStruct.Field.getLeafField().isInterfaceMap {
				objStruct.Field.isInterfaceMap = true
			}
		} else {
			for _, child := range objStruct.Children {
				if child.getLeafField().isInterfaceMap {
					child.isInterfaceMap = true
				}
			}
		}
	}
	_saveableStructsMap.Set(objTyp, objStruct)
	return objStruct
}
