package barrister

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"reflect"
)

var zeroVal reflect.Value

//////////////////////////////////////////////////
// IDL //
/////////

func ParseIdlJson(jsonData []byte) (*Idl, error) {

	elems := []IdlJsonElem{}
	err := json.Unmarshal(jsonData, &elems)
	if err != nil {
		return nil, err
	}

	idl := &Idl{
		Elems: elems, 
		Interfaces: map[string]string{},
		Methods: map[string]Function{}, 
		Structs: map[string]Struct{},
		Enums: map[string][]EnumValue{},
	}

	for _, el := range elems {
		if el.Type == "meta" {
			idl.Meta = Meta{el.BarristerVersion, el.DateGenerated * 1000000, el.Checksum}
		} else if el.Type == "interface" {
			idl.Interfaces[el.Name] = el.Name
			for _, f := range(el.Functions) {
				meth := fmt.Sprintf("%s.%s", el.Name, f.Name)
				idl.Methods[meth] = f
			}
		} else if el.Type == "struct" {
			idl.Structs[el.Name] = Struct{Name: el.Name, Extends: el.Extends, Fields: el.Fields}
		} else if el.Type == "enum" {
			idl.Enums[el.Name] = el.Values
		}
	}

	return idl, nil
}

type IdlJsonElem struct {
	// common fields
	Type    string `json:"type"`
	Name    string `json:"name"`
	Comment string `json:"comment"`

	// type=comment
	Value string `json:"value"`

	// type=struct
	Extends string  `json:"extends"`
	Fields  []Field `json:"fields"`

	// type=enum
	Values []EnumValue `json:"values"`

	// type=interface
	Functions []Function `json:"functions"`

	// type=meta
	BarristerVersion string `json:"barrister_version"`
	DateGenerated    int64  `json:"date_generated"`
	Checksum         string `json:"checksum"`
}

type Function struct {
	Name     string   `json:"name"`
	Comment  string   `json:"comment"`
	Params   []Field  `json:"params"`
	Returns  Field    `json:"returns"`
}

type Struct struct {
	Name    string
	Extends string
	Fields  []Field
}

type Field struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Optional bool   `json:"optional"`
	IsArray  bool   `json:"is_array"`
	Comment  string `json:"comment"`
}

type EnumValue struct {
	Value   string `json:"value"`
	Comment string `json:"comment"`
}

type Meta struct {
	BarristerVersion string
	DateGenerated    int64
	Checksum         string
}

type Idl struct {
	// raw data from IDL file
	Elems   []IdlJsonElem
	Meta    Meta

	// hashed elements
	Interfaces map[string]string
	Methods    map[string]Function
	Structs    map[string]Struct
	Enums      map[string][]EnumValue
}

func (idl *Idl) ValidateParams(method string, params ...interface{}) *JsonRpcError {
	meth, ok := idl.Methods[method]
	if !ok {
		msg := fmt.Sprintf("Method not found: %s", method)
		return &JsonRpcError{Code: -32601, Message: msg}
	}

	if len(meth.Params) != len(params) {
		msg := fmt.Sprintf("Incorrect param count for method: %s - expected %d but got %d", 
			method, len(meth.Params), len(params))
		return &JsonRpcError{Code: -32602, Message: msg}
	}

	//fmt.Printf("ValidateParams: %v\n", params)

	for x, field := range(meth.Params) {
		path := fmt.Sprintf("param[%d]", x)
		err := idl.Validate(field, params[x], path)
		if err != nil {
			return &JsonRpcError{Code: -32602, Message: *err}
		}
	}

	return nil
}

func (idl *Idl) ValidateResult(method string, result interface{}) *JsonRpcError {
	meth, ok := idl.Methods[method]
	if !ok {
		msg := fmt.Sprintf("Method not found: %s", method)
		return &JsonRpcError{Code: -32601, Message: msg}
	}

	err := idl.Validate(meth.Returns, result, "")
	if err != nil {
		msg := fmt.Sprintf("Method %s returned invalid result: %s", method, *err)
		return &JsonRpcError{Code: -32001, Message: msg}
	}
	return nil
}

func typeErr(expected Field, actual interface{}, path string) *string {
	msg := fmt.Sprintf("Type mismatch for '%s' - Expected: %s Got: %v", path, expected.Type, reflect.TypeOf(actual).Name())
	return &msg
}

func nullErr(path string) *string {
	msg := fmt.Sprintf("Received null for required field: '%s'", path)
	return &msg
}

func (idl *Idl) Validate(expected Field, actual interface{}, path string) *string {
	//fmt.Printf("Validate:  field=%v    actual=%v\n", expected, actual)

	if actual == nil {
		if expected.Optional {
			return nil
		}
		return nullErr(path)
	}

	if expected.Type == "string" {
		_, ok := actual.(string); if !ok {
			return typeErr(expected, actual, path)
		}
	} else if expected.Type == "int" {
		t := reflect.TypeOf(actual).Name()
		if t == "int" || t == "int64" ||  t == "int8" || t == "int16" || t == "int32" {
			return nil
		}
		return typeErr(expected, actual, path)
	}

	return nil
}

func (idl *Idl) GenerateGo(pkgName string) []byte {
	s := bytes.Buffer{}
	s.WriteString(fmt.Sprintf("package %s\n\n", pkgName))

	for _, el := range idl.Elems {
		if el.Type == "comment" {
			for _, line := range strings.Split(el.Value, "\n") {
				s.WriteString("// ")
				s.WriteString(line)
				s.WriteString("\n")
			}
		}
		s.WriteString("\n")
	}

	return s.Bytes()
}

//////////////////////////////////////////////////
// Request / Response //
////////////////////////

type JsonRpcRequest struct {
    Id     string
    Method string
    Params interface{}
}

type JsonRpcError struct {
	Code      int          `json:"code"`
	Message   string       `json:"message"`
	Data      interface{}  `json:"data,omitempty"`
}

func (e *JsonRpcError) Error() string { 
	return fmt.Sprintf("JsonRpcError: code: %d message: %s", e.Code, e.Message);
}

type BaseJsonRpcResponse struct {
    Id      string
    Error   JsonRpcError `json:"error,omitempty"`
}

type JsonRpcResponse struct {
    Result  interface{}  `json:"result,omitempty"`
	BaseJsonRpcResponse
}

type BarristerIdlRpcResponse struct {
	Result []IdlJsonElem
	BaseJsonRpcResponse
}

//////////////////////////////////////////////////
// Server //
////////////

func NewServer(idl *Idl) Server {
	return Server{idl, map[string]interface{}{} }
}

type Server struct {
	idl         *Idl
	handlers    map[string]interface{}
}

func (s Server) AddHandler(iface string, impl interface{}) {
	// TODO: verify that iface is in the idl
	s.handlers[iface] = impl
}

func (s Server) InvokeJson(j []byte) []byte {

	//  - parse json into JsonRpcRequest
	rpcReq := JsonRpcRequest{}
	err := json.Unmarshal(j, &rpcReq)
	if err != nil {
		err := JsonRpcError{Code:-32700, Message:fmt.Sprintf("Unable to parse JSON: %s", err)}
		resp := JsonRpcResponse{}
		resp.Id = rpcReq.Id
		resp.Error = err
		b, _ := json.Marshal(resp)
		return b
	}

	var rpcerr *JsonRpcError

	if rpcReq.Method == "barrister-idl" {
		// handle 'barrister-idl' method
		resp := BarristerIdlRpcResponse{Result: s.idl.Elems}
		resp.Id = rpcReq.Id
		b, err := json.Marshal(resp); if err != nil {
			panic(err)
		}
		return b
	} else {
		// handle normal RPC method executions
		result, rpcerr := s.Call(rpcReq.Method, rpcReq.Params)

		if rpcerr == nil {
			// successful Call
			resp := JsonRpcResponse{Result: result}
			resp.Id = rpcReq.Id
			b, err := json.Marshal(resp)
			if err == nil {
				return b
			}

			msg := fmt.Sprintf("Unable to marshal response for method: %s - %v", rpcReq.Method, err)
			rpcerr = &JsonRpcError{Code: -32603, Message: msg}
		} 
	}

	// RPC error occurred
	resp := BaseJsonRpcResponse{Id: rpcReq.Id, Error: *rpcerr}
	b, _ := json.Marshal(resp); if err != nil {
		panic(err)
	}
	return b
}

func Convert(desired reflect.Type, actual interface{}) (reflect.Value, error) {
	//fmt.Printf("Convert: %s to %s\n", reflect.TypeOf(actual).Name(), desired.Name())
	return reflect.ValueOf(actual), nil
}

func (s Server) Call(method string, params ...interface{}) (interface{}, *JsonRpcError) {
	iface, fname := ParseMethod(method)

	handler, ok := s.handlers[iface]; if !ok {
		return nil, &JsonRpcError{Code:-32601, Message:fmt.Sprintf("No handler registered for interface: %s", iface)}
	}

	err := s.idl.ValidateParams(method, params...)
	if err != nil {
		return nil, err
	}

	elem := reflect.ValueOf(handler)
	fn := elem.MethodByName(fname)
	if fn == zeroVal {
		return nil, &JsonRpcError{Code:-32601, Message:fmt.Sprintf("Function %s not found on handler %s", fname, iface)}
	}

	// check params
	fnType := fn.Type()
	if fnType.NumIn() != len(params) {
		return nil, &JsonRpcError{Code:-32602, Message:fmt.Sprintf("Method %s expects %d params but was passed %d", method, fnType.NumIn(), len(params))}
	}

	// convert params
	paramVals := []reflect.Value{}
	for x, param := range params {
		desiredType := fnType.In(x)
		converted, err := Convert(desiredType, param)
		if err != nil {
			return nil, &JsonRpcError{Code:-32602, Message: err.Error()}
		}
		paramVals = append(paramVals, converted)
	}

	// make the call
	ret := fn.Call(paramVals)
	if len(ret) != 2 {
		return nil, &JsonRpcError{Code:-32603, Message:fmt.Sprintf("Method %s did not return 2 values. len(ret)=%d", method, len(ret))}
	}

	ret0 := ret[0].Interface()
	ret1 := ret[1].Interface()

	if ret1 != nil {
		rpcErr, ok := ret1.(*JsonRpcError); if !ok {
			return nil, &JsonRpcError{Code:-32603, Message:fmt.Sprintf("Method %s did not return JsonRpcError for last return val: %v", method, ret1)}
		}
		return ret0, rpcErr
	}

	err = s.idl.ValidateResult(method, ret0)
	if err != nil{
		return nil, err
	}

	return ret0, nil
}

func ParseMethod(method string) (string, string) {
	i := strings.Index(method, ".")
	if i > -1 && i < (len(method)-1) {
		iface := method[0:i]
		if i < (len(method)-2) {
			return iface, strings.ToUpper(method[i+1:i+2]) + method[i+2:]
		} else {
			return iface, strings.ToUpper(method[i+1:]);
		}
	}
	return method, ""
}
