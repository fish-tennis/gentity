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
			InterfaceMap: make(map[string]interface{}),
		}
	})
}

// 动态数据组件
type InterfaceMap struct {
	gentity.MapComponent
	// 动态的数据结构
	InterfaceMap map[string]interface{} `db:""`
}

func (this *Player) GetInterfaceMap() *InterfaceMap {
	return this.GetComponentByName(ComponentNameInterfaceMap).(*InterfaceMap)
}

// 反序列化
func (im *InterfaceMap) LoadFromBytesMap(bytesMap any) error {
	registerValueCtor := map[string]func() interface{}{
		"item1": func() interface{} {
			return &item1{
				Data: &pb.BaseInfo{},
			}
		},
		"item2": func() interface{} {
			return &item2{
				Data: &pb.QuestData{},
			}
		},
		"item3": func() interface{} {
			return &item1{
				Data: &pb.BaseInfo{},
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
			im.InterfaceMap[k] = val
			gentity.GetLogger().Info("loadData %v %v", k, val)
		}
	}
	if len(im.InterfaceMap) == 0 {
		im.makeTestData()
	}
	return nil
}

func (im *InterfaceMap) makeTestData() {
	gentity.GetLogger().Info("makeTestData")
	i1 := &item1{
		MapValueDirtyMark: gentity.NewMapValueDirtyMark(im, "item1"),
		Data: &pb.BaseInfo{
			Level: 10086,
			Exp:   168,
		},
	}
	im.InterfaceMap["item1"] = i1
	i1.SetDirty()

	i2 := &item2{
		MapValueDirtyMark: gentity.NewMapValueDirtyMark(im, "item2"),
		Data: &pb.QuestData{
			CfgId:    120,
			Progress: 3,
		},
	}
	im.InterfaceMap["item2"] = i2
	i2.SetDirty()

	i3 := &item3{
		MapValueDirtyMark: gentity.NewMapValueDirtyMark(im, "item3"),
		Data: &pb.BaseInfo{
			Level: 3,
			Exp:   3,
		},
	}
	im.InterfaceMap["item3"] = i3
	i3.SetDirty()
}

// InterfaceMap的子对象
type item1 struct {
	*gentity.MapValueDirtyMark[string]
	Data *pb.BaseInfo `db:""`
}

func (i1 *item1) addExp(exp int32) {
	i1.Data.Exp += exp
}

// InterfaceMap的子对象
type item2 struct {
	*gentity.MapValueDirtyMark[string]
	Data *pb.QuestData `db:""`
}

// InterfaceMap的子对象
type item3 struct {
	*gentity.MapValueDirtyMark[string]
	Data *pb.BaseInfo `db:""`
}
