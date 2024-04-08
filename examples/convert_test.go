package examples

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/examples/pb"
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
	m := gentity.ConvertProtoToMap(message)
	t.Logf("%v", m)
}
