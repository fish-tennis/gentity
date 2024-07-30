package examples

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/examples/pb"
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gnet"
	"reflect"
	"testing"
)

var _entityComponentRegister = gentity.ComponentRegister[*gentity.BaseEntity]{}

type SingleFieldEmbedPointer struct {
	gentity.BaseDirtyMark
	*pb.BaseInfo `db:""`
}

type SingleFieldEmbedStruct struct {
	gentity.BaseDirtyMark
	pb.BaseInfo `db:""`
}

type SingleFieldFieldStruct struct {
	gentity.BaseDirtyMark
	FieldStruct pb.BaseInfo `db:""`
}

type SingleFieldFieldPointer struct {
	gentity.BaseDirtyMark
	FieldPointer *pb.BaseInfo `db:""`
}

type SingleFieldEmbedPointerComponent struct {
	*gentity.BaseComponent
	*SingleFieldEmbedPointer `db:""`
}

type SingleFieldEmbedStructComponent struct {
	*gentity.BaseComponent
	SingleFieldEmbedStruct `db:""`
}

type SingleFieldFieldStructComponent struct {
	*gentity.BaseComponent
	Field SingleFieldFieldStruct `db:""`
}

type SingleFieldFieldPointerComponent struct {
	*gentity.BaseComponent
	Field *SingleFieldFieldPointer `db:""`
}

type EntityData struct {
	SingleFieldEmbedPointerComponent []byte
	SingleFieldEmbedStructComponent  []byte
	SingleFieldFieldStructComponent  []byte
	SingleFieldFieldPointerComponent []byte
}

func initEntity(t *testing.T) {
	_entityComponentRegister.Register("SingleFieldEmbedPointerComponent", 0, func(entity *gentity.BaseEntity, arg any) gentity.Component {
		component := &SingleFieldEmbedPointerComponent{
			BaseComponent: gentity.NewBaseComponent(entity, "SingleFieldEmbedPointerComponent"),
			SingleFieldEmbedPointer: &SingleFieldEmbedPointer{
				BaseInfo: &pb.BaseInfo{},
			},
		}
		return component
	})
	_entityComponentRegister.Register("SingleFieldEmbedStructComponent", 0, func(entity *gentity.BaseEntity, arg any) gentity.Component {
		component := &SingleFieldEmbedStructComponent{
			BaseComponent: gentity.NewBaseComponent(entity, "SingleFieldEmbedStructComponent"),
			SingleFieldEmbedStruct: SingleFieldEmbedStruct{
				BaseInfo: pb.BaseInfo{},
			},
		}
		return component
	})
	_entityComponentRegister.Register("SingleFieldFieldStructComponent", 0, func(entity *gentity.BaseEntity, arg any) gentity.Component {
		component := &SingleFieldFieldStructComponent{
			BaseComponent: gentity.NewBaseComponent(entity, "SingleFieldFieldStructComponent"),
			Field: SingleFieldFieldStruct{
				FieldStruct: pb.BaseInfo{},
			},
		}
		return component
	})
	_entityComponentRegister.Register("SingleFieldFieldPointerComponent", 0, func(entity *gentity.BaseEntity, arg any) gentity.Component {
		component := &SingleFieldFieldPointerComponent{
			BaseComponent: gentity.NewBaseComponent(entity, "SingleFieldFieldPointerComponent"),
			Field: &SingleFieldFieldPointer{
				FieldPointer: &pb.BaseInfo{},
			},
		}
		return component
	})
}

func TestSingleField(t *testing.T) {
	gnet.SetLogLevel(gnet.DebugLevel)
	mongoDb := gentity.NewMongoDb(_mongoUri, _mongoDbName)
	playerDb := mongoDb.RegisterPlayerDb(_collectionName, true, "_id", "AccountId", "RegionId")
	if !mongoDb.Connect() {
		t.Fatal("connect db error")
	}
	defer func() {
		mongoDb.Disconnect()
	}()
	kvCache := initRedis()

	initEntity(t)

	entity := &gentity.BaseEntity{
		Id: 1,
	}
	_entityComponentRegister.InitComponents(entity, nil)
	entityData := &EntityData{}
	hasData, err := playerDb.FindEntityById(entity.Id, entityData)
	if err != nil {
		t.Fatal("FindEntityById error")
	}
	if hasData {
		t.Logf("load data from db")
		entity.RangeComponent(func(component gentity.Component) bool {
			dataVal := reflect.ValueOf(entityData).Elem().FieldByName(component.GetName())
			if util.IsValueNil(dataVal) {
				return true
			}
			loadErr := gentity.LoadData(component, dataVal.Interface())
			if loadErr != nil {
				t.Logf("loadErr:%v", loadErr.Error())
				return true
			}
			t.Logf("%v:%v", component.GetName(), component)
			return true
		})
		return
	}

	playerDb.InsertEntity(entity.Id, map[string]any{
		"_id": entity.Id,
	})

	cSingleFieldEmbedPointer := entity.GetComponentByName("SingleFieldEmbedPointerComponent").(*SingleFieldEmbedPointerComponent)
	cSingleFieldEmbedPointer.BaseInfo = &pb.BaseInfo{
		Gender:            1,
		Level:             1,
		Exp:               1,
		LongFieldNameTest: "SingleFieldEmbedPointer",
	}
	cSingleFieldEmbedPointer.SetDirty()

	cSingleFieldEmbedStructComponent := entity.GetComponentByName("SingleFieldEmbedStructComponent").(*SingleFieldEmbedStructComponent)
	cSingleFieldEmbedStructComponent.BaseInfo = pb.BaseInfo{
		Gender:            2,
		Level:             2,
		Exp:               2,
		LongFieldNameTest: "SingleFieldEmbedStruct",
	}
	cSingleFieldEmbedStructComponent.SetDirty()

	cSingleFieldFieldStruct := entity.GetComponentByName("SingleFieldFieldStructComponent").(*SingleFieldFieldStructComponent)
	cSingleFieldFieldStruct.Field.FieldStruct = pb.BaseInfo{
		Gender:            3,
		Level:             3,
		Exp:               3,
		LongFieldNameTest: "SingleFieldFieldStruct",
	}
	cSingleFieldFieldStruct.Field.SetDirty()

	cSingleFieldFieldPointer := entity.GetComponentByName("SingleFieldFieldPointerComponent").(*SingleFieldFieldPointerComponent)
	cSingleFieldFieldPointer.Field.FieldPointer = &pb.BaseInfo{
		Gender:            4,
		Level:             4,
		Exp:               4,
		LongFieldNameTest: "SingleFieldFieldPointer",
	}
	cSingleFieldFieldPointer.Field.SetDirty()

	entity.SaveCache(kvCache, "t", entity.Id)
	gentity.SaveEntityChangedDataToDb(playerDb, entity, kvCache, false, "t")

	t.Logf("fix data from cache")
	fixEntity := &gentity.BaseEntity{
		Id: 1,
	}
	_entityComponentRegister.InitComponents(fixEntity, nil)
	gentity.FixEntityDataFromCache(fixEntity, playerDb, kvCache, "t", fixEntity.Id)
	fixEntity.RangeComponent(func(component gentity.Component) bool {
		t.Logf("%v:%v", component.GetName(), component)
		return true
	})
}
