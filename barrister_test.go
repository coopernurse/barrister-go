package barrister

import (
	"encoding/json"
	"fmt"
	. "github.com/couchbaselabs/go.assert"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
)

var strField = &Field{Type: "string", Optional: false, IsArray: false}
var enumField = &Field{Type: "StringAlias", Optional: false, IsArray: false}
var arrField = &Field{Type: "float", Optional: false, IsArray: true}

var noNestStruct = &Struct{Name: "NoNesting", Fields: []Field{
	Field{Name: "a", Type: "string", Optional: true, IsArray: false},
	Field{Name: "b", Type: "int", Optional: true, IsArray: false},
	Field{Name: "C", Type: "float", Optional: true, IsArray: false},
	Field{Name: "d", Type: "bool", Optional: true, IsArray: false},
	Field{Name: "E", Type: "string", Optional: true, IsArray: true},
}}
var noNestField = &Field{Type: "NoNesting", Optional: false, IsArray: true}

var nestStruct = &Struct{Name: "Nested", Fields: []Field{
	Field{Name: "name", Type: "string", Optional: false, IsArray: false},
	Field{Name: "Nest", Type: "NoNesting", Optional: false, IsArray: false},
}}
var nestField = &Field{Type: "Nested", Optional: false, IsArray: true}

type ConvertTest struct {
	target interface{}
	input  interface{}
	field  *Field
	ok     bool
}

type NoNesting struct {
	A string
	B int64
	C float64
	D bool
	E []string
}

type StringAlias string

type Nested struct {
	Name string
	Nest NoNesting
}

//////////////////////////////////////

func createTestIdl() *Idl {
	idl := &Idl{structs: map[string]*Struct{}, enums: map[string][]EnumValue{}}
	idl.structs["NoNesting"] = noNestStruct
	idl.structs["Nested"] = nestStruct
	idl.enums["StringAlias"] = []EnumValue{
		EnumValue{"blah", ""},
		EnumValue{"foo", ""},
	}
	idl.computeAllStructFields()
	return idl
}

func TestIdl2Go(t *testing.T) {
	idl := parseTestIdl()

	pkgNameToCode := idl.GenerateGo("conform", "", true)
	err := os.MkdirAll("conform/generated", 0755)
	if err != nil {
		t.Error(err)
	}
	err = ioutil.WriteFile("conform/generated/conform.go", pkgNameToCode["conform"], 0644)
	if err != nil {
		t.Error(err)
	}
}

func TestParseMethod(t *testing.T) {
	cases := [][]string{
		[]string{"B.echo", "B", "Echo"},
		[]string{"B.", "B.", ""},
		[]string{"Cat.a", "Cat", "A"},
		[]string{"barrister-idl", "barrister-idl", ""},
	}

	for _, c := range cases {
		iface, fname := parseMethod(c[0])
		Equals(t, iface, c[1])
		Equals(t, fname, c[2])
	}
}

func TestParseStuff(t *testing.T) {
	s := []byte(`{"jsonrpc":"2.0", "id":"123", "method": "blah", "params":["a","b"]}`)
	target := map[string]interface{}{}
	err := json.Unmarshal(s, &target)
	if err != nil {
		panic(err)
	}
}

func TestConvert(t *testing.T) {
	idl := createTestIdl()

	cases := []ConvertTest{
		ConvertTest{"hi", "hi", strField, true},
		ConvertTest{"", 10, strField, false},
		ConvertTest{[]float64{1, 2.1, 3}, []interface{}{1, 2.1, 3}, arrField, true},
		ConvertTest{StringAlias("blah"), "blah", enumField, true},
		ConvertTest{StringAlias("invalid"), "invalid", enumField, false},
		ConvertTest{NoNesting{A: "hi", B: 30}, map[string]interface{}{"a": "hi", "b": 30}, noNestField, true},
		ConvertTest{NoNesting{}, map[string]interface{}{"a": "hi", "b": "foo"}, noNestField, false},
		ConvertTest{NoNesting{C: 3.2, D: true}, map[string]interface{}{"C": 3.2, "d": true}, noNestField, true},
		ConvertTest{NoNesting{C: 2.8, D: false}, map[string]interface{}{"C": 2.8, "D": false}, noNestField, true},
		ConvertTest{NoNesting{E: []string{"a", "b"}}, map[string]interface{}{"E": []string{"a", "b"}}, noNestField, true},
		ConvertTest{Nested{Name: "hi", Nest: NoNesting{B: 30}}, map[string]interface{}{"name": "hi", "Nest": map[string]interface{}{"b": 30.0}}, nestField, true},
	}

	for x, test := range cases {
		msg := fmt.Sprintf("TestConvert[%d]", x)
		targetType := reflect.TypeOf(test.target)
		conv := newConvert(idl, test.field, targetType, test.input, msg)
		val, err := conv.run()
		if test.ok {
			if err != nil {
				t.Errorf("%s - Couldn't convert %v to %v: %v",
					msg, test.input, reflect.TypeOf(test.target), err)
			} else {
				if val.Kind() == reflect.Ptr {
					val = val.Elem()
				}

				if val.Type() != targetType {
					t.Errorf("%s - Return type: %v != %v", msg, val.Type(), targetType)
				} else if !reflect.DeepEqual(val.Interface(), test.target) {
					t.Errorf("%s - Expected %v but was %v", msg, test.target, val.Interface())
				}
			}
		} else if err == nil {
			t.Errorf("%s - Expected err converting %v to %v, but it worked: %v",
				msg, test.input, reflect.TypeOf(test.target).Name(), val.Interface())
		}
	}
}

func TestAddHandlerPanicsIfIfaceNotInIdl(t *testing.T) {
	idl := createTestIdl()
	svr := NewJSONServer(idl, true)

	fx := func() {
		defer func() {
			if r := recover(); r != nil {
				// ok
			}
		}()
		svr.AddHandler("C", BImpl{})
		t.Errorf("AddHandler didn't panic when called w/invalid iface name")
	}
	fx()
}

///////////////////////////////

func BenchmarkConvertSlice(b *testing.B) {
	b.StopTimer()
	idl := &Idl{structs: map[string]*Struct{}, enums: map[string][]EnumValue{}}
	arrField := &Field{Type: "float", Optional: false, IsArray: true}

	cases := []ConvertTest{
		ConvertTest{[]float64{}, []interface{}{1, 2.1, 3, 30.3, 32.0, 32323.3, 1, 2.1, 3, 30.3, 32.0, 32323.3}, arrField, true},
	}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		for _, test := range cases {
			targetType := reflect.TypeOf(test.target)
			conv := newConvert(idl, test.field, targetType, test.input, "")
			_, err := conv.run()
			if err != nil {
				panic(err)
			}
		}
	}
}

func BenchmarkConvertString(b *testing.B) {
	b.StopTimer()
	idl := &Idl{structs: map[string]*Struct{}, enums: map[string][]EnumValue{}}
	strField := &Field{Type: "string", Optional: false, IsArray: false}

	cases := []ConvertTest{
		ConvertTest{"hi", "hi", strField, true},
	}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		for _, test := range cases {
			targetType := reflect.TypeOf(test.target)
			conv := newConvert(idl, test.field, targetType, test.input, "")
			_, err := conv.run()
			if err != nil {
				panic(err)
			}
		}
	}
}

func BenchmarkConvertStruct(b *testing.B) {
	b.StopTimer()
	idl := &Idl{structs: map[string]*Struct{}, enums: map[string][]EnumValue{}}
	noNestStruct := &Struct{Name: "NoNesting", Fields: []Field{
		Field{Name: "a", Type: "string", Optional: true, IsArray: false},
		Field{Name: "b", Type: "int", Optional: true, IsArray: false},
		Field{Name: "C", Type: "float", Optional: true, IsArray: false},
		Field{Name: "d", Type: "bool", Optional: true, IsArray: false},
		Field{Name: "E", Type: "string", Optional: true, IsArray: true},
	}}
	noNestField := &Field{Type: "NoNesting", Optional: false, IsArray: true}
	idl.structs["NoNesting"] = noNestStruct

	cases := []ConvertTest{
		ConvertTest{NoNesting{A: "hi", B: 30}, map[string]interface{}{"a": "hi", "b": 30}, noNestField, true},
	}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		for _, test := range cases {
			targetType := reflect.TypeOf(test.target)
			conv := newConvert(idl, test.field, targetType, test.input, "")
			_, err := conv.run()
			if err != nil {
				panic(err)
			}
		}
	}
}
