package barrister

import (
	"encoding/json"
	"fmt"
	. "github.com/sdegutis/go.assert"
	"io/ioutil"
	"math"
	"reflect"
	"testing"
)

// enums from conform.idl
type Status string

const (
	StatusOk  Status = "ok"
	StatusErr        = "err"
)

type MathOp string

const (
	MathOpAdd      MathOp = "add"
	MathOpMultiply        = "multiply"
)

// structs from conform.idl
type Response struct {
	Status Status `json:"status"`
}

type RepeatResponse struct {
	Status Status   `json:"status"`
	Count  int      `json:"count"`
	Items  []string `json:"items"`
}

type HiResponse struct {
	Hi string `json:"hi"`
}

type RepeatRequest struct {
	To_repeat       string `json:"to_repeat"`
	Count           int64  `json:"count"`
	Force_uppercase bool   `json:"force_uppercase"`
}

type Person struct {
	PersonId  string  `json:"personId"`
	FirstName string  `json:"firstName"`
	LastName  string  `json:"lastName"`
	Email     *string `json:"email"`
}

// implementation of "A" interface from conform.idl
type AImpl struct{}

func (i AImpl) Add(a int64, b int64) (int64, error) {
	return a + b, nil
}

func (i AImpl) Calc(nums []float64, operation MathOp) (float64, error) {
	switch operation {
	case MathOpAdd:
		sum := float64(0)
		for i := 0; i < len(nums); i++ {
			sum += nums[i]
		}
		return sum, nil
	case MathOpMultiply:
		x := float64(1)
		for i := 0; i < len(nums); i++ {
			x = x * nums[i]
		}
		return x, nil
	}

	msg := fmt.Sprintf("Unknown operation: %s", operation)
	return 0, &JsonRpcError{Code: -32000, Message: msg}
}

func (i AImpl) Sqrt(a float64) (float64, error) {
	return math.Sqrt(a), nil
}

func (i AImpl) Repeat(req1 RepeatRequest) (RepeatResponse, error) {
	return RepeatResponse{}, nil
}

func (i AImpl) Say_hi() (HiResponse, error) {
	return HiResponse{"hi"}, nil
}

func (i AImpl) Repeat_num(num int64, count int64) ([]int64, error) {
	arr := []int64{}
	return arr, nil
}

func (i AImpl) PutPerson(p Person) (string, error) {
	return p.PersonId, nil
}

type BImpl struct{}

// implementation of "B" interface from conform.idl
func (i BImpl) Echo(s string) (*string, error) {
	if s == "return-null" {
		return nil, nil
	}
	return &s, nil
}

type BImpl_MissingFunc struct{}

type BImpl_BadParam struct{}

func (b BImpl_BadParam) Echo(f float64) (*string, error) {
	s := "blah"
	return &s, nil
}

type BImpl_BadReturn struct{}

func (b BImpl_BadReturn) Echo(s string) (int, error) {
	return 10, nil
}

type BImpl_BadReturn2 struct{}

func (b BImpl_BadReturn2) Echo(s string) (*string, int) {
	s2 := "blah"
	return &s2, 0
}

type BImpl_BadReturn3 struct{}

func (b BImpl_BadReturn3) Echo(s string) *string {
	s2 := "blah"
	return &s2
}

type CallFail struct {
	method  string
	errcode int
}

type GenericCall struct {
	method  string
	params  []interface{}
	result  interface{}
	errcode int
}

type EchoCall struct {
	in  string
	out interface{}
}

func readFile(fname string) []byte {
	b, err := ioutil.ReadFile(fname)
	if err != nil {
		panic(err)
	}
	return b
}

func readConformJson() []byte {
	return readFile("test/conform.json")
}

func parseTestIdl() *Idl {
	idl, err := ParseIdlJson(readConformJson())
	if err != nil {
		panic(err)
	}
	return idl
}

///////////////////////////////////

func TestServerCallSuccess(t *testing.T) {
	aimpl := AImpl{}
	bimpl := BImpl{}
	idl := parseTestIdl()
	svr := NewJSONServer(idl, true)
	svr.AddHandler("A", aimpl)
	svr.AddHandler("B", bimpl)

	genericCalls := []GenericCall{
		GenericCall{"A.add", []interface{}{1, 2}, int64(3), 0},
		GenericCall{"A.sqrt", []interface{}{16}, float64(4), 0},
		GenericCall{"A.say_hi", []interface{}{}, HiResponse{"hi"}, 0},
		GenericCall{"A.calc", []interface{}{[]float64{2, 3}, "multiply"}, float64(6), 0},
	}

	for x, generic := range genericCalls {
		retval := svr.Call(generic.method, generic.params...)
		res := retval.result
		err := toJsonRpcError(generic.method, retval.err)

		if err == nil {
			if generic.errcode == 0 {
				if !reflect.DeepEqual(generic.result, res) {
					t.Errorf("generic[%d] - %v != %v", x, generic.result, res)
				}
			} else {
				t.Errorf("generic[%d] - expected error, but got result: %v", x, res)
			}
		} else {
			if generic.errcode == 0 {
				t.Errorf("generic[%d] - expected success, got err: %v", x, err)
			} else if generic.errcode != err.Code {
				t.Errorf("generic[%d] - expected errcode %d, got err: %v", x, generic.errcode, err)
			}
		}
	}

	calls := []EchoCall{
		EchoCall{"hi", "hi"},
		EchoCall{"2", "2"},
		EchoCall{"return-null", nil},
	}

	for _, call := range calls {
		retval := svr.Call("B.echo", call.in)
		if retval.err != nil {
			t.Fatalf("B.echo retval.err !=nil - result=%v err=%v", retval.result, retval.err)
		}

		resStr, ok := retval.result.(*string)
		if !ok {
			s := fmt.Sprintf("B.echo return val cannot be converted to *string. type=%v",
				reflect.TypeOf(retval.result).Name())
			t.Fatal(s)
		}

		if !((resStr == nil && call.out == nil) || (*resStr == call.out)) {
			t.Errorf("B.echo %v != %v", resStr, call.out)
		}
	}
}

func TestServerCallFail(t *testing.T) {
	bimpl := BImpl{}
	idl := parseTestIdl()
	svr := NewJSONServer(idl, true)
	svr.AddHandler("B", bimpl)

	calls := []CallFail{
		CallFail{"B.", -32601},
		CallFail{"", -32601},
		CallFail{"B.foo", -32601},
		CallFail{"B.echo", -32602},
	}

	for _, call := range calls {
		retval := svr.Call(call.method)
		res := retval.result
		err := retval.err.(*JsonRpcError)
		if res != nil {
			t.Errorf("%v != nil on expected fail call: %s", res, call.method)
		} else if err == nil {
			t.Errorf("err == nil on expected fail call: %s", call.method)
		} else if err.Code != call.errcode {
			t.Errorf("errcode %d != %d on expected fail call: %s", err.Code,
				call.errcode, call.method)
		}
	}
}

func TestServerInvokeJSONSuccess(t *testing.T) {
	bimpl := BImpl{}
	idl := parseTestIdl()
	svr := NewJSONServer(idl, true)
	svr.AddHandler("B", bimpl)

	calls := []EchoCall{
		EchoCall{"hi", "hi"},
		EchoCall{"2", "2"},
		EchoCall{"return-null", nil},
	}

	for _, call := range calls {
		req := JsonRpcRequest{Id: "123", Method: "B.echo", Params: []interface{}{call.in}}
		reqBytes, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}

		resBytes := svr.InvokeBytes(reqBytes)
		resp := JsonRpcResponse{}
		err = json.Unmarshal(resBytes, &resp)
		if err != nil {
			t.Fatal(err)
		}

		if resp.Error != nil {
			t.Errorf("B.echo %v returned err: %v", call.in, resp.Error)
		} else {
			res := resp.Result
			if res == nil {
				if call.out != nil {
					t.Errorf("B.echo nil != %v", call.out)
				}
			} else {
				resStr, ok := res.(string)
				if !ok {
					n := reflect.TypeOf(res).Name()
					t.Errorf("B.echo return val cannot be converted to string. type=%v", n)
				}

				if resStr != call.out {
					t.Errorf("B.echo %v != %v", resStr, call.out)
				}
			}
		}

	}
}

func TestAddHandlerPanicsIfImplDoesntMatchInterface(t *testing.T) {
	idl := parseTestIdl()
	svr := NewJSONServer(idl, true)

	badHandlers := []interface{}{
		BImpl_MissingFunc{},
		BImpl_BadParam{},
		BImpl_BadReturn{},
		BImpl_BadReturn2{},
		BImpl_BadReturn3{},
	}

	for x, handler := range badHandlers {
		fx := func() {
			defer func() {
				if r := recover(); r != nil {
					// ok
				}
			}()
			svr.AddHandler("B", handler)
			t.Errorf("[%d] - AddHandler() allowed invalid handler impl", x)
		}
		fx()
	}
}

func TestParseIdlJson(t *testing.T) {
	idl := parseTestIdl()

	meta := Meta{BarristerVersion: "0.1.2", DateGenerated: 1337654725230000000, Checksum: "34f6238ed03c6319017382e0fdc638a7"}

	expected := Idl{Meta: meta}
	expected.elems = append(expected.elems, IdlJsonElem{Type: "comment", Value: "Barrister conformance IDL\n\nThe bits in here have silly names and the operations\nare not intended to be useful.  The intent is to\nexercise as much of the IDL grammar as possible"})

	enumVals := []EnumValue{
		EnumValue{Value: "ok"},
		EnumValue{Value: "err"},
	}
	expected.elems = append(expected.elems,
		IdlJsonElem{Type: "enum", Name: "Status", Values: enumVals})

	enumVals2 := []EnumValue{
		EnumValue{Value: "add"},
		EnumValue{Value: "multiply", Comment: "mult comment"},
	}
	expected.elems = append(expected.elems,
		IdlJsonElem{Type: "enum", Name: "MathOp", Values: enumVals2})

	fields := []Field{
		Field{Optional: false, IsArray: false, Type: "Status", Name: "status"},
	}
	expected.elems = append(expected.elems, IdlJsonElem{
		Type: "struct", Name: "Response", Fields: fields})

	fields2 := []Field{
		Field{Optional: false, IsArray: false, Type: "int", Name: "count"},
		Field{Optional: false, IsArray: true, Type: "string", Name: "items"},
	}
	expected.elems = append(expected.elems,
		IdlJsonElem{Type: "struct", Name: "RepeatResponse",
			Extends: "Response", Fields: fields2,
			Comment: "testing struct inheritance"})

	DeepEquals(t, expected.Meta, idl.Meta)
	Equals(t, len(idl.elems), 11)

	for i, ex := range expected.elems {
		DeepEquals(t, ex, idl.elems[i])
	}

	Equals(t, 2, len(idl.interfaces))
	Equals(t, 7, len(idl.interfaces["A"]))
	Equals(t, 1, len(idl.interfaces["B"]))

	methodKeys := []string{
		"A.add", "A.calc", "A.sqrt", "A.repeat", "A.say_hi",
		"A.repeat_num", "A.putPerson", "B.echo",
	}
	for _, key := range methodKeys {
		_, ok := idl.methods[key]
		if !ok {
			t.Errorf("No method with key: %s", key)
		}
	}

	structKeys := []string{
		"Response", "RepeatResponse", "HiResponse", "RepeatRequest", "Person",
	}
	for _, key := range structKeys {
		_, ok := idl.structs[key]
		if !ok {
			t.Errorf("No struct with key: %s", key)
		}
	}

	enumKeys := []string{
		"Status", "MathOp",
	}
	for _, key := range enumKeys {
		_, ok := idl.enums[key]
		if !ok {
			t.Errorf("No enum with key: %s", key)
		}
	}

	mathOps := []EnumValue{
		EnumValue{"add", ""},
		EnumValue{"multiply", "mult comment"},
	}
	if !reflect.DeepEqual(idl.enums["MathOp"], mathOps) {
		t.Errorf("MathOp enum: %v != %v", idl.enums["MathOp"], mathOps)
	}

}

func TestServerBarristerIdl(t *testing.T) {
	idl := parseTestIdl()
	svr := NewJSONServer(idl, true)

	rpcReq := JsonRpcRequest{Id: "123", Method: "barrister-idl", Params: ""}
	reqJson, _ := json.Marshal(rpcReq)
	respJson := svr.InvokeBytes(reqJson)
	rpcResp := BarristerIdlRpcResponse{}
	err := json.Unmarshal(respJson, &rpcResp)
	if err != nil {
		panic(err)
	}

	//fmt.Printf("%v\n", rpcResp.Result)

	DeepEquals(t, idl.elems, rpcResp.Result)
}
