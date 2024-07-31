package examples

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/examples/pb"
	"math"
)

const (
	// 组件名
	ComponentNameBag = "Bag"
)

// 利用go的init进行组件的自动注册
func init() {
	_playerComponentRegister.Register(ComponentNameBag, 0, func(player *Player, _ any) gentity.Component {
		component := &Bag{
			BaseComponent:  *gentity.NewBaseComponent(player, ComponentNameBag),
			BagCountItem:   new(BagCountItem),
			BagUniqueItem:  new(BagUniqueItem),
			TestUniqueItem: new(gentity.SliceData[*pb.UniqueItem]),
		}
		component.BagCountItem.Init()
		component.BagUniqueItem.Init()
		return component
	})
}

// 背包模块
// 演示通过组合模式,整合多个不同的子背包模块,提供更高一级的背包接口
type Bag struct {
	gentity.BaseComponent
	BagCountItem  *BagCountItem  `child:"CountItem"`
	BagUniqueItem *BagUniqueItem `child:"UniqueItem"`
	// 假设一种特殊背包,用于测试gentity.SliceData
	TestUniqueItem *gentity.SliceData[*pb.UniqueItem] `child:"TestUniqueItem"`
}

func (this *Player) GetBag() *Bag {
	return this.GetComponentByName(ComponentNameBag).(*Bag)
}

// 有数量的物品背包
type BagCountItem struct {
	gentity.MapData[int32, int32] `db:"plain"`
}

func (this *BagCountItem) AddItem(itemCfgId, addCount int32) int32 {
	if addCount <= 0 {
		return 0
	}
	curCount, ok := this.Data[itemCfgId]
	if ok {
		// 检查数值溢出
		if int64(curCount)+int64(addCount) > math.MaxInt32 {
			addCount = math.MaxInt32 - curCount
			curCount = math.MaxInt32
		} else {
			curCount += addCount
		}
	} else {
		curCount = addCount
	}
	this.Set(itemCfgId, curCount)
	gentity.GetLogger().Debug("AddItem cfgId:%v curCount:%v addCount:%v", itemCfgId, curCount, addCount)
	return addCount
}

func (this *BagCountItem) DelItem(itemCfgId, delCount int32) int32 {
	if delCount <= 0 {
		return 0
	}
	curCount, ok := this.Data[itemCfgId]
	if !ok {
		return 0
	}
	if delCount >= curCount {
		this.Delete(itemCfgId)
		gentity.GetLogger().Debug("DelItem cfgId:%v delCount:%v/%v", itemCfgId, curCount, delCount)
		return curCount
	} else {
		this.Set(itemCfgId, curCount-delCount)
		gentity.GetLogger().Debug("DelItem cfgId:%v delCount:%v", itemCfgId, delCount)
		return delCount
	}
}

// 不可叠加的物品背包
type BagUniqueItem struct {
	gentity.MapData[int64, *pb.UniqueItem] `db:""`
}

func (this *BagUniqueItem) AddUniqueItem(uniqueItem *pb.UniqueItem) int32 {
	if _, ok := this.Data[uniqueItem.UniqueId]; !ok {
		this.Set(uniqueItem.UniqueId, uniqueItem)
		gentity.GetLogger().Debug("AddUniqueItem CfgId:%v UniqueId:%v", uniqueItem.CfgId, uniqueItem.UniqueId)
		return 1
	}
	return 0
}

func (this *BagUniqueItem) DelUniqueItem(uniqueId int64) int32 {
	if _, ok := this.Data[uniqueId]; ok {
		this.Delete(uniqueId)
		gentity.GetLogger().Debug("DelUniqueItem UniqueId:%v", uniqueId)
		return 1
	}
	return 0
}
