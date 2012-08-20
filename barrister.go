package barrister

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"reflect"
	"strings"
	"time"
)

var zeroVal reflect.Value

type TypeError struct {
	// string that describes location of value in the
	// param or return value graph.  e.g. param[0].Addresses[0].Street1
	path string

	msg string
}

func (e *TypeError) Error() string {
	return fmt.Sprintf("Type error: %s: %s", e.path, e.msg)
}

func EncodeASCII(b []byte) (string, error) {
	in := bytes.NewBuffer(b)
	out := bytes.NewBufferString("")
	for {
		r, size, err := in.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		if size == 1 {
			out.WriteRune(r)
		} else if size == 2 {
			out.WriteString(fmt.Sprintf("\\u%04x", r))
		} else {
			out.WriteString(fmt.Sprintf("\\U%08x", r))
		}
	}
	return out.String(), nil
}

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
		Elems:      elems,
		Interfaces: map[string]string{},
		Methods:    map[string]Function{},
		Structs:    map[string]Struct{},
		Enums:      map[string][]EnumValue{},
	}

	for _, el := range elems {
		if el.Type == "meta" {
			idl.Meta = Meta{el.BarristerVersion, el.DateGenerated * 1000000, el.Checksum}
		} else if el.Type == "interface" {
			idl.Interfaces[el.Name] = el.Name
			for _, f := range el.Functions {
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
	Name    string  `json:"name"`
	Comment string  `json:"comment"`
	Params  []Field `json:"params"`
	Returns Field   `json:"returns"`
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
	Elems []IdlJsonElem
	Meta  Meta

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

	for x, field := range meth.Params {
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
		_, ok := actual.(string)
		if !ok {
			return typeErr(expected, actual, path)
		}
	} else if expected.Type == "int" {
		t := reflect.TypeOf(actual).Name()
		if t == "int" || t == "int64" || t == "int8" || t == "int16" || t == "int32" {
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
	Jsonrpc string      `json:"jsonrpc"`
	Id      string      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type JsonRpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *JsonRpcError) Error() string {
	return fmt.Sprintf("JsonRpcError: code: %d message: %s", e.Code, e.Message)
}

type JsonRpcResponse struct {
	Jsonrpc string       `json:"jsonrpc"`
	Id     string        `json:"id"`
	Error  *JsonRpcError `json:"error,omitempty"`
	Result interface{}   `json:"result,omitempty"`
}

type BarristerIdlRpcResponse struct {
	Id     string        `json:"id"`
	Error  *JsonRpcError `json:"error,omitempty"`
	Result []IdlJsonElem `json:"result,omitempty"`
}

//////////////////////////////////////////////////
// Client //
////////////

type Transport interface {
	Call(method string, params ...interface{}) (interface{}, *JsonRpcError)
	CallBatch(batch []JsonRpcRequest) []JsonRpcResponse
}

type HttpTransport struct {
	Url string
}

func (t *HttpTransport) post(jsonReq interface{}) []byte {
	post, err := json.Marshal(jsonReq)
	if err != nil {
		panic(err)
	}

	//fmt.Printf("request:\n%s\n", post)

	req, err := http.NewRequest("POST", t.Url, bytes.NewBuffer(post))
	if err != nil {
		panic(err)
	}

	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	//fmt.Printf("%s\n\n", body)

	return body
}

func (t *HttpTransport) CallBatch(batch []JsonRpcRequest) []JsonRpcResponse {
	respBytes := t.post(batch)

	var batchResp []JsonRpcResponse
	err := json.Unmarshal(respBytes, &batchResp)
	if err != nil {
		panic(err)
	}

	return batchResp
}

func (t *HttpTransport) Call(method string, params ...interface{}) (interface{}, *JsonRpcError) {
	jsonReq := JsonRpcRequest{Jsonrpc: "2.0", Id: randStr(20), Method: method, Params: params}

	respBytes := t.post(jsonReq)

	jsonResp := JsonRpcResponse{}

	err := json.Unmarshal(respBytes, &jsonResp)
	if err != nil {
		panic(err)
	}

	if jsonResp.Error != nil {
		return nil, jsonResp.Error
	}

	return jsonResp.Result, nil
}

//////////////////////////////////////////////////
// Server //
////////////

func NewServer(idl *Idl) Server {
	return Server{idl, map[string]interface{}{}}
}

type Server struct {
	idl      *Idl
	handlers map[string]interface{}
}

func (s *Server) AddHandler(iface string, impl interface{}) {
	// TODO: verify that iface is in the idl
	s.handlers[iface] = impl
}

func (s *Server) InvokeJSON(j []byte) []byte {

	// determine if batch or single
	batch := false
	for i := 0; i < len(j); i++ {
		if j[i] == '{' {
			break
		} else if j[i] == '[' {
			batch = true
			break
		}
	}

	if batch {
		var batchReq []JsonRpcRequest
		batchResp := []JsonRpcResponse{}
		err := json.Unmarshal(j, &batchReq)
		if err != nil {
			return jsonParseErr("", err)
		}

		for _, req := range batchReq {
			resp := s.InvokeOne(&req)
			batchResp = append(batchResp, *resp)
		}

		b, _ := json.Marshal(batchResp)
		if err != nil {
			panic(err)
		}
		return b
	}

	//  - parse json into JsonRpcRequest
	rpcReq := JsonRpcRequest{}
	err := json.Unmarshal(j, &rpcReq)
	if err != nil {
		return jsonParseErr("", err)
	}

	resp := s.InvokeOne(&rpcReq)

	b, _ := json.Marshal(resp)
	if err != nil {
		panic(err)
	}
	return b
}

func (s *Server) InvokeOne(rpcReq *JsonRpcRequest) *JsonRpcResponse {
	var rpcerr *JsonRpcError

	if rpcReq.Method == "barrister-idl" {
		// handle 'barrister-idl' method
		return &JsonRpcResponse{Jsonrpc: "2.0", Id: rpcReq.Id, Result: s.idl.Elems}
	} else {
		// handle normal RPC method executions
		var result interface{}
		arr, ok := rpcReq.Params.([]interface{})
		if ok {
			result, rpcerr = s.Call(rpcReq.Method, arr...)
		} else {
			result, rpcerr = s.Call(rpcReq.Method, rpcReq.Params)
		}
		if rpcerr == nil {
			// successful Call
			return &JsonRpcResponse{Jsonrpc: "2.0", Id: rpcReq.Id, Result: result}
		}
	}

	// RPC error occurred
	return &JsonRpcResponse{Jsonrpc: "2.0", Id: rpcReq.Id, Error: rpcerr}
}

func (s *Server) Call(method string, params ...interface{}) (interface{}, *JsonRpcError) {
	iface, fname := ParseMethod(method)

	handler, ok := s.handlers[iface]
	if !ok {
		return nil, &JsonRpcError{Code: -32601, Message: fmt.Sprintf("No handler registered for interface: %s", iface)}
	}

	//err := s.idl.ValidateParams(method, params...)
	//if err != nil {
	//	return nil, err
	//}

	elem := reflect.ValueOf(handler)
	fn := elem.MethodByName(fname)
	if fn == zeroVal {
		return nil, &JsonRpcError{Code: -32601, Message: fmt.Sprintf("Function %s not found on handler %s", fname, iface)}
	}

	// check params
	fnType := fn.Type()
	if fnType.NumIn() != len(params) {
		return nil, &JsonRpcError{Code: -32602, Message: fmt.Sprintf("Method %s expects %d params but was passed %d", method, fnType.NumIn(), len(params))}
	}

	// convert params
	paramVals := []reflect.Value{}
	for x, param := range params {
		desiredType := fnType.In(x)
		path := fmt.Sprintf("param[%d]", x)
		converted, err := Convert(desiredType, param, path)
		if err != nil {
			return nil, &JsonRpcError{Code: -32602, Message: err.Error()}
		}
		paramVals = append(paramVals, converted)
		//fmt.Printf("%s - %v\n", path, reflect.TypeOf(converted.Interface()))
	}

	// make the call
	ret := fn.Call(paramVals)
	if len(ret) != 2 {
		return nil, &JsonRpcError{Code: -32603, Message: fmt.Sprintf("Method %s did not return 2 values. len(ret)=%d", method, len(ret))}
	}

	ret0 := ret[0].Interface()
	ret1 := ret[1].Interface()

	if ret1 != nil {
		rpcErr, ok := ret1.(*JsonRpcError)
		if !ok {
			return nil, &JsonRpcError{Code: -32603, Message: fmt.Sprintf("Method %s did not return JsonRpcError for last return val: %v", method, ret1)}
		}
		return ret0, rpcErr
	}

	//err = s.idl.ValidateResult(method, ret0)
	//if err != nil {
	//	return nil, err
	//}

	return ret0, nil
}

func Convert(desired reflect.Type, actual interface{}, path string) (reflect.Value, error) {
	kind := desired.Kind()

	if actual == nil {
		if kind == reflect.Ptr {
			return reflect.ValueOf(nil), nil
		} else {
			return zeroVal, &TypeError{path, "Unable to convert nil to non-pointer"}
		}
	}

	actType := reflect.TypeOf(actual)

	goal := fmt.Sprintf("convert: %v - %v to %v", path, actType.Kind().String(), desired)
	//fmt.Printf("%s\n", goal)

	if actType == desired {
		return reflect.ValueOf(actual), nil
	}

	desirePtr := false
	if kind == reflect.Ptr {
		desirePtr = true
		desired = desired.Elem()
		kind = desired.Kind()
	}

	if actType.Kind() == kind {
		//fmt.Printf("%v is assignable to %v\n", actType, desired)
		v := reflect.New(desired).Elem()
		switch kind {
		case reflect.String:
			s, ok := actual.(string)
			if ok {
				v.SetString(s)
				return returnVal(v, desirePtr)
			}
		}
	} else {
		//fmt.Printf("%v is NOT assignable to %v\n", actType, desired)
	}

	switch kind {
	case reflect.String:
		s, ok := actual.(string)
		if ok {
			return returnVal(reflect.ValueOf(string(s)), desirePtr)
		}
	case reflect.Int:
		s, ok := actual.(int)
		if ok {
			return returnVal(reflect.ValueOf(s), desirePtr)
		}
		s2, ok := actual.(int64)
		if ok {
			return returnVal(reflect.ValueOf(int(s2)), desirePtr)
		}
	case reflect.Int64:
		s, ok := actual.(int64)
		if ok {
			return returnVal(reflect.ValueOf(s), desirePtr)
		}
		s2, ok := actual.(int)
		if ok {
			return returnVal(reflect.ValueOf(int64(s2)), desirePtr)
		}
		s3, ok := actual.(float64)
		if ok {
			s4 := int64(s3)
			if float64(s4) == s3 {
				return returnVal(reflect.ValueOf(s4), desirePtr)
			}
		}
	case reflect.Float32:
		s, ok := actual.(float32)
		if ok {
			return returnVal(reflect.ValueOf(s), desirePtr)
		}
	case reflect.Float64:
		s, ok := actual.(float64)
		if ok {
			return returnVal(reflect.ValueOf(s), desirePtr)
		}
		s2, ok := actual.(float32)
		if ok {
			return returnVal(reflect.ValueOf(float64(s2)), desirePtr)
		}
	case reflect.Bool:
		b, ok := actual.(bool)
		if ok {
			return returnVal(reflect.ValueOf(b), desirePtr)
		}
	case reflect.Slice:
		actVal := reflect.ValueOf(actual)
		actType := actVal.Type()
		if actType.Kind() == reflect.Slice {
			return convertSlice(desired, actVal, path)
		}
	case reflect.Struct:
		m, ok := actual.(map[string]interface{})
		if ok {
			return convertStruct(desired, m, desirePtr, path)
		}
	}

	return zeroVal, &TypeError{path, "Unable to " + goal}
}

func convertSlice(desired reflect.Type, actVal reflect.Value, path string) (reflect.Value, error) {
	sliceType := desired.Elem()
	sliceV := reflect.New(desired)
	slice := sliceV.Elem()
	for x := 0; x < actVal.Len(); x++ {
		el := actVal.Index(x)
		pathCh := fmt.Sprintf("%s[%d]", path, x)
		conv, err := Convert(sliceType, el.Interface(), pathCh)
		if err != nil {
			return zeroVal, err
		}
		slice = reflect.Append(slice, conv)
	}
	return slice, nil
}

func convertStruct(desired reflect.Type, m map[string]interface{},
	desirePtr bool, path string) (reflect.Value, error) {

	val := reflect.New(desired)
	num := desired.NumField()
	for i := 0; i < num; i++ {
		fieldType := desired.Field(i)
		key := fieldType.Name
		mval, ok := m[key]
		if !ok {
			mval, ok = m[uncapitalize(key)]
		}
		if ok {
			conv, err := Convert(fieldType.Type, mval, path+"."+key)
			if err != nil {
				return zeroVal, err
			}
			f := val.Elem().Field(i)
			if f.Kind() == reflect.Ptr {
				if conv.Kind() == reflect.Ptr {
					f.Set(conv)
				} else {
					f.Set(conv.Addr())
				}
			} else {
				if conv.Kind() == reflect.Ptr {
					f.Set(conv.Elem())
				} else {
					f.Set(conv)
				}
			}
		}
	}
	return returnVal(val.Elem(), desirePtr)
}

func ParseMethod(method string) (string, string) {
	i := strings.Index(method, ".")
	if i > -1 && i < (len(method)-1) {
		iface := method[0:i]
		if i < (len(method) - 2) {
			return iface, strings.ToUpper(method[i+1:i+2]) + method[i+2:]
		} else {
			return iface, strings.ToUpper(method[i+1:])
		}
	}
	return method, ""
}

func capitalize(s string) string {
	switch len(s) {
	case 0:
		return s
	case 1:
		return strings.ToUpper(s)
	}
	return strings.ToUpper(s[0:1]) + s[1:]
}

func uncapitalize(s string) string {
	switch len(s) {
	case 0:
		return s
	case 1:
		return strings.ToLower(s)
	}
	return strings.ToLower(s[0:1]) + s[1:]
}

func jsonParseErr(reqId string, err error) []byte {
	rpcerr := &JsonRpcError{Code: -32700, Message: fmt.Sprintf("Unable to parse JSON: %s", err.Error())}
	resp := JsonRpcResponse{Jsonrpc: "2.0"}
	resp.Id = reqId
	resp.Error = rpcerr
	b, _ := json.Marshal(resp)
	return b
}

func returnVal(val reflect.Value, desirePtr bool) (reflect.Value, error) {
	if desirePtr {
		return val.Addr(), nil
	}
	return val, nil
}

func randStr(length int) string {
	rand.Seed(time.Now().UnixNano())
	b := bytes.Buffer{}
	for i := 0; i < length; i++ {
		x := rand.Int31n(36)
		if x < 10 {
			b.WriteString(string(48 + x))
		} else {
			b.WriteString(string(87 + x))
		}
	}
	return b.String()
}
