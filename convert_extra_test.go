package gentity

import (
	"reflect"
	"testing"

	"github.com/fish-tennis/gentity/examples/pb"
	"google.golang.org/protobuf/proto"
)

// TestConvertValueToInterface_Numeric 测试 ConvertValueToInterface 各种数值类型转换.
func TestConvertValueToInterface_Numeric(t *testing.T) {
	// int32 -> int64
	r := ConvertValueToInterface(reflect.TypeOf(int32(0)), reflect.TypeOf(int64(0)), reflect.ValueOf(int32(42)))
	if v, ok := r.(int64); !ok || v != 42 {
		t.Errorf("int32->int64: expect int64(42), got %T(%v)", r, r)
	}

	// int64 -> int32
	r = ConvertValueToInterface(reflect.TypeOf(int64(0)), reflect.TypeOf(int32(0)), reflect.ValueOf(int64(42)))
	if v, ok := r.(int32); !ok || v != 42 {
		t.Errorf("int64->int32: expect int32(42), got %T(%v)", r, r)
	}

	// uint32 -> uint64
	r = ConvertValueToInterface(reflect.TypeOf(uint32(0)), reflect.TypeOf(uint64(0)), reflect.ValueOf(uint32(42)))
	if v, ok := r.(uint64); !ok || v != 42 {
		t.Errorf("uint32->uint64: expect uint64(42), got %T(%v)", r, r)
	}

	// float32 -> float64
	r = ConvertValueToInterface(reflect.TypeOf(float32(0)), reflect.TypeOf(float64(0)), reflect.ValueOf(float32(1.5)))
	if v, ok := r.(float64); !ok || v != float64(1.5) {
		t.Errorf("float32->float64: expect float64(1.5), got %T(%v)", r, r)
	}

	// int -> string: ConvertInterfaceToRealType 对 String 目标原样返回值, 因此结果为 int64(42)
	r = ConvertValueToInterface(reflect.TypeOf(int(0)), reflect.TypeOf(""), reflect.ValueOf(int(42)))
	if v, ok := r.(int64); !ok || v != 42 {
		t.Errorf("int->string: expect int64(42) (String 目标原样返回), got %T(%v)", r, r)
	}
}

// TestConvertInterfaceToRealType_AllTypes 测试 ConvertInterfaceToRealType 对所有基础类型的转换.
func TestConvertInterfaceToRealType_AllTypes(t *testing.T) {
	// typ=int, v=int64(42) -> int(42)
	r := ConvertInterfaceToRealType(reflect.TypeOf(int(0)), int64(42))
	if v, ok := r.(int); !ok || v != 42 {
		t.Errorf("int: expect int(42), got %T(%v)", r, r)
	}

	// typ=int32, v=int64(42) -> int32(42)
	r = ConvertInterfaceToRealType(reflect.TypeOf(int32(0)), int64(42))
	if v, ok := r.(int32); !ok || v != 42 {
		t.Errorf("int32: expect int32(42), got %T(%v)", r, r)
	}

	// typ=uint32, v=uint64(42) -> uint32(42)
	r = ConvertInterfaceToRealType(reflect.TypeOf(uint32(0)), uint64(42))
	if v, ok := r.(uint32); !ok || v != 42 {
		t.Errorf("uint32: expect uint32(42), got %T(%v)", r, r)
	}

	// typ=float32, v=float32(1.5) -> float32(1.5)
	// 注: 内部断言为 v.(float32), 需传入 float32 以避免类型不匹配 panic
	r = ConvertInterfaceToRealType(reflect.TypeOf(float32(0)), float32(1.5))
	if v, ok := r.(float32); !ok || v != float32(1.5) {
		t.Errorf("float32: expect float32(1.5), got %T(%v)", r, r)
	}

	// typ=string, v="hello" -> "hello"
	r = ConvertInterfaceToRealType(reflect.TypeOf(""), "hello")
	if v, ok := r.(string); !ok || v != "hello" {
		t.Errorf("string: expect \"hello\", got %T(%v)", r, r)
	}

	// typ=bool, v=true -> true
	r = ConvertInterfaceToRealType(reflect.TypeOf(true), true)
	if v, ok := r.(bool); !ok || !v {
		t.Errorf("bool: expect true, got %T(%v)", r, r)
	}
}

// TestConvertStringToRealType_AllTypes 测试 ConvertStringToRealType 把字符串转成各种类型.
func TestConvertStringToRealType_AllTypes(t *testing.T) {
	// int32: "42" -> int32(42)
	r := ConvertStringToRealType(reflect.TypeOf(int32(0)), "42")
	if v, ok := r.(int32); !ok || v != 42 {
		t.Errorf("int32: expect int32(42), got %T(%v)", r, r)
	}

	// int64: "9999999999" -> int64(9999999999)
	r = ConvertStringToRealType(reflect.TypeOf(int64(0)), "9999999999")
	if v, ok := r.(int64); !ok || v != int64(9999999999) {
		t.Errorf("int64: expect int64(9999999999), got %T(%v)", r, r)
	}

	// uint32: "42" -> uint32(42)
	r = ConvertStringToRealType(reflect.TypeOf(uint32(0)), "42")
	if v, ok := r.(uint32); !ok || v != 42 {
		t.Errorf("uint32: expect uint32(42), got %T(%v)", r, r)
	}

	// float32: "1.5" -> float32(1.5)
	r = ConvertStringToRealType(reflect.TypeOf(float32(0)), "1.5")
	if v, ok := r.(float32); !ok || v != float32(1.5) {
		t.Errorf("float32: expect float32(1.5), got %T(%v)", r, r)
	}

	// float64: "3.14" -> float64(3.14)
	r = ConvertStringToRealType(reflect.TypeOf(float64(0)), "3.14")
	if v, ok := r.(float64); !ok || v != float64(3.14) {
		t.Errorf("float64: expect float64(3.14), got %T(%v)", r, r)
	}

	// bool: "true"/"1" -> true, "false"/"0" -> false
	boolCases := []struct {
		in   string
		want bool
	}{
		{"true", true},
		{"1", true},
		{"false", false},
		{"0", false},
	}
	for _, c := range boolCases {
		got := ConvertStringToRealType(reflect.TypeOf(true), c.in)
		if v, ok := got.(bool); !ok || v != c.want {
			t.Errorf("bool(%q): expect %v, got %T(%v)", c.in, c.want, got, got)
		}
	}

	// string: "hello" -> "hello"
	r = ConvertStringToRealType(reflect.TypeOf(""), "hello")
	if v, ok := r.(string); !ok || v != "hello" {
		t.Errorf("string: expect \"hello\", got %T(%v)", r, r)
	}

	// []byte: "abc" -> []byte("abc")
	r = ConvertStringToRealType(reflect.TypeOf([]byte{}), "abc")
	if v, ok := r.([]byte); !ok || string(v) != "abc" {
		t.Errorf("[]byte: expect []byte(\"abc\"), got %T(%v)", r, r)
	}
}

// TestConvertValueToString_AllTypes 测试 convertValueToString 对各种类型的转换.
func TestConvertValueToString_AllTypes(t *testing.T) {
	// int: 42 -> "42"
	if s, err := convertValueToString(reflect.ValueOf(int(42))); err != nil || s != "42" {
		t.Errorf("int: expect \"42\", got %q err=%v", s, err)
	}

	// int64: -123 -> "-123"
	if s, err := convertValueToString(reflect.ValueOf(int64(-123))); err != nil || s != "-123" {
		t.Errorf("int64: expect \"-123\", got %q err=%v", s, err)
	}

	// uint: 99 -> "99"
	if s, err := convertValueToString(reflect.ValueOf(uint(99))); err != nil || s != "99" {
		t.Errorf("uint: expect \"99\", got %q err=%v", s, err)
	}

	// float64: 3.14 -> "3.14"
	if s, err := convertValueToString(reflect.ValueOf(float64(3.14))); err != nil || s != "3.14" {
		t.Errorf("float64: expect \"3.14\", got %q err=%v", s, err)
	}

	// bool: true -> "true", false -> "false" (今天修复的)
	if s, err := convertValueToString(reflect.ValueOf(true)); err != nil || s != "true" {
		t.Errorf("bool(true): expect \"true\", got %q err=%v", s, err)
	}
	if s, err := convertValueToString(reflect.ValueOf(false)); err != nil || s != "false" {
		t.Errorf("bool(false): expect \"false\", got %q err=%v", s, err)
	}

	// string: "hello" -> "hello"
	if s, err := convertValueToString(reflect.ValueOf("hello")); err != nil || s != "hello" {
		t.Errorf("string: expect \"hello\", got %q err=%v", s, err)
	}
}

// TestConvertValueToStringOrInterface 测试 convertValueToStringOrInterface.
func TestConvertValueToStringOrInterface(t *testing.T) {
	// int 值 -> string
	r, err := convertValueToStringOrInterface(reflect.ValueOf(int(42)))
	if err != nil || r != "42" {
		t.Errorf("int: expect \"42\", got %T(%v) err=%v", r, r, err)
	}

	// string 值 -> string
	r, err = convertValueToStringOrInterface(reflect.ValueOf("hello"))
	if err != nil || r != "hello" {
		t.Errorf("string: expect \"hello\", got %T(%v) err=%v", r, r, err)
	}

	// bool 值 -> string (今天修复的)
	r, err = convertValueToStringOrInterface(reflect.ValueOf(true))
	if err != nil || r != "true" {
		t.Errorf("bool(true): expect \"true\", got %T(%v) err=%v", r, r, err)
	}
	r, err = convertValueToStringOrInterface(reflect.ValueOf(false))
	if err != nil || r != "false" {
		t.Errorf("bool(false): expect \"false\", got %T(%v) err=%v", r, r, err)
	}

	// nil interface: 不应 panic, 返回 error
	r, err = convertValueToStringOrInterface(reflect.ValueOf((*int)(nil)))
	if err == nil {
		t.Errorf("nil: expect non-nil error, got r=%T(%v)", r, r)
	}

	// proto.Message -> []byte (序列化结果)
	quest := &pb.QuestData{CfgId: 1, Progress: 50}
	r, err = convertValueToStringOrInterface(reflect.ValueOf(quest))
	if err != nil {
		t.Fatalf("proto: unexpected err: %v", err)
	}
	gotBytes, ok := r.([]byte)
	if !ok {
		t.Fatalf("proto: expect []byte, got %T(%v)", r, r)
	}
	expectedBytes, mErr := proto.Marshal(quest)
	if mErr != nil {
		t.Fatalf("proto.Marshal: unexpected err: %v", mErr)
	}
	if !reflect.DeepEqual(gotBytes, expectedBytes) {
		t.Errorf("proto: expect marshalled %v, got %v", expectedBytes, gotBytes)
	}
}

// TestGetFieldValue 测试 GetFieldValue 对 struct 和 map 的查找.
func TestGetFieldValue(t *testing.T) {
	type tmpStruct struct {
		Name string
		Age  int
	}

	// struct: 已存在字段返回有效值
	v := GetFieldValue(reflect.ValueOf(tmpStruct{Name: "abc", Age: 18}), "Name")
	if !v.IsValid() {
		t.Fatal("struct Name: expect valid value, got invalid")
	}
	if got, ok := v.Interface().(string); !ok || got != "abc" {
		t.Errorf("struct Name: expect \"abc\", got %v", v.Interface())
	}

	// struct: 不存在字段返回 invalid
	v = GetFieldValue(reflect.ValueOf(tmpStruct{}), "Missing")
	if v.IsValid() {
		t.Errorf("struct Missing: expect invalid value, got valid %v", v)
	}

	// map: 已存在 key 返回有效值
	m := map[string]any{"key": "val"}
	v = GetFieldValue(reflect.ValueOf(m), "key")
	if !v.IsValid() {
		t.Fatal("map key: expect valid value, got invalid")
	}
	if got := v.Interface(); got != "val" {
		t.Errorf("map key: expect \"val\", got %v", got)
	}

	// map: 不存在 key 返回 invalid
	v = GetFieldValue(reflect.ValueOf(m), "missing")
	if v.IsValid() {
		t.Errorf("map missing: expect invalid value, got valid %v", v)
	}
}
