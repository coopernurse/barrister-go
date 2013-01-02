package barrister

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
)

var zeroVal reflect.Value

func RandStr(bytes int) string {
	buf := make([]byte, bytes)
	io.ReadFull(rand.Reader, buf)
	return fmt.Sprintf("%x", buf)
}

//////////////////////////////////////////////////
// IDL //
/////////

func ParseIdlJsonFile(fname string) (*Idl, error) {
	b, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	return ParseIdlJson(b)
}

func ParseIdlJson(jsonData []byte) (*Idl, error) {

	elems := []IdlJsonElem{}
	err := json.Unmarshal(jsonData, &elems)
	if err != nil {
		return nil, err
	}

	return NewIdl(elems), nil
}

func MustParseIdlJson(jsonData []byte) *Idl {
	idl, err := ParseIdlJson(jsonData)
	if err != nil {
		panic(err)
	}
	return idl
}

func NewIdl(elems []IdlJsonElem) *Idl {
	idl := &Idl{
		elems:      elems,
		interfaces: map[string][]Function{},
		methods:    map[string]Function{},
		structs:    map[string]*Struct{},
		enums:      map[string][]EnumValue{},
	}

	for _, el := range elems {
		if el.Type == "meta" {
			idl.Meta = Meta{el.BarristerVersion, el.DateGenerated * 1000000, el.Checksum}
		} else if el.Type == "interface" {
			funcs := []Function{}
			for _, f := range el.Functions {
				meth := fmt.Sprintf("%s.%s", el.Name, f.Name)
				idl.methods[meth] = f
				funcs = append(funcs, f)
			}
			idl.interfaces[el.Name] = funcs
		} else if el.Type == "struct" {
			idl.structs[el.Name] = &Struct{Name: el.Name, Extends: el.Extends, Fields: el.Fields}
		} else if el.Type == "enum" {
			idl.enums[el.Name] = el.Values
		}
	}

	idl.computeAllStructFields()

	return idl
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

	// fields in this struct, and its parents
	allFields []Field
}

type Field struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Optional bool   `json:"optional"`
	IsArray  bool   `json:"is_array"`
	Comment  string `json:"comment"`
}

func (f Field) goType(optionalToPtr bool) string {
	if f.IsArray {
		f2 := Field{f.Name, f.Type, f.Optional, false, ""}
		return "[]" + f2.goType(optionalToPtr)
	}

	prefix := ""
	if optionalToPtr && f.Optional {
		prefix = "*"
	}

	switch f.Type {
	case "string":
		return prefix + "string"
	case "int":
		return prefix + "int64"
	case "float":
		return prefix + "float64"
	case "bool":
		return prefix + "bool"
	}

	return prefix + f.Type
}

func (f Field) zeroVal(idl *Idl, optionalToPtr bool) interface{} {

	if f.Optional && optionalToPtr {
		return "nil"
	}

	if f.IsArray {
		return f.goType(false) + "{}"
	}

	switch f.Type {
	case "string":
		return `""`
	case "int":
		return "int64(0)"
	case "float":
		return "float64(0)"
	case "bool":
		return "false"
	}

	s, ok := idl.structs[f.Type]
	if ok {
		return capitalize(s.Name) + "{}"
	}

	e, ok := idl.enums[f.Type]
	if ok && len(e) > 0 {
		return `""`
	}

	msg := fmt.Sprintf("Unable to create val for field: %s type: %s",
		f.Name, f.Type)
	panic(msg)
}

func (f Field) testVal(idl *Idl) interface{} {

	if f.IsArray {
		f2 := Field{f.Name, f.Type, f.Optional, false, ""}
		arr := []interface{}{}
		arr = append(arr, f2.testVal(idl))
		return arr
	}

	switch f.Type {
	case "string":
		return "testval"
	case "int":
		return int64(99)
	case "float":
		return float64(10.3)
	case "bool":
		return true
	}

	s, ok := idl.structs[f.Type]
	if ok {
		val := map[string]interface{}{}
		for _, f2 := range s.allFields {
			val[f2.Name] = f2.testVal(idl)
		}
		return val
	}

	e, ok := idl.enums[f.Type]
	if ok && len(e) > 0 {
		return e[0].Value
	}

	msg := fmt.Sprintf("Unable to create val for field: %s type: %s",
		f.Name, f.Type)
	panic(msg)
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
	elems []IdlJsonElem

	// meta information about the contract
	Meta Meta

	// hashed elements
	interfaces map[string][]Function
	methods    map[string]Function
	structs    map[string]*Struct
	enums      map[string][]EnumValue
}

func (idl *Idl) computeAllStructFields() {
	for _, s := range idl.structs {
		s.allFields = idl.computeStructFields(s, []Field{})
	}
}

func (idl *Idl) computeStructFields(toAdd *Struct, allFields []Field) []Field {
	if toAdd.Extends != "" {
		parent, ok := idl.structs[toAdd.Extends]
		if ok {
			allFields = idl.computeStructFields(parent, allFields)
		}
	}

	for _, f := range toAdd.Fields {
		allFields = append(allFields, f)
	}

	return allFields
}

func (idl *Idl) GenerateGo(pkgName string, optionalToPtr bool) []byte {
	g := generateGo{idl, pkgName, optionalToPtr}
	return g.generate()
}

func (idl *Idl) Method(name string) Function {
	return idl.methods[name]
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
	Jsonrpc string        `json:"jsonrpc"`
	Id      string        `json:"id"`
	Error   *JsonRpcError `json:"error,omitempty"`
	Result  interface{}   `json:"result,omitempty"`
}

type BarristerIdlRpcResponse struct {
	Id     string        `json:"id"`
	Error  *JsonRpcError `json:"error,omitempty"`
	Result []IdlJsonElem `json:"result,omitempty"`
}

// RequestResponse holds the request method and params and the
// handler instance that the method resolves to.
//
// This struct is used with the Filter interface and allows
// Filter implementations to inspect the request, mutate the
// handler (e.g. set out of band authentication information),
// and set the result/error (e.g. to terminate an unauthorized request)
type RequestResponse struct {
	// from Transport
	Headers map[string][]string

	// from JsonRpcRequest
	Method string
	Params []interface{}

	// handler instance that will process
	// this request method - this is passed
	// by value, so pointer fields in the handler
	// may be modified by filters
	Handler interface{}

	// to JsonRpcResponse
	Result interface{}
	Err    error
}

func (rr RequestResponse) GetHeader(key string) string {
	xs, ok := rr.Headers[key]
	if ok && len(xs) > 0 {
		return xs[0]
	}

	return ""
}

func toJsonRpcError(method string, err error) *JsonRpcError {
	if err == nil {
		return nil
	}

	e, ok := err.(*JsonRpcError)
	if ok {
		return e
	}
	msg := fmt.Sprintf("barrister: method '%s' raised unknown error: %v", method, err)
	return &JsonRpcError{Code: -32000, Message: msg}
}

//////////////////////////////////////////////////
// Client //
////////////

func EncodeASCII(b []byte) (*bytes.Buffer, error) {
	in := bytes.NewBuffer(b)
	out := bytes.NewBufferString("")
	for {
		r, size, err := in.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if size == 1 {
			out.WriteRune(r)
		} else if size == 2 {
			out.WriteString(fmt.Sprintf("\\u%04x", r))
		} else {
			out.WriteString(fmt.Sprintf("\\U%08x", r))
		}
	}
	return out, nil
}

type Serializer interface {
	Marshal(in interface{}) ([]byte, error)
	Unmarshal(in []byte, out interface{}) error
	IsBatch(b []byte) bool
	MimeType() string
}

type JsonSerializer struct {
	ForceASCII bool
}

func (s *JsonSerializer) Marshal(in interface{}) ([]byte, error) {
	b, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}

	if s.ForceASCII {
		buf, err := EncodeASCII(b)
		if err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	return b, nil
}

func (s *JsonSerializer) Unmarshal(in []byte, out interface{}) error {
	return json.Unmarshal(in, out)
}

func (s *JsonSerializer) IsBatch(b []byte) bool {
	batch := false
	for i := 0; i < len(b); i++ {
		if b[i] == '{' {
			break
		} else if b[i] == '[' {
			batch = true
			break
		}
	}
	return batch
}

func (s *JsonSerializer) MimeType() string {
	return "application/json"
}

type Transport interface {
	Send(in []byte) ([]byte, error)
}

type HttpClientVisitor interface {
	BeforeRequest(req *http.Request)
	AfterRequest(req *http.Request, resp *http.Response)
}

type HttpTransport struct {
	Url     string
	Visitor HttpClientVisitor
	Jar     http.CookieJar
}

func (t *HttpTransport) Send(in []byte) ([]byte, error) {

	//fmt.Printf("request:\n%s\n", post)

	req, err := http.NewRequest("POST", t.Url, bytes.NewBuffer(in))
	if err != nil {
		msg := fmt.Sprintf("barrister: HttpTransport NewRequest failed: %s", err)
		return nil, errors.New(msg)
	}

	req.Header.Add("Content-Type", "application/json")

	if t.Visitor != nil {
		t.Visitor.BeforeRequest(req)
	}

	client := &http.Client{}
	if t.Jar != nil {
		client.Jar = t.Jar
	}
	resp, err := client.Do(req)
	if err != nil {
		msg := fmt.Sprintf("barrister: HttpTransport POST to %s failed: %s", t.Url, err)
		return nil, errors.New(msg)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		msg := fmt.Sprintf("barrister: HttpTransport Unable to read resp.Body: %s", err)
		return nil, errors.New(msg)
	}

	if t.Visitor != nil {
		t.Visitor.AfterRequest(req, resp)
	}

	//fmt.Printf("%s\n\n", body)

	return body, nil
}

type Client interface {
	Call(method string, params ...interface{}) (interface{}, error)
	CallBatch(batch []JsonRpcRequest) []JsonRpcResponse
}

func NewRemoteClient(trans Transport, forceASCII bool) Client {
	return &RemoteClient{trans, &JsonSerializer{forceASCII}}
}

type RemoteClient struct {
	Trans Transport
	Ser   Serializer
}

func (c *RemoteClient) CallBatch(batch []JsonRpcRequest) []JsonRpcResponse {
	reqBytes, err := c.Ser.Marshal(batch)
	if err != nil {
		msg := fmt.Sprintf("barrister: CallBatch unable to Marshal request: %s", err)
		return []JsonRpcResponse{
			JsonRpcResponse{Error: &JsonRpcError{Code: -32600, Message: msg}}}
	}

	respBytes, err := c.Trans.Send(reqBytes)
	if err != nil {
		msg := fmt.Sprintf("barrister: CallBatch Transport error during request: %s", err)
		return []JsonRpcResponse{
			JsonRpcResponse{Error: &JsonRpcError{Code: -32603, Message: msg}}}
	}

	var batchResp []JsonRpcResponse
	err = c.Ser.Unmarshal(respBytes, &batchResp)
	if err != nil {
		msg := fmt.Sprintf("barrister: CallBatch unable to Unmarshal response: %s", err)
		return []JsonRpcResponse{
			JsonRpcResponse{Error: &JsonRpcError{Code: -32603, Message: msg}}}
	}

	return batchResp
}

func (c *RemoteClient) Call(method string, params ...interface{}) (interface{}, error) {
	rpcReq := JsonRpcRequest{Jsonrpc: "2.0", Id: RandStr(20), Method: method, Params: params}

	reqBytes, err := c.Ser.Marshal(rpcReq)
	if err != nil {
		msg := fmt.Sprintf("barrister: %s: Call unable to Marshal request: %s", method, err)
		return nil, &JsonRpcError{Code: -32600, Message: msg}
	}

	respBytes, err := c.Trans.Send(reqBytes)
	if err != nil {
		msg := fmt.Sprintf("barrister: %s: Transport error during request: %s", method, err)
		return nil, &JsonRpcError{Code: -32603, Message: msg}
	}

	var rpcResp JsonRpcResponse
	err = c.Ser.Unmarshal(respBytes, &rpcResp)
	if err != nil {
		msg := fmt.Sprintf("barrister: %s: Call unable to Unmarshal response: %s", method, err)
		return nil, &JsonRpcError{Code: -32603, Message: msg}
	}

	if rpcResp.Error != nil {
		return nil, rpcResp.Error
	}

	return rpcResp.Result, nil
}

//////////////////////////////////////////////////
// Server //
////////////

// If a server handler implements Cloneable, it will
// be cloned per JSON-RPC call.  Used in conjunction with
// Filters, this allows you to initialize out of band context 
// that your service implementation may need.  The most common
// example is security information.  A Filter might check HTTP
// headers and resolve the caller's identity and role and set that
// as context on the cloned service implementation.
//
// Another use case is thread safety.  By implementing Cloneable
// your services no longer need to be threadsafe, and can safely store
// state locally for the lifetime of the service method invocation.
//
type Cloneable interface {
	Clone() interface{}
}

// Filters allow you to intercept requests before and after the handler method
// is invoked.  Filters are useful for implementing cross cutting concerns
// such as authentication, performance measurement, logging, etc.
type Filter interface {

	// PreInvoke is called after the handler has been resolved, but prior
	// to handler method invocation.  
	//
	// Return value of false terminates the filter chain, and r.result, r.err
	// will be used as the response.  
	// Return value of true continues filter chain execution.
	//
	PreInvoke(r *RequestResponse) bool

	// PostInvoke is called after the handler method has been invoked and
	// returns a bool that indicates whether later filters should be called.
	//
	// Implementations may alter the ReturnVal, which will be later marshaled
	// into the JSON-RPC response.
	//
	// Return value of false terminates the filter chain, and r.result, r.err
	// will be used.  Return value of true continues filter chain execution.
	//
	PostInvoke(r *RequestResponse) bool
}

func NewJSONServer(idl *Idl, forceASCII bool) Server {
	return NewServer(idl, &JsonSerializer{forceASCII})
}

func NewServer(idl *Idl, ser Serializer) Server {
	return Server{idl, ser, map[string]interface{}{}, make([]Filter, 0)}
}

type Server struct {
	idl      *Idl
	ser      Serializer
	handlers map[string]interface{}
	filters  []Filter
}

// AddFilter registers a Filter implementation with the Server. 
// 
// * Filter.PreInvoke is called in the order of registration
// * Filter.PostInvoke is called in reverse order of registration
//
func (s *Server) AddFilter(f Filter) {
	s.filters = append(s.filters, f)
}

func (s *Server) AddHandler(iface string, impl interface{}) {
	ifaceFuncs, ok := s.idl.interfaces[iface]

	if !ok {
		msg := fmt.Sprintf("barrister: IDL has no interface: %s", iface)
		panic(msg)
	}

	var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

	elem := reflect.ValueOf(impl)
	for _, idlFunc := range ifaceFuncs {
		fname := capitalize(idlFunc.Name)
		fn := elem.MethodByName(fname)
		if fn == zeroVal {
			msg := fmt.Sprintf("barrister: %s impl has no method named: %s",
				iface, fname)
			panic(msg)
		}

		fnType := fn.Type()
		if fnType.NumIn() != len(idlFunc.Params) {
			msg := fmt.Sprintf("barrister: %s impl method: %s accepts %d params but IDL specifies %d", iface, fname, fnType.NumIn(), len(idlFunc.Params))
			panic(msg)
		}

		if fnType.NumOut() != 2 {
			msg := fmt.Sprintf("barrister: %s impl method: %s returns %d params but must be 2", iface, fname, fnType.NumOut())
			panic(msg)
		}

		for x, param := range idlFunc.Params {
			path := fmt.Sprintf("%s.%s param[%d]", iface, fname, x)
			s.validate(param, fnType.In(x), path)
		}

		path := fmt.Sprintf("%s.%s return value[0]", iface, fname)
		s.validate(idlFunc.Returns, fnType.Out(0), path)

		errType := fnType.Out(1)
		if !errType.Implements(typeOfError) {
			msg := fmt.Sprintf("%s.%s return value[1] has invalid type: %s (expected: error)", iface, fname, errType)
			panic(msg)
		}
	}

	s.handlers[iface] = impl
}

func (s *Server) validate(idlField Field, implType reflect.Type, path string) {
	testVal := idlField.testVal(s.idl)
	conv := newConvert(s.idl, &idlField, implType, testVal, "")
	_, err := conv.run()
	if err != nil {
		msg := fmt.Sprintf("barrister: %s has invalid type: %s reason: %s", path, implType, err)
		panic(msg)
	}
}

func (s *Server) InvokeBytes(headers map[string][]string, req []byte) []byte {

	// determine if batch or single
	batch := s.ser.IsBatch(req)

	// batch execution
	if batch {
		var batchReq []JsonRpcRequest
		batchResp := []JsonRpcResponse{}
		err := s.ser.Unmarshal(req, &batchReq)
		if err != nil {
			return jsonParseErr("", true, err)
		}

		for _, req := range batchReq {
			resp := s.InvokeOne(headers, &req)
			batchResp = append(batchResp, *resp)
		}

		b, err := s.ser.Marshal(batchResp)
		if err != nil {
			panic(err)
		}
		return b
	}

	// single request execution
	rpcReq := JsonRpcRequest{}
	err := s.ser.Unmarshal(req, &rpcReq)
	if err != nil {
		return jsonParseErr("", false, err)
	}

	resp := s.InvokeOne(headers, &rpcReq)

	b, err := s.ser.Marshal(resp)
	if err != nil {
		panic(err)
	}
	return b
}

func (s *Server) InvokeOne(headers map[string][]string, rpcReq *JsonRpcRequest) *JsonRpcResponse {
	if rpcReq.Method == "barrister-idl" {
		// handle 'barrister-idl' method
		return &JsonRpcResponse{Jsonrpc: "2.0", Id: rpcReq.Id, Result: s.idl.elems}
	}

	// handle normal RPC method executions
	var result interface{}
	var err error
	arr, ok := rpcReq.Params.([]interface{})
	if ok {
		result, err = s.Call(headers, rpcReq.Method, arr...)
	} else {
		result, err = s.Call(headers, rpcReq.Method)
	}

	if err == nil {
		// successful Call
		return &JsonRpcResponse{Jsonrpc: "2.0", Id: rpcReq.Id, Result: result}
	}

	return &JsonRpcResponse{Jsonrpc: "2.0", Id: rpcReq.Id, Error: toJsonRpcError(rpcReq.Method, err)}
}

func (s *Server) CallBatch(headers map[string][]string, batch []JsonRpcRequest) []JsonRpcResponse {
	batchResp := make([]JsonRpcResponse, len(batch))

	for _, req := range batch {
		result, err := s.Call(headers, req.Method, req.Params)
		resp := JsonRpcResponse{Jsonrpc: "2.0", Id: req.Id}
		if err == nil {
			resp.Result = result
		} else {
			resp.Error = toJsonRpcError(req.Method, err)
		}
		batchResp = append(batchResp, resp)
	}

	return batchResp
}

func (s *Server) Call(headers map[string][]string, method string, params ...interface{}) (interface{}, error) {

	idlFunc, ok := s.idl.methods[method]
	if !ok {
		return nil, &JsonRpcError{Code: -32601, Message: fmt.Sprintf("Unsupported method: %s", method)}
	}

	iface, fname := parseMethod(method)

	handler, ok := s.handlers[iface]
	if !ok {
		return nil, &JsonRpcError{Code: -32601,
			Message: fmt.Sprintf("No handler registered for interface: %s", iface)}
	}

	// If handler supports cloning, create a new instance for this request
	c, ok := handler.(Cloneable)
	if ok {
		handler = c.Clone()
	}

	elem := reflect.ValueOf(handler)
	fn := elem.MethodByName(fname)
	if fn == zeroVal {
		return nil, &JsonRpcError{Code: -32601,
			Message: fmt.Sprintf("Function %s not found on handler %s", fname, iface)}
	}

	//fmt.Printf("Call method: %s  params: %v\n", method, params)

	// check params
	fnType := fn.Type()
	if fnType.NumIn() != len(params) {
		return nil, &JsonRpcError{Code: -32602,
			Message: fmt.Sprintf("Method %s expects %d params but was passed %d", method, fnType.NumIn(), len(params))}
	}

	if len(idlFunc.Params) != len(params) {
		return nil, &JsonRpcError{Code: -32602,
			Message: fmt.Sprintf("Method %s expects %d params but was passed %d", method, len(idlFunc.Params), len(params))}
	}

	rr := &RequestResponse{headers, method, params, handler, nil, nil}

	// run filters - PreInvoke
	flen := len(s.filters)
	for i := 0; i < flen; i++ {
		ok := s.filters[i].PreInvoke(rr)
		if !ok {
			return rr.Result, rr.Err
		}
	}

	// convert params
	paramVals := []reflect.Value{}
	for x, param := range params {
		desiredType := fnType.In(x)
		idlField := idlFunc.Params[x]
		path := fmt.Sprintf("param[%d]", x)
		paramConv := newConvert(s.idl, &idlField, desiredType, param, path)
		converted, err := paramConv.run()
		if err != nil {
			return nil, &JsonRpcError{Code: -32602, Message: err.Error()}
		}
		paramVals = append(paramVals, converted)
	}

	// make the call
	ret := fn.Call(paramVals)
	if len(ret) != 2 {
		msg := fmt.Sprintf("Method %s did not return 2 values. len(ret)=%d", method, len(ret))
		return nil, &JsonRpcError{Code: -32603, Message: msg}
	}

	ret0 := ret[0].Interface()
	ret1 := ret[1].Interface()

	rr.Result = ret0
	if ret1 != nil {
		e, ok := ret1.(error)
		if ok {
			rr.Err = e
		}
	}

	// run filters - PostInvoke
	for i := flen - 1; i >= 0; i-- {
		ok := s.filters[i].PostInvoke(rr)
		if !ok {
			break
		}
	}

	return rr.Result, rr.Err
}

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	buf := bytes.Buffer{}
	_, err := buf.ReadFrom(req.Body)
	if err != nil {
		panic(err)
	}
	resp := s.InvokeBytes(req.Header, buf.Bytes())
	w.Header().Set("Content-Type", s.ser.MimeType())
	fmt.Fprintf(w, string(resp))
}

func parseMethod(method string) (string, string) {
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

func jsonParseErr(reqId string, batch bool, err error) []byte {
	rpcerr := &JsonRpcError{Code: -32700, Message: fmt.Sprintf("Unable to parse JSON: %s", err.Error())}
	resp := JsonRpcResponse{Jsonrpc: "2.0"}
	resp.Id = reqId
	resp.Error = rpcerr

	if batch {
		respBatch := []JsonRpcResponse{resp}
		b, err := json.Marshal(respBatch)
		if err != nil {
			panic(err)
		}
		return b
	}

	b, err := json.Marshal(resp)
	if err != nil {
		panic(err)
	}
	return b
}
