package gentity

import (
	"fmt"
	"github.com/fish-tennis/gentity/util"
	"strings"
)

// 实体接口
type Entity interface {
	// 唯一id
	GetId() int64

	AddComponent(component Component, arg any)

	// 查找某个组件
	GetComponentByName(componentName string) Component

	// 遍历组件
	RangeComponent(fun func(component Component) bool)
}

// 实体组件接口
type Component interface {
	// 组件名
	GetName() string

	// 所属的实体
	GetEntity() Entity
	SetEntity(entity Entity)
}

// 事件接口
type EventReceiver interface {
	OnEvent(event interface{})
}

type BaseEntity struct {
	// Entity唯一id
	Id int64
	// 组件表
	components []Component
}

// Entity唯一id
func (this *BaseEntity) GetId() int64 {
	return this.Id
}

// 获取组件
func (this *BaseEntity) GetComponentByName(componentName string) Component {
	for _, v := range this.components {
		if v.GetName() == componentName {
			return v
		}
	}
	return nil
}

func (this *BaseEntity) GetComponentByIndex(componentIndex int) Component {
	return this.components[componentIndex]
}

// 组件列表
func (this *BaseEntity) GetComponents() []Component {
	return this.components
}

func (this *BaseEntity) RangeComponent(fun func(component Component) bool) {
	for _, v := range this.components {
		if !fun(v) {
			return
		}
	}
}

func (this *BaseEntity) AddComponent(component Component, sourceData interface{}) {
	if len(component.GetName()) == 0 {
		GetLogger().Error("Component Name empty")
	}
	if this.GetComponentByName(component.GetName()) != nil {
		GetLogger().Error("Component Name already exist:%v", component.GetName())
		return
	}
	if sourceData != nil {
		LoadData(component, sourceData)
	}
	this.components = append(this.components, component)
}

func (this *BaseEntity) SaveCache(kvCache KvCache, cacheKeyPrefix string, entityKey interface{}) error {
	for _, component := range this.components {
		SaveComponentChangedDataToCache(kvCache, cacheKeyPrefix, entityKey, component)
	}
	return nil
}

type BaseComponent struct {
	entity Entity
	// 组件名
	name string
}

func NewBaseComponent(entity Entity, name string) *BaseComponent {
	return &BaseComponent{
		entity: entity,
		name:   name,
	}
}

// 组件名
func (this *BaseComponent) GetName() string {
	return this.name
}

func (this *BaseComponent) GetEntity() Entity {
	return this.entity
}

func (this *BaseComponent) SetEntity(entity Entity) {
	this.entity = entity
}

type DataComponent struct {
	BaseComponent
	BaseDirtyMark
}

func NewDataComponent(entity Entity, componentName string) *DataComponent {
	return &DataComponent{
		BaseComponent: BaseComponent{
			entity: entity,
			name:   componentName,
		},
	}
}

type MapDataComponent struct {
	BaseComponent
	BaseMapDirtyMark
}

func NewMapDataComponent(entity Entity, componentName string) *MapDataComponent {
	return &MapDataComponent{
		BaseComponent: BaseComponent{
			entity: entity,
			name:   componentName,
		},
	}
}

// 获取对象组件的缓存key
func GetEntityComponentCacheKey(prefix string, entityId interface{}, componentName string) string {
	// 使用{entityId}形式的hashtag,使同一个实体的不同组件的数据都落在一个redis节点上
	// 落在一个redis节点上的好处:可以使用redis function对数据进行类似事务的原子操作
	// https://redis.io/topics/cluster-tutorial
	if _saveableStructsMap.useLowerName {
		return fmt.Sprintf("%v.{%v}.%v", prefix, util.ToStringWithoutError(entityId), strings.ToLower(componentName))
	} else {
		return fmt.Sprintf("%v.{%v}.%v", prefix, util.ToStringWithoutError(entityId), componentName)
	}
}

// 获取对象组件子对象的缓存key
func GetEntityComponentChildCacheKey(prefix string, entityId interface{}, componentName string, childName string) string {
	if _saveableStructsMap.useLowerName {
		return fmt.Sprintf("%v.{%v}.%v.%v", prefix, util.ToStringWithoutError(entityId), strings.ToLower(componentName), childName)
	} else {
		return fmt.Sprintf("%v.{%v}.%v.%v", prefix, util.ToStringWithoutError(entityId), componentName, childName)
	}
}

func GetChildCacheKey(parentName, childName string) string {
	return fmt.Sprintf("%v.%v", parentName, childName)
}
