package gentity

import (
	"fmt"
	"strings"
)

// 实体接口
type Entity interface {
	// 唯一id
	GetId() int64

	// 查找某个组件
	GetComponentByName(componentName string) Component

	// 遍历组件
	RangeComponent(fun func(component Component) bool)
}

// 实体组件接口
type Component interface {
	// 组件名
	GetName() string
	GetNameLower() string

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
	for _,v := range this.components {
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
	for _,v := range this.components {
		if !fun(v) {
			return
		}
	}
}

func (this *BaseEntity) AddComponent(component Component, sourceData interface{}) {
	if len(component.GetName()) == 0 {
		GetLogger().Error("Component Name empty")
	}
	if sourceData != nil {
		LoadData(component, sourceData)
	}
	this.components = append(this.components, component)
}

func (this *BaseEntity) SaveCache(kvCache KvCache) error {
	for _, component := range this.components {
		SaveComponentChangedDataToCache(kvCache, component)
	}
	return nil
}

// 分发事件
func (this *BaseEntity) FireEvent(event interface{}) {
	this.RangeComponent(func(component Component) bool {
		if eventReceiver, ok := component.(EventReceiver); ok {
			eventReceiver.OnEvent(event)
		}
		return true
	})
}

type BaseComponent struct {
	entity Entity
	// 组件名
	name string
}

func NewBaseComponent(entity Entity, name string) *BaseComponent {
	return &BaseComponent{
		entity: entity,
		name: name,
	}
}

// 组件名
func (this *BaseComponent) GetName() string {
	return this.name
}

func (this *BaseComponent) GetNameLower() string {
	return strings.ToLower(this.name)
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

// 获取玩家组件的缓存key
func GetPlayerComponentCacheKey(playerId int64, componentName string) string {
	// 使用{playerId}形式的hashtag,使同一个玩家的不同组件的数据都落在一个redis节点上
	// 落在一个redis节点上的好处:可以使用redis lua对玩家数据进行类似事务的原子操作
	// https://redis.io/topics/cluster-tutorial
	return fmt.Sprintf("p.{%v}.%v", playerId, strings.ToLower(componentName) )
}

// 获取玩家组件子对象的缓存key
func GetPlayerComponentChildCacheKey(playerId int64, componentName string, childName string) string {
	return fmt.Sprintf("p.{%v}.%v.%v", playerId, strings.ToLower(componentName), childName)
}

// 获取对象组件的缓存key
func GetEntityComponentCacheKey(prefix string, entityId int64, componentName string) string {
	// 使用{entityId}形式的hashtag,使同一个实体的不同组件的数据都落在一个redis节点上
	// 落在一个redis节点上的好处:可以使用redis lua对数据进行类似事务的原子操作
	// https://redis.io/topics/cluster-tutorial
	return fmt.Sprintf("%v.{%v}.%v", prefix, entityId, strings.ToLower(componentName))
}

// 获取对象组件子对象的缓存key
func GetEntityComponentChildCacheKey(prefix string, entityId int64, componentName string, childName string) string {
	return fmt.Sprintf("%v.{%v}.%v.%v", prefix, entityId, strings.ToLower(componentName), childName)
}

func GetChildCacheKey(parentName,childName string) string {
	return fmt.Sprintf("%v.%v", parentName, childName)
}