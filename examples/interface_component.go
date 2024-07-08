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
	registerPlayerComponentCtor(ComponentNameInterfaceMap, 100, func(player *testPlayer, playerData *pb.PlayerData) gentity.Component {
		component := &interfaceMapComponent{
			MapComponent: *gentity.NewMapComponent(player, ComponentNameInterfaceMap),
			InterfaceMap: make(map[string]interface{}),
		}
		// 该组件使用了动态结构,不能使用gentity.LoadData来自动加载数据
		// 需要自己解析出具体的数据
		component.LoadFromBytesMap(playerData.GetInterfaceMap())
		return component
	})
}

// 动态数据组件
type interfaceMapComponent struct {
	gentity.MapComponent
	// 动态的数据结构
	InterfaceMap map[string]interface{} `db:"InterfaceMap"`
}

func (this *testPlayer) GetInterfaceMap() *interfaceMapComponent {
	return this.GetComponentByName(ComponentNameInterfaceMap).(*interfaceMapComponent)
}

// 反序列化
func (im *interfaceMapComponent) LoadFromBytesMap(bytesMap any) error {
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
			err := gentity.LoadData(val, v)
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

func (im *interfaceMapComponent) makeTestData() {
	gentity.GetLogger().Info("makeTestData")
	i1 := &item1{
		Data: &pb.BaseInfo{
			Level: 10086,
			Exp:   168,
		},
	}
	i1.SetDirty()
	im.InterfaceMap["item1"] = i1

	i2 := &item2{
		Data: &pb.QuestData{
			CfgId:    120,
			Progress: 3,
		},
	}
	i2.SetDirty()
	im.InterfaceMap["item2"] = i2

	i3 := &item3{
		Data: &pb.BaseInfo{
			Level: 3,
			Exp:   3,
		},
	}
	im.InterfaceMap["item3"] = i3
}

type item1 struct {
	gentity.BaseDirtyMark
	Data *pb.BaseInfo `db:"item1"`
}

func (i1 *item1) addExp(exp int32) {
	i1.Data.Exp += exp
	i1.SetDirty()
}

type item2 struct {
	gentity.BaseDirtyMark
	Data *pb.QuestData `db:"item2"`
}

type item3 struct {
	gentity.BaseDirtyMark
	Data *pb.BaseInfo `db:"item3"`
}
