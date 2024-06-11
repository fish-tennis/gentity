package gentity

import (
	"cmp"
	"slices"
)

// 组件构造接口
type ComponentCtor[E Entity] func(entity E, arg any) Component

// 组件注册信息
type ComponentRegisterInfo[E Entity] struct {
	// 组件名
	ComponentName string
	// 组件构造接口
	Ctor ComponentCtor[E]
	// 构造顺序,数值小的组件,先执行
	// 因为有的组件有依赖关系
	CtorOrder int
}

// 组件注册信息管理
type ComponentRegister[E Entity] struct {
	RegisterInfos []*ComponentRegisterInfo[E]
}

// 注册组件
func (ecc *ComponentRegister[E]) Register(componentName string, ctorOrder int, ctor ComponentCtor[E]) {
	if slices.ContainsFunc(ecc.RegisterInfos, func(ctor *ComponentRegisterInfo[E]) bool {
		return ctor.ComponentName == componentName
	}) {
		return
	}
	ecc.RegisterInfos = append(ecc.RegisterInfos, &ComponentRegisterInfo[E]{
		ComponentName: componentName,
		Ctor:          ctor,
		CtorOrder:     ctorOrder,
	})
	slices.SortStableFunc(ecc.RegisterInfos, func(a, b *ComponentRegisterInfo[E]) int {
		if a.CtorOrder == b.CtorOrder {
			return cmp.Compare(a.ComponentName, b.ComponentName)
		}
		return cmp.Compare(a.CtorOrder, b.CtorOrder)
	})
}

// 初始化组件
func (ecc *ComponentRegister[E]) InitComponents(entity E, arg any) {
	for _, ctor := range ecc.RegisterInfos {
		component := ctor.Ctor(entity, arg)
		if component != nil && entity.GetComponentByName(component.GetName()) == nil {
			entity.AddComponent(component, arg)
		}
	}
}
