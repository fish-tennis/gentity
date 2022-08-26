package examples

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/examples/pb"
)

// 基本信息组件
type baseInfoComponent struct {
	gentity.DataComponent
	BaseInfo *pb.BaseInfo `db:"plain"`
}

func (this *baseInfoComponent) AddExp(exp int32) {
	this.BaseInfo.Exp += exp
	this.SetDirty()
}