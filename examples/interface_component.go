package examples

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/examples/pb"
)

const (
	// 组件名
	ComponentNameInterfaceMap = "InterfaceMap"
)

// 利用go的init进行组件的自动注册
func init() {
	_playerComponentRegister.Register(ComponentNameInterfaceMap, 100, func(player *Player, _ any) gentity.Component {
		return &InterfaceMap{
			MapComponent: *gentity.NewMapComponent(player, ComponentNameInterfaceMap),
			InterfaceMap: gentity.NewMapData[string, gentity.Saveable](),
		}
	})
}

// 动态数据组件
type InterfaceMap struct {
	gentity.MapComponent
	// 动态的数据结构 map[string]Saveable
	InterfaceMap *gentity.MapData[string, gentity.Saveable] `db:""`
}

func (this *Player) GetInterfaceMap() *InterfaceMap {
	return this.GetComponentByName(ComponentNameInterfaceMap).(*InterfaceMap)
}

// 反序列化
func (im *InterfaceMap) LoadFromBytesMap(bytesMap any) error {
	registerValueCtor := map[string]func() interface{}{
		"mapItem1": func() interface{} {
			return &mapItem1{
				MapValueDirtyMark: gentity.NewMapValueDirtyMark(im, "mapItem1"),
				Data:              &pb.BaseInfo{},
			}
		},
		"mapItem2": func() interface{} {
			return &mapItem2{
				MapValueDirtyMark: gentity.NewMapValueDirtyMark(im, "mapItem2"),
				Data:              &pb.QuestData{},
			}
		},
		"mapItem3": func() interface{} {
			return &mapItem1{
				MapValueDirtyMark: gentity.NewMapValueDirtyMark(im, "mapItem3"),
				Data:              &pb.BaseInfo{},
			}
		},
	}
	sourceData := bytesMap.(map[string][]byte)
	for k, v := range sourceData {
		if ctor, ok := registerValueCtor[k]; ok {
			// 动态构造
			val := ctor()
			err := gentity.LoadObjData(val, v)
			if err != nil {
				gentity.GetLogger().Error("loadDataErr %v %v", k, err.Error())
				continue
			}
			im.InterfaceMap.Set(k, val.(gentity.Saveable))
			gentity.GetLogger().Info("loadData %v %v", k, val)
		}
	}
	if len(im.InterfaceMap.Data) == 0 {
		im.makeTestData()
	}
	return nil
}

func (im *InterfaceMap) makeTestData() {
	gentity.GetLogger().Info("makeTestData")
	i1 := &mapItem1{
		MapValueDirtyMark: gentity.NewMapValueDirtyMark(im, "mapItem1"),
		Data: &pb.BaseInfo{
			Level: 10086,
			Exp:   168,
		},
	}
	im.InterfaceMap.Set("mapItem1", i1)

	i2 := &mapItem2{
		MapValueDirtyMark: gentity.NewMapValueDirtyMark(im, "mapItem2"),
		Data: &pb.QuestData{
			CfgId:    120,
			Progress: 3,
		},
	}
	im.InterfaceMap.Set("mapItem2", i2)

	i3 := &mapItem3{
		MapValueDirtyMark: gentity.NewMapValueDirtyMark(im, "mapItem3"),
		Data: &pb.BaseInfo{
			Level: 3,
			Exp:   3,
		},
	}
	im.InterfaceMap.Set("mapItem3", i3)
}

// InterfaceMap的子对象
type mapItem1 struct {
	*gentity.MapValueDirtyMark[string]
	Data *pb.BaseInfo `db:""`
}

func (i1 *mapItem1) addExp(exp int32) {
	i1.Data.Exp += exp
	i1.SetDirty()
}

// InterfaceMap的子对象
type mapItem2 struct {
	*gentity.MapValueDirtyMark[string]
	Data *pb.QuestData `db:""`
}

// InterfaceMap的子对象
type mapItem3 struct {
	*gentity.MapValueDirtyMark[string]
	Data *pb.BaseInfo `db:""`
}
