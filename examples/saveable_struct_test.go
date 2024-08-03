package examples

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/examples/pb"
	"github.com/fish-tennis/gentity/util"
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

type SingleEntityData struct {
	SingleFieldEmbedPointerComponent []byte
	SingleFieldEmbedStructComponent  []byte
	SingleFieldFieldStructComponent  []byte
	SingleFieldFieldPointerComponent []byte
}

func initSingleComponents(t *testing.T) {
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

func TestLoadMongo(t *testing.T) {
	gentity.SetLogLevel(gentity.DebugLevel)
	mongoDb := gentity.NewMongoDb(_mongoUri, _mongoDbName)
	playerDb := mongoDb.RegisterPlayerDb(_collectionName, true, "_id", "AccountId", "RegionId")
	if !mongoDb.Connect() {
		t.Fatal("connect db error")
	}
	defer func() {
		mongoDb.Disconnect()
	}()

	entityData := &ChildFieldsEntityData{}
	hasData, err := playerDb.FindEntityById(1, entityData)
	if err != nil {
		t.Fatalf("FindEntityById error:%v", err.Error())
	}
	t.Logf("hasData:%v", hasData)
	t.Logf("%v", entityData)
}

func TestSingleField(t *testing.T) {
	gentity.SetLogLevel(gentity.DebugLevel)
	mongoDb := gentity.NewMongoDb(_mongoUri, _mongoDbName)
	playerDb := mongoDb.RegisterPlayerDb(_collectionName, true, "_id", "AccountId", "RegionId")
	if !mongoDb.Connect() {
		t.Fatal("connect db error")
	}
	defer func() {
		mongoDb.Disconnect()
	}()

	kvCache := initRedis()

	initSingleComponents(t)

	entity := &gentity.BaseEntity{
		Id: 1,
	}
	_entityComponentRegister.InitComponents(entity, nil)
	entityData := &SingleEntityData{}
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
			loadErr := gentity.LoadComponentData(component, dataVal.Interface())
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

type MapStruct struct {
	*gentity.BaseComponent
	gentity.MapData[int32, *pb.BaseInfo] `db:""`
}

type MapPtr struct {
	*gentity.BaseComponent
	*gentity.MapData[int32, *pb.BaseInfo] `db:""`
}

type BaseMapField struct {
	*gentity.BaseComponent
	gentity.BaseMapDirtyMark
	Field map[int32]*pb.BaseInfo `db:""`
}

type MapField struct {
	*gentity.BaseComponent
	Field gentity.MapData[int32, *pb.BaseInfo] `db:""`
}

type MapFieldPtr struct {
	*gentity.BaseComponent
	FieldPtr *gentity.MapData[int32, *pb.BaseInfo] `db:""`
}

func initMapComponents(t *testing.T) {
	_entityComponentRegister.Register("MapStruct", 0, func(entity *gentity.BaseEntity, arg any) gentity.Component {
		component := &MapStruct{
			BaseComponent: gentity.NewBaseComponent(entity, "MapStruct"),
			MapData:       *gentity.NewMapData[int32, *pb.BaseInfo](),
		}
		return component
	})
	_entityComponentRegister.Register("MapPtr", 0, func(entity *gentity.BaseEntity, arg any) gentity.Component {
		component := &MapPtr{
			BaseComponent: gentity.NewBaseComponent(entity, "MapPtr"),
			MapData:       gentity.NewMapData[int32, *pb.BaseInfo](),
		}
		return component
	})
	_entityComponentRegister.Register("BaseMapField", 0, func(entity *gentity.BaseEntity, arg any) gentity.Component {
		component := &BaseMapField{
			BaseComponent: gentity.NewBaseComponent(entity, "BaseMapField"),
			Field:         make(map[int32]*pb.BaseInfo),
		}
		return component
	})
	_entityComponentRegister.Register("MapField", 0, func(entity *gentity.BaseEntity, arg any) gentity.Component {
		component := &MapField{
			BaseComponent: gentity.NewBaseComponent(entity, "MapField"),
			Field:         *gentity.NewMapData[int32, *pb.BaseInfo](),
		}
		return component
	})
	_entityComponentRegister.Register("MapFieldPtr", 0, func(entity *gentity.BaseEntity, arg any) gentity.Component {
		component := &MapFieldPtr{
			BaseComponent: gentity.NewBaseComponent(entity, "MapFieldPtr"),
			FieldPtr:      gentity.NewMapData[int32, *pb.BaseInfo](),
		}
		return component
	})
}

type ChildEntityData struct {
	MapStruct    map[int32][]byte
	MapPtr       map[int32][]byte
	BaseMapField map[int32][]byte
	MapField     map[int32][]byte
	MapFieldPtr  map[int32][]byte
}

func TestMapField(t *testing.T) {
	gentity.SetLogLevel(gentity.DebugLevel)
	mongoDb := gentity.NewMongoDb(_mongoUri, _mongoDbName)
	playerDb := mongoDb.RegisterPlayerDb(_collectionName, true, "_id", "AccountId", "RegionId")
	if !mongoDb.Connect() {
		t.Fatal("connect db error")
	}
	defer func() {
		mongoDb.Disconnect()
	}()
	kvCache := initRedis()

	initMapComponents(t)

	entity := &gentity.BaseEntity{
		Id: 1,
	}
	_entityComponentRegister.InitComponents(entity, nil)
	playerDb.DeleteEntity(entity.Id)
	playerDb.InsertEntity(entity.Id, map[string]any{
		"_id": entity.Id,
	})

	mapStruct := entity.GetComponentByName("MapStruct").(*MapStruct)
	mapStruct.Set(1, &pb.BaseInfo{
		Level:             1,
		LongFieldNameTest: "MapStruct",
	})

	mapPtr := entity.GetComponentByName("MapPtr").(*MapPtr)
	mapPtr.Set(2, &pb.BaseInfo{
		Level:             2,
		LongFieldNameTest: "MapPtr",
	})

	baseMapField := entity.GetComponentByName("BaseMapField").(*BaseMapField)
	baseMapField.Field[3] = &pb.BaseInfo{
		Level:             3,
		LongFieldNameTest: "BaseMapField",
	}
	baseMapField.SetDirty(int32(3), true)

	mapField := entity.GetComponentByName("MapField").(*MapField)
	mapField.Field.Set(4, &pb.BaseInfo{
		Level:             4,
		LongFieldNameTest: "MapField",
	})

	mapFieldPtr := entity.GetComponentByName("MapFieldPtr").(*MapFieldPtr)
	mapFieldPtr.FieldPtr.Set(5, &pb.BaseInfo{
		Level:             5,
		LongFieldNameTest: "MapFieldPtr",
	})

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

	// load data from db
	entityData := &ChildEntityData{}
	hasData, err := playerDb.FindEntityById(entity.Id, entityData)
	if err != nil {
		t.Fatal("FindEntityById error")
	}
	if !hasData {
		t.Fatal("hasData false")
	}
	loadEntity := &gentity.BaseEntity{
		Id: 1,
	}
	_entityComponentRegister.InitComponents(loadEntity, nil)
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
}

type ChildFields struct {
	*gentity.BaseComponent
	ProtoField       gentity.ProtoData[*pb.BaseInfo]       `child:"Proto"`
	ProtoFieldPtr    *gentity.ProtoData[*pb.BaseInfo]      `child:"ProtoPtr"`
	MapField         gentity.MapData[int32, *pb.BaseInfo]  `child:"MapField"`
	MapFieldPtr      *gentity.MapData[int32, *pb.BaseInfo] `child:"MapFieldPtr"`
	MapStructField   *MapStruct                            `child:"MapStruct"`
	MapPtrField      *MapPtr                               `child:"MapPtr"`
	MapFieldPtrField *MapFieldPtr                          `child:"MapFieldPtrField"`
}

type ChildFieldsData struct {
	Proto            []byte
	ProtoPtr         []byte
	MapField         map[int32][]byte
	MapFieldPtr      map[int32][]byte
	MapStruct        map[int32][]byte
	MapPtr           map[int32][]byte
	MapFieldPtrField map[int32][]byte
}

type ChildFieldsEntityData struct {
	ChildFields *ChildFieldsData
}

func initChildComponents(t *testing.T) {
	_entityComponentRegister.Register("ChildFields", 0, func(entity *gentity.BaseEntity, arg any) gentity.Component {
		component := &ChildFields{
			BaseComponent: gentity.NewBaseComponent(entity, "ChildFields"),
			ProtoField:    *gentity.NewProtoData(&pb.BaseInfo{}),
			ProtoFieldPtr: gentity.NewProtoData(&pb.BaseInfo{}),
			MapField:      *gentity.NewMapData[int32, *pb.BaseInfo](),
			MapFieldPtr:   gentity.NewMapData[int32, *pb.BaseInfo](),
			MapStructField: &MapStruct{
				MapData: *gentity.NewMapData[int32, *pb.BaseInfo](),
			},
			MapPtrField: &MapPtr{
				MapData: gentity.NewMapData[int32, *pb.BaseInfo](),
			},
			MapFieldPtrField: &MapFieldPtr{
				FieldPtr: gentity.NewMapData[int32, *pb.BaseInfo](),
			},
		}
		return component
	})
}

func TestChildFields(t *testing.T) {
	gentity.SetLogLevel(gentity.DebugLevel)
	mongoDb := gentity.NewMongoDb(_mongoUri, _mongoDbName)
	playerDb := mongoDb.RegisterPlayerDb(_collectionName, true, "_id", "AccountId", "RegionId")
	if !mongoDb.Connect() {
		t.Fatal("connect db error")
	}
	defer func() {
		mongoDb.Disconnect()
	}()
	kvCache := initRedis()

	initChildComponents(t)

	entity := &gentity.BaseEntity{
		Id: 1,
	}
	_entityComponentRegister.InitComponents(entity, nil)
	playerDb.DeleteEntity(entity.Id)
	playerDb.InsertEntity(entity.Id, map[string]any{
		"_id": entity.Id,
	})

	childFields := entity.GetComponentByName("ChildFields").(*ChildFields)
	childFields.ProtoField.Data = &pb.BaseInfo{
		Level:             1,
		LongFieldNameTest: "ProtoField",
	}
	childFields.ProtoField.SetDirty()

	childFields.ProtoFieldPtr.Data = &pb.BaseInfo{
		Level:             2,
		LongFieldNameTest: "ProtoFieldPtr",
	}
	childFields.ProtoFieldPtr.SetDirty()

	childFields.MapField.Set(3, &pb.BaseInfo{
		Level:             3,
		LongFieldNameTest: "MapField",
	})

	childFields.MapFieldPtr.Set(4, &pb.BaseInfo{
		Level:             4,
		LongFieldNameTest: "MapFieldPtr",
	})

	childFields.MapStructField.Set(5, &pb.BaseInfo{
		Level:             5,
		LongFieldNameTest: "MapStructField",
	})

	childFields.MapPtrField.Set(6, &pb.BaseInfo{
		Level:             6,
		LongFieldNameTest: "MapPtrField",
	})

	childFields.MapFieldPtrField.FieldPtr.Set(7, &pb.BaseInfo{
		Level:             7,
		LongFieldNameTest: "MapFieldPtrField",
	})

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

	// load data from db
	entityData := &ChildFieldsEntityData{}
	hasData, err := playerDb.FindEntityById(entity.Id, entityData)
	if err != nil {
		t.Fatalf("FindEntityById error:%v", err.Error())
	}
	if !hasData {
		t.Fatal("hasData false")
	}
	loadEntity := &gentity.BaseEntity{
		Id: 1,
	}
	_entityComponentRegister.InitComponents(loadEntity, nil)
	t.Logf("load data from db")
	err = gentity.LoadEntityData(loadEntity, entityData)
	if err != nil {
		t.Fatalf("LoadEntityData error:%v", err.Error())
	}
	childFields = loadEntity.GetComponentByName("ChildFields").(*ChildFields)
	t.Logf("ProtoField:%v", childFields.ProtoField.Data)
	t.Logf("ProtoFieldPtr:%v", childFields.ProtoFieldPtr.Data)
	t.Logf("MapField:%v", childFields.MapField.Data)
	t.Logf("MapFieldPtr:%v", childFields.MapFieldPtr.Data)
	t.Logf("MapStructField:%v", childFields.MapStructField.Data)
	t.Logf("MapPtrField:%v", childFields.MapPtrField.Data)
	t.Logf("MapFieldPtrField:%v", childFields.MapFieldPtrField.FieldPtr.Data)

}
