package gentity

// 保存数据的接口
// 用于检查数据是否修改过
type Saveable interface {
	// 数据是否改变过
	IsChanged() bool

	// 重置
	ResetChanged()
}

// 保存数据作为一个整体,只要一个字段修改了,整个数据都需要缓存
//
// examples:
//
//	type ProteTest struct {
//	  BaseDirtyMark
//	  Data *pb.Data `db:"ProteTest"`
//	}
//
//	type MapTest struct {
//	  BaseDirtyMark
//	  Data map[int32]*pb.Data `db:"MapTest"`
//	}
//
//	type MapTest struct {
//	  BaseDirtyMark
//	  Data map[int32]string `db:"MapTest"`
//	}
//
//	type SliceTest struct {
//	  BaseDirtyMark
//	  Data []*pb.Data `db:"SliceTest"`
//	}
type DirtyMark interface {
	// 需要保存的数据是否修改了
	IsDirty() bool
	// 设置数据修改标记
	SetDirty()
	// 重置标记
	ResetDirty()
}

// 同时需要保存数据库和缓存的接口
// Saveable:
//
//	保存数据库的频率低,比如玩家下线时才会保存数据库,那么Saveable只会在上线期间记录有没有改变过就可以
//
// DirtyMark:
//
//	缓存的保存频率高,比如玩家每一次操作都可能引起缓存的更新
type SaveableDirtyMark interface {
	Saveable
	DirtyMark
}

type BaseDirtyMark struct {
	// 数据是否修改过,用于保存数据库的数据修改标记
	isChanged bool
	// 脏数据标记,用于缓存的数据修改标记
	isDirty bool
}

// 数据是否改变过
func (this *BaseDirtyMark) IsChanged() bool {
	return this.isChanged
}

func (this *BaseDirtyMark) ResetChanged() {
	this.isChanged = false
}

func (this *BaseDirtyMark) IsDirty() bool {
	return this.isDirty
}

func (this *BaseDirtyMark) SetDirty() {
	this.isDirty = true
	this.isChanged = true
}

func (this *BaseDirtyMark) ResetDirty() {
	this.isDirty = false
}

// map格式的保存数据
// 第一次有数据修改时,会把整体数据缓存一次,之后只保存修改过的项(增量更新)
//
// examples:
//
//	type MapTest struct {
//	  BaseMapDirtyMark
//	  Data map[int32]*pb.Data `db:"MapTest"`
//	}
//
//	type MapTest struct {
//	  BaseMapDirtyMark
//	  Data map[int32]string `db:"MapTest"`
//	}
type MapDirtyMark interface {
	// 需要保存的数据是否修改了
	IsDirty() bool
	// 设置数据修改标记
	SetDirty(k interface{}, isAddOrUpdate bool)
	// 重置标记
	ResetDirty()

	// 是否把整体数据缓存过了
	HasCached() bool
	// 第一次有数据修改时,会把整体数据缓存一次,之后只保存修改过的项(增量更新)
	SetCached()

	RangeDirtyMap(f func(dirtyKey interface{}, isAddOrUpdate bool))
}

type BaseMapDirtyMark struct {
	isChanged bool
	hasCached bool
	dirtyMap  map[interface{}]bool
}

func (this *BaseMapDirtyMark) IsChanged() bool {
	return this.isChanged
}

func (this *BaseMapDirtyMark) ResetChanged() {
	this.isChanged = false
}

func (this *BaseMapDirtyMark) IsDirty() bool {
	return len(this.dirtyMap) > 0
}

func (this *BaseMapDirtyMark) SetDirty(k interface{}, isAddOrUpdate bool) {
	if this.dirtyMap == nil {
		this.dirtyMap = make(map[interface{}]bool)
	}
	this.dirtyMap[k] = isAddOrUpdate
	this.isChanged = true
}

func (this *BaseMapDirtyMark) ResetDirty() {
	this.dirtyMap = make(map[interface{}]bool)
}

func (this *BaseMapDirtyMark) HasCached() bool {
	return this.hasCached
}

func (this *BaseMapDirtyMark) SetCached() {
	this.hasCached = true
}

func (this *BaseMapDirtyMark) RangeDirtyMap(f func(dirtyKey interface{}, isAddOrUpdate bool)) {
	for k, v := range this.dirtyMap {
		f(k, v)
	}
}

// 用于InterfaceMap的map's value的DirtyMark
type MapValueDirtyMark[K comparable] struct {
	// 父类才是真正的脏标记
	Parent MapDirtyMark
	MapKey K // key of parent map
}

func NewMapValueDirtyMark[K comparable](parent MapDirtyMark, mapKey K) *MapValueDirtyMark[K] {
	return &MapValueDirtyMark[K]{
		Parent: parent,
		MapKey: mapKey,
	}
}

// 只是为了实现Saveable接口,无实际作用
func (m *MapValueDirtyMark[K]) IsChanged() bool {
	return false
}

// 只是为了实现Saveable接口,无实际作用
func (m *MapValueDirtyMark[K]) ResetChanged() {
}

// 设置脏标记,实际设置的是父类
func (m *MapValueDirtyMark[K]) SetDirty() {
	if m.Parent == nil {
		return
	}
	m.Parent.SetDirty(m.MapKey, true)
}
