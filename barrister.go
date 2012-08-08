package barrister

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"reflect"
)

var zeroVal reflect.Value

type IdlJsonElem struct {
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

	// type=meta
	BarristerVersion string `json:"barrister_version"`
	DateGenerated    int64  `json:"date_generated"`
	Checksum         string `json:"checksum"`
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
	Elems []IdlJsonElem
	Meta  Meta
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

type JsonRpcRequest struct {
    id     string
    method string
    params interface{}
}

type JsonRpcError struct {
	Code      int          `json:"code"`
	Message   string       `json:"message"`
	Data      interface{}  `json:"data,omitempty"`
}

func (e *JsonRpcError) Error() string { 
	return fmt.Sprintf("JsonRpcError: code: %d message: %s", e.Code, e.Message);
}

type JsonRpcResponse struct {
    id      string
    result  interface{}  `json:"result,omitempty"`
    error   JsonRpcError `json:"error,omitempty"`
}

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
	// TODO
	//  - parse json into JsonRpcRequest
	rpcReq := JsonRpcRequest{}
	err := json.Unmarshal(j, &rpcReq)
	if err != nil {
		err := JsonRpcError{Code:-32700, Message:fmt.Sprintf("Unable to parse JSON: %s", err)}
		resp := JsonRpcResponse{id: rpcReq.id, error: err}
		b, _ := json.Marshal(resp)
		return b
	}

	// - handle 'barrister-idl' method
	if rpcReq.method == "barrister-idl" {

	} else {
		s.Call(rpcReq.method, rpcReq.params)
	}

	//  - find handler based on method

	//  - if found, marshal params to correct types, validating against idl
	//  - invoke handler func using reflect

	//ret := fn.Call(nil)
	//if len(ret) > 0 && !ret[0].IsNil() {
	//	return ret[0].Interface().(error)
	//}

	//  - marshal return val

	return j
}

func (s Server) Call(method string, args ...interface{}) (interface{}, *JsonRpcError) {
	iface, fname := ParseMethod(method)

	handler, ok := s.handlers[iface]; if !ok {
		return nil, &JsonRpcError{Code:-32601, Message:fmt.Sprintf("No handler registered for interface: %s", iface)}
	}

	elem := reflect.ValueOf(handler)
	fn := elem.MethodByName(fname)
	if fn == zeroVal {
		return nil, &JsonRpcError{Code:-32601, Message:fmt.Sprintf("Function %s not found on handler %s", fname, iface)}
	}

	// check args
	fnType := fn.Type()
	if fnType.NumIn() != len(args) {
		return nil, &JsonRpcError{Code:-32602, Message:fmt.Sprintf("Method %s expects %d params but was passed %d", method, fnType.NumIn(), len(args))}
	}

	// convert args
	argVals := []reflect.Value{}
	for _, arg := range args {
		argVals = append(argVals, reflect.ValueOf(arg))
	}

	// make the call
	ret := fn.Call(argVals)
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

	return ret0, nil
}

func ParseIdlJson(jsonData []byte) (*Idl, error) {

	elems := []IdlJsonElem{}
	err := json.Unmarshal(jsonData, &elems)
	if err != nil {
		return nil, err
	}

	idl := &Idl{Elems: elems}
	for _, el := range elems {
		if el.Type == "meta" {
			idl.Meta = Meta{el.BarristerVersion, el.DateGenerated * 1000000, el.Checksum}
		}
	}

	return idl, nil
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
