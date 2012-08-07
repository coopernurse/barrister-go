package barrister_test

import (
	. "github.com/coopernurse/barrister-go"
	"io/ioutil"
	"reflect"
	"testing"
	"fmt"
)

func readFile(fname string) []byte {
	b, err := ioutil.ReadFile(fname)
	if err != nil {
		panic(err)
	}
	return b
}

func parseTestIdl() *Idl {
	json := readFile("test/conform.json")
	idl, err := ParseIdlJson(json)
	if err != nil {
		panic(err)
	}
	return idl
}

func TestIdl2Go(t *testing.T) {
	idl := parseTestIdl()
	
	code := idl.GenerateGo("conform")
	ioutil.WriteFile("conform.go", code, 0644)
}

func TestParseIdlJson(t *testing.T) {
	idl := parseTestIdl()
	
	meta := Meta{BarristerVersion: "0.1.2", DateGenerated: 1337654725230000000, Checksum: "34f6238ed03c6319017382e0fdc638a7"}
	
	expected := Idl{Meta: meta}
	expected.Elems = append(expected.Elems, IdlJsonElem{Type: "comment", Value: "Barrister conformance IDL\n\nThe bits in here have silly names and the operations\nare not intended to be useful.  The intent is to\nexercise as much of the IDL grammar as possible"})

	enumVals := []EnumValue{
		EnumValue{Value: "ok"},
		EnumValue{Value: "err"},
	}
	expected.Elems = append(expected.Elems, 
		IdlJsonElem{Type: "enum", Name: "Status", Values: enumVals})

	enumVals2 := []EnumValue{
		EnumValue{Value: "add"},
		EnumValue{Value: "multiply", Comment: "mult comment"},
	}
	expected.Elems = append(expected.Elems, 
		IdlJsonElem{Type: "enum", Name: "MathOp", Values: enumVals2})

	fields := []Field{
		Field{Optional: false, IsArray: false, Type: "Status", Name: "status"},
	}
	expected.Elems = append(expected.Elems, IdlJsonElem{
		Type: "struct", Name: "Response", Fields: fields})

	fields2 := []Field{
		Field{Optional: false, IsArray: false, Type: "int", Name: "count"},
		Field{Optional: false, IsArray: true, Type: "string", Name: "items"},
	}
	expected.Elems = append(expected.Elems, 
		IdlJsonElem{Type: "struct", Name: "RepeatResponse", 
		Extends: "Response", Fields: fields2, 
	Comment: "testing struct inheritance"})
	
	if !reflect.DeepEqual(expected.Meta, idl.Meta) {
		t.Errorf("idl.Meta mismatch: %v != %v", expected.Meta, idl.Meta)
	}

	if len(idl.Elems) != 11 {
		t.Errorf("idl.Elems len %d != 11", len(idl.Elems))
	}

	for i, ex := range expected.Elems {
		if !reflect.DeepEqual(ex, idl.Elems[i]) {
			t.Errorf("idl.Elems[%d] mismatch: %v != %v", i, ex, idl.Elems[i])
		}
	}
}

///////////////////////////////

type B interface {
  // simply returns s 
  // if s == "return-null" then you should return a null 
  Echo(s string) *string 
}

type BImpl struct { }

func (b BImpl) Echo(s string) (*string, *JsonRpcError) {
	if s == "return-null" {
		return nil, nil
	}
	return &s, nil
}

func TestServerCall(t *testing.T) {
	bimpl := BImpl{}
	idl := parseTestIdl()
	svr := NewServer(idl)
	svr.AddHandler("B", bimpl)
	res, err := svr.Call("B.echo", "hi"); if err != nil {
		panic(err)
	}

	resStr, ok := res.(*string); if !ok {
		s := fmt.Sprintf("B.echo return val cannot be converted to *string. type=%v", 
			reflect.TypeOf(res).Name())
		panic(s)
	}

	if *resStr != "hi" {
		t.Errorf("B.echo %s != hi", resStr)
	}
}