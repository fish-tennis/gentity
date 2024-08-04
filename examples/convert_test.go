package examples

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/examples/pb"
	"google.golang.org/protobuf/proto"
	"reflect"
	"testing"
)

func TestConvert(t *testing.T) {
	bTrue := true
	bFalse := false
	f32 := 1.23456
	f64 := 1234567890.1234567890
	typeBool := reflect.TypeOf(bTrue)
	typeString := reflect.TypeOf("")
	typeFloat32 := reflect.TypeOf(f32)
	typeFloat64 := reflect.TypeOf(f64)
	t.Log(gentity.ConvertValueToInterface(typeBool, typeString, reflect.ValueOf(bTrue)))
	t.Log(gentity.ConvertValueToInterface(typeBool, typeString, reflect.ValueOf(bFalse)))
	t.Log(gentity.ConvertValueToInterface(typeFloat32, typeString, reflect.ValueOf(f32)))
	t.Log(gentity.ConvertValueToInterface(typeFloat64, typeString, reflect.ValueOf(f64)))

	t.Log(gentity.ConvertInterfaceToRealType(typeBool, gentity.ConvertValueToInterface(typeBool, typeString, reflect.ValueOf(bTrue))))
	t.Log(gentity.ConvertInterfaceToRealType(typeBool, gentity.ConvertValueToInterface(typeBool, typeString, reflect.ValueOf(bFalse))))
	t.Log(gentity.ConvertInterfaceToRealType(typeFloat32, gentity.ConvertValueToInterface(typeFloat32, typeString, reflect.ValueOf(f32))))
	t.Log(gentity.ConvertInterfaceToRealType(typeFloat64, gentity.ConvertValueToInterface(typeFloat64, typeString, reflect.ValueOf(f64))))

	message := &pb.PlayerData{
		XId:       1,
		Name:      "test",
		AccountId: 2,
		RegionId:  3,
		IsGM:      true,
		BaseInfo: &pb.BaseInfo{
			Gender: 1,
			Level:  2,
			Exp:    12345,
		},
	}
	m := map[string]any{
		"BaseInfo": &pb.BaseInfo{
			Gender: 3,
			Level:  4,
			Exp:    67890,
		},
	}
	field := gentity.GetFieldValue(reflect.ValueOf(message), "BaseInfo")
	t.Logf("%v", field.Interface())
	field = gentity.GetFieldValue(reflect.ValueOf(m), "BaseInfo")
	t.Logf("%v", field.Interface())
}

func TestSlice(t *testing.T) {
	type SliceA struct {
		S []int32
	}
	a := &SliceA{}
	var data, smallData, bigData []int32
	for i := 0; i < 10; i++ {
		a.S = append(a.S, int32(i+1))
		data = append(data, int32(i+11))
	}
	for i := 0; i < 5; i++ {
		smallData = append(smallData, int32(i+21))
	}
	for i := 0; i < 20; i++ {
		bigData = append(bigData, int32(i+31))
	}
	objA := reflect.ValueOf(a).Elem()
	fieldA := objA.FieldByName("S")

	dataVal := reflect.ValueOf(data)
	reflect.Copy(fieldA, dataVal)
	t.Logf("copy:%v", a.S)

	smallDataVal := reflect.ValueOf(smallData)
	fieldA.SetLen(smallDataVal.Len())
	reflect.Copy(fieldA, smallDataVal)
	t.Logf("copySmallData:%v", a.S)

	bigDataVal := reflect.ValueOf(bigData)
	t.Logf("before grow len:%v cap:%v", fieldA.Len(), fieldA.Cap())
	fieldA.Grow(bigDataVal.Len())
	fieldA.SetLen(bigDataVal.Len())
	t.Logf("after grow len:%v cap:%v", fieldA.Len(), fieldA.Cap())
	reflect.Copy(fieldA, bigDataVal)
	t.Logf("copyBigData:%v", a.S)

	if fieldA.CanSet() {
		fieldA.Set(dataVal)
		t.Logf("set:%v", a.S)
	}
}

func TestSaveableStruct(t *testing.T) {
	type AnyField struct {
		// 不支持的类型
		Any any `db:""`
	}
	s := gentity.GetObjSaveableStruct(&AnyField{})
	t.Logf("FieldStruct:%v", s)

	type Item struct {
		S string
	}
	type StructField struct {
		// 不支持的类型
		Item    pb.QuestData `db:""`
		ItemPtr *Item
	}
	s = gentity.GetObjSaveableStruct(&StructField{})
	t.Logf("FieldStruct:%v", s)

	obj := &StructField{}
	obj.Item.CfgId = 123
	obj.Item.Progress = 456
	obj.ItemPtr = &Item{S: "def"}
	t.Logf("Item:%v", &obj.Item)

	objVal := reflect.ValueOf(obj).Elem()

	ptrFieldVal := objVal.FieldByName("ItemPtr").Elem()
	ptrFieldVal.FieldByName("S").SetString("uvw")
	t.Logf("ItemPtr:%v", obj.ItemPtr)

	fieldVal := objVal.FieldByName("Item")
	fieldVal.FieldByName("CfgId").SetInt(999)
	t.Logf("Item:%v", &obj.Item)

	//fieldInterface := fieldVal.Interface()
	bytes, err := proto.Marshal(&obj.Item)
	t.Logf("err:%v len:%v", err, len(bytes))

	//fieldInterface := fieldVal.Interface()
	//fieldVal.Addr()

	t.Logf("canAddr:%v CanInterface:%v", fieldVal.CanAddr(), fieldVal.CanInterface())
	if fieldVal.CanAddr() {
		ptrField := fieldVal.Addr()
		v := ptrField.Interface()
		t.Logf("v:%v", v)
		if protoMessage, ok := v.(proto.Message); ok {
			bytes, err = proto.Marshal(protoMessage)
			t.Logf("err:%v len:%v", err, len(bytes))
			newData := pb.QuestData{}
			err = proto.Unmarshal(bytes, &newData)
			t.Logf("err:%v newData:%v", err, newData)
		}
	}
}

func TestChildSaveableStruct(t *testing.T) {
	gentity.SetLogLevel(-1)
	//bag := &Bag{
	//	BagCountItem: &BagCountItem{},
	//	BagUniqueItem: &BagUniqueItem{
	//		Data: make(map[int64]int32),
	//	},
	//}
	//bag.BagCountItem.Init()
	//gentity.GetSaveableStruct(reflect.TypeOf(bag))

	player := newTestPlayer(1, 1)
	gentity.ParseEntitySaveableStruct(player)
	bag := player.GetBag()
	for i := 0; i < 3; i++ {
		bag.BagCountItem.AddItem(int32(i+1), int32((i+1)*10))
	}
	//saveData, err := gentity.GetComponentSaveData(bag)
	//if err != nil {
	//	t.Error(err)
	//}
	//t.Log(fmt.Sprintf("%v", saveData))

	structCache := gentity.GetObjSaveableStruct(bag.BagCountItem)
	objVal := reflect.ValueOf(bag.BagCountItem).Elem()
	field := objVal.Field(structCache.Field.FieldIndex)
	fieldInterface := field.Interface()
	t.Logf("CanAddr:%v kind:%v", field.CanAddr(), field.Kind())
	if _, ok := fieldInterface.(gentity.Saveable); ok {
		t.Logf("%v", fieldInterface)
	} else if field.CanAddr() {
		fieldAddr := field.Addr()
		fieldInterface = fieldAddr.Interface()
		if saveable, ok := fieldInterface.(gentity.Saveable); ok {
			t.Logf("%v", fieldInterface)
			valueSaveData, valueSaveErr := gentity.GetSaveData(saveable, "Bag")
			t.Logf("valueSaveData:%v err:%v", valueSaveData, valueSaveErr)
		}
	}

}
