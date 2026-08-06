package gentity

import (
	"reflect"
	"testing"

	"github.com/fish-tennis/gentity/examples/pb"
	"google.golang.org/protobuf/proto"
)

// TestConvertValueToInt_AllTypes 测试 ConvertValueToInt 对各种类型的转换.
func TestConvertValueToInt_AllTypes(t *testing.T) {
	// int32 -> 42
	r := ConvertValueToInt(reflect.TypeOf(int32(0)), reflect.ValueOf(int32(42)))
	if r != 42 {
		t.Errorf("int32: expect 42, got %d", r)
	}

	// int64 -> -99
	r = ConvertValueToInt(reflect.TypeOf(int64(0)), reflect.ValueOf(int64(-99)))
	if r != -99 {
		t.Errorf("int64: expect -99, got %d", r)
	}

	// uint32 -> 7
	r = ConvertValueToInt(reflect.TypeOf(uint32(0)), reflect.ValueOf(uint32(7)))
	if r != 7 {
		t.Errorf("uint32: expect 7, got %d", r)
	}

	// uint64 -> 100
	r = ConvertValueToInt(reflect.TypeOf(uint64(0)), reflect.ValueOf(uint64(100)))
	if r != 100 {
		t.Errorf("uint64: expect 100, got %d", r)
	}

	// float64 -> 3 (注意精度)
	r = ConvertValueToInt(reflect.TypeOf(float64(0)), reflect.ValueOf(3.0))
	if r != 3 {
		t.Errorf("float64: expect 3, got %d", r)
	}

	// 非数值类型(string) -> 0
	r = ConvertValueToInt(reflect.TypeOf(""), reflect.ValueOf("abc"))
	if r != 0 {
		t.Errorf("string: expect 0, got %d", r)
	}
}

// TestConvertInterfaceToRealType_NumericConversions 测试跨数值宽度转换的正确性及溢出行为.
func TestConvertInterfaceToRealType_NumericConversions(t *testing.T) {
	// typ=int8, v=int64(127) -> int8(127)
	r := ConvertInterfaceToRealType(reflect.TypeOf(int8(0)), int64(127))
	if v, ok := r.(int8); !ok || v != 127 {
		t.Errorf("int8(127): expect int8(127), got %T(%v)", r, r)
	}

	// typ=int8, v=int64(128) -> int8(-128) (溢出,验证不panic)
	r = ConvertInterfaceToRealType(reflect.TypeOf(int8(0)), int64(128))
	if v, ok := r.(int8); !ok || v != int8(-128) {
		t.Errorf("int8(128): expect int8(-128), got %T(%v)", r, r)
	}

	// typ=uint8, v=uint64(255) -> uint8(255)
	r = ConvertInterfaceToRealType(reflect.TypeOf(uint8(0)), uint64(255))
	if v, ok := r.(uint8); !ok || v != 255 {
		t.Errorf("uint8(255): expect uint8(255), got %T(%v)", r, r)
	}

	// typ=uint8, v=uint64(256) -> uint8(0) (溢出)
	r = ConvertInterfaceToRealType(reflect.TypeOf(uint8(0)), uint64(256))
	if v, ok := r.(uint8); !ok || v != 0 {
		t.Errorf("uint8(256): expect uint8(0), got %T(%v)", r, r)
	}

	// typ=int16, v=int64(-1) -> int16(-1)
	r = ConvertInterfaceToRealType(reflect.TypeOf(int16(0)), int64(-1))
	if v, ok := r.(int16); !ok || v != -1 {
		t.Errorf("int16(-1): expect int16(-1), got %T(%v)", r, r)
	}

	// typ=float64, v=float64(3.14) -> 3.14
	r = ConvertInterfaceToRealType(reflect.TypeOf(float64(0)), float64(3.14))
	if v, ok := r.(float64); !ok || v != 3.14 {
		t.Errorf("float64(3.14): expect float64(3.14), got %T(%v)", r, r)
	}
}

// TestConvertInterfaceToRealType_ProtoMessage 测试 proto.Message 的反序列化.
func TestConvertInterfaceToRealType_ProtoMessage(t *testing.T) {
	typ := reflect.TypeOf((*pb.BaseInfo)(nil))
	original := &pb.BaseInfo{Level: 5}
	bytes, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("proto.Marshal: unexpected err: %v", err)
	}

	r := ConvertInterfaceToRealType(typ, bytes)
	if r == nil {
		t.Fatal("expect *pb.BaseInfo, got nil")
	}
	bi, ok := r.(*pb.BaseInfo)
	if !ok {
		t.Fatalf("expect *pb.BaseInfo, got %T(%v)", r, r)
	}
	if bi.Level != 5 {
		t.Errorf("Level: expect 5, got %d", bi.Level)
	}
}

// TestConvertStringToRealType_BoolVariants 测试 bool 从字符串转换的各种情况.
func TestConvertStringToRealType_BoolVariants(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"true", true},
		{"1", true},
		{"false", false},
		{"0", false},
		{"", false},
		{"anything", false},
	}
	for _, c := range cases {
		got := ConvertStringToRealType(reflect.TypeOf(true), c.in)
		if v, ok := got.(bool); !ok || v != c.want {
			t.Errorf("bool(%q): expect %v, got %T(%v)", c.in, c.want, got, got)
		}
	}
}

// TestConvertStringToRealType_FloatPrecision 测试 float 转换的精度.
func TestConvertStringToRealType_FloatPrecision(t *testing.T) {
	// typ=float32, v="1.5" -> float32(1.5)
	r := ConvertStringToRealType(reflect.TypeOf(float32(0)), "1.5")
	if v, ok := r.(float32); !ok || v != float32(1.5) {
		t.Errorf("float32: expect float32(1.5), got %T(%v)", r, r)
	}

	// typ=float64, v="3.14159" -> float64(3.14159)
	r = ConvertStringToRealType(reflect.TypeOf(float64(0)), "3.14159")
	if v, ok := r.(float64); !ok || v != float64(3.14159) {
		t.Errorf("float64: expect float64(3.14159), got %T(%v)", r, r)
	}
}

// TestConvertValueToStringOrInterface_ProtoMarshal 测试 protobuf 序列化路径.
func TestConvertValueToStringOrInterface_ProtoMarshal(t *testing.T) {
	val := reflect.ValueOf(&pb.BaseInfo{Level: 1})
	r, err := convertValueToStringOrInterface(val)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	gotBytes, ok := r.([]byte)
	if !ok {
		t.Fatalf("expect []byte, got %T(%v)", r, r)
	}
	var decoded pb.BaseInfo
	if uerr := proto.Unmarshal(gotBytes, &decoded); uerr != nil {
		t.Fatalf("proto.Unmarshal: unexpected err: %v", uerr)
	}
	if decoded.Level != 1 {
		t.Errorf("Level: expect 1, got %d", decoded.Level)
	}
}

// TestConvertValueToStringOrInterface_IntValue 测试整数值转换为字符串.
func TestConvertValueToStringOrInterface_IntValue(t *testing.T) {
	val := reflect.ValueOf(int32(42))
	r, err := convertValueToStringOrInterface(val)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if s, ok := r.(string); !ok || s != "42" {
		t.Errorf("expect string \"42\", got %T(%v)", r, r)
	}
}

// TestGetFieldValue_StructNotFound 测试 struct 中不存在的字段.
func TestGetFieldValue_StructNotFound(t *testing.T) {
	type tmpStruct struct {
		Name string
	}
	v := GetFieldValue(reflect.ValueOf(tmpStruct{Name: "abc"}), "NotExist")
	if v.IsValid() {
		t.Errorf("expect invalid value for missing field, got valid %v", v)
	}
}

// TestGetFieldValue_MapNotFound 测试 map 中不存在的 key.
func TestGetFieldValue_MapNotFound(t *testing.T) {
	m := map[string]any{"a": 1}
	v := GetFieldValue(reflect.ValueOf(m), "b")
	if v.IsValid() {
		t.Errorf("expect invalid value for missing key, got valid %v", v)
	}
}

// TestConvertInterfaceToRealType_UnsupportedKind 测试不支持的类型返回 nil.
func TestConvertInterfaceToRealType_UnsupportedKind(t *testing.T) {
	typ := reflect.TypeOf(make(chan int))
	r := ConvertInterfaceToRealType(typ, make(chan int))
	if r != nil {
		t.Errorf("expect nil for unsupported kind (chan), got %T(%v)", r, r)
	}
}
