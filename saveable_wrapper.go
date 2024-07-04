package gentity

import (
	"cmp"
	"slices"
)

// map类型的数据的辅助类
type MapData[K comparable, V any] struct {
	BaseMapDirtyMark
	Data map[K]V `db:""`
}

func (md *MapData[K, V]) Init() {
	md.Data = make(map[K]V)
}

func (md *MapData[K, V]) Add(k K, v V) {
	md.Data[k] = v
	md.SetDirty(k, true)
}

func (md *MapData[K, V]) Delete(k K) {
	delete(md.Data, k)
	md.SetDirty(k, false)
}

func NewMapData[K comparable, V any]() *MapData[K, V] {
	return &MapData[K, V]{
		Data: make(map[K]V),
	}
}

// slice类型的数据的辅助类
type SliceData[E any] struct {
	BaseDirtyMark
	Data []E `db:""`
}

func (sd *SliceData[E]) Add(v ...E) {
	if len(v) == 0 {
		return
	}
	sd.Data = append(sd.Data, v...)
	sd.SetDirty()
}

// see slices.Delete
func (sd *SliceData[E]) Delete(i, j int) {
	if j > len(sd.Data) {
		j = len(sd.Data)
	}
	if i < 0 || i > j {
		return
	}
	sd.Data = slices.Delete(sd.Data, i, j)
	sd.SetDirty()
}

func Set[Field cmp.Ordered](obj DirtyMark, field *Field, value Field) {
	*field = value
	obj.SetDirty()
}

func SetFn(obj DirtyMark, setFieldValueFn func()) {
	setFieldValueFn()
	obj.SetDirty()
}

// map类型的数据的辅助接口,自动调用MapDirtyMark.SetDirty
func MapSet[M ~map[K]V, K comparable, V any](mapDirtyMark MapDirtyMark, m M, k K, v V) {
	m[k] = v
	mapDirtyMark.SetDirty(k, true)
}

// map类型的数据的辅助接口,自动调用MapDirtyMark.SetDirty
func MapDel[M ~map[K]V, K comparable, V any](mapDirtyMark MapDirtyMark, m M, k K) {
	delete(m, k)
	mapDirtyMark.SetDirty(k, false)
}
