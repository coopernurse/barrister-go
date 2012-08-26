package barrister

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

var reservedWords []string = []string{
	"break", "default", "func", "interface", "select",
    "case", "defer", "go", "map", "struct",
    "chan", "else", "goto", "package", "switch",
    "const", "fallthrough", "if", "range", "type",
	"continue", "for", "import", "return", "var",
}

func escReserved(s string) string {
	for _, word := range reservedWords {
		if word == s {
			return "_" + s
		}
	}
	return s
}

type generateGo struct {
	idl           *Idl
	pkgName       string
	optionalToPtr bool
}

func (g *generateGo) generate() []byte {
	b := &bytes.Buffer{}
	line(b, 0, fmt.Sprintf("package %s\n", g.pkgName))
	line(b, 0, "import (")
	line(b, 1, `"fmt"`)
	line(b, 1, `"reflect"`)
	line(b, 1, `"github.com/coopernurse/barrister-go"`)
	line(b, 0, ")\n")

	for name, _ := range g.idl.enums {
		g.generateEnum(b, name)
	}

	for _, elem := range g.idl.elems {
		if elem.Type == "struct" {
			s, ok := g.idl.structs[elem.Name]
			if !ok {
				panic("No struct found: " + elem.Name)
			}
			g.generateStruct(b, s)
		}
	}
	line(b, 0, "")

	for _, name := range sortedKeys(g.idl.interfaces) {
		g.generateInterface(b, name)
		line(b, 0, "}\n")
		g.generateProxy(b, name)
	}

	g.generateNewServer(b)

	return b.Bytes()
}

func (g *generateGo) generateEnum(b *bytes.Buffer, enumName string) {
	vals, ok := g.idl.enums[enumName]
	if !ok {
		panic("No enum found: " + enumName)
	}

	goName := capitalize(enumName)
	line(b, 0, fmt.Sprintf("type %s string", goName))
	line(b, 0, "const (")
	for x, val := range vals {
		typeStr := ""
		if x == 0 {
			typeStr = goName
		}
		line(b, 1, fmt.Sprintf("%s%s %s = \"%s\"",
			goName, capitalize(val.Value), typeStr, val.Value))
	}
	line(b, 0, ")\n")
}

func (g *generateGo) generateStruct(b *bytes.Buffer, s *Struct) {
	goName := capitalize(s.Name)
	line(b, 0, fmt.Sprintf("type %s struct {", goName))
	for _, f := range s.allFields {
		goName = capitalize(f.Name)
		omit := ""
		if f.Optional {
			omit = ",omitempty"
		}
		line(b, 1, fmt.Sprintf("%s\t%s\t`json:\"%s%s\"`",
			goName, f.goType(g.optionalToPtr), f.Name, omit))
	}
	line(b, 0, "}\n")
}

func (g *generateGo) generateNewServer(b *bytes.Buffer) {
	ifaceKeys := sortedKeys(g.idl.interfaces)
	ifaces := ""
	ifaceIdents := ""
	for _, name := range ifaceKeys {
		upper := capitalize(name)
		lower := escReserved(strings.ToLower(name))
		ifaces = fmt.Sprintf("%s, %s %s", ifaces, lower, upper)
		ifaceIdents += ", " + lower
	}

	line(b, 0, fmt.Sprintf("func NewJSONServer(idl *barrister.Idl, forceASCII bool%s) barrister.Server {", ifaces))
	line(b, 1, fmt.Sprintf("return NewServer(idl, &barrister.JsonSerializer{forceASCII}%s)", ifaceIdents))
	line(b, 0, "}\n")

	line(b, 0, fmt.Sprintf("func NewServer(idl *barrister.Idl, ser barrister.Serializer%s) barrister.Server {", ifaces))
	line(b, 1, fmt.Sprintf("_svr := barrister.NewServer(idl, ser)"))
	for _, name := range ifaceKeys {
		lower := strings.ToLower(name)
		line(b, 1, fmt.Sprintf("_svr.AddHandler(\"%s\", %s)", name, lower))
	}
	line(b, 1, "return _svr")
	line(b, 0, "}")
}

func (g *generateGo) generateInterface(b *bytes.Buffer, ifaceName string) {
	funcs, ok := g.idl.interfaces[ifaceName]
	if !ok {
		panic("No interface found: " + ifaceName)
	}

	goName := capitalize(ifaceName)
	line(b, 0, fmt.Sprintf("type %s interface {", goName))
	for _, fn := range funcs {
		goName = capitalize(fn.Name)
		params := ""
		for x, p := range fn.Params {
			if x > 0 {
				params += ", "
			}
			params += fmt.Sprintf("%s %s", escReserved(p.Name), p.goType(g.optionalToPtr))
		}
		line(b, 1, fmt.Sprintf("%s(%s) (%s, *barrister.JsonRpcError)",
			goName, params, fn.Returns.goType(g.optionalToPtr)))
	}
}

func (g *generateGo) generateProxy(b *bytes.Buffer, ifaceName string) {
	funcs, ok := g.idl.interfaces[ifaceName]
	if !ok {
		panic("No interface found: " + ifaceName)
	}

	goName := capitalize(ifaceName) + "Proxy"
	line(b, 0, fmt.Sprintf("type %s struct {", goName))
	line(b, 1, "client barrister.Client")
	line(b, 0, "}\n")
	for _, fn := range funcs {
		method := fmt.Sprintf("%s.%s", ifaceName, fn.Name)
		retType := fn.Returns.goType(g.optionalToPtr)
		zeroVal := fn.Returns.zeroVal(g.idl, g.optionalToPtr)
		fnName := capitalize(fn.Name)
		params := ""
		paramIdents := ""
		for x, p := range fn.Params {
			if x > 0 {
				params += ", "
			}
			ident := escReserved(p.Name)
			params += fmt.Sprintf("%s %s", ident, p.goType(g.optionalToPtr))
			paramIdents += ", "
			paramIdents += ident
		}
		line(b, 0, fmt.Sprintf("func (_p %s) %s(%s) (%s, *barrister.JsonRpcError) {",
			goName, fnName, params, retType))
		line(b, 1, fmt.Sprintf("_res, _err := _p.client.Call(\"%s\"%s)",
			method, paramIdents))
		line(b, 1, "if _err == nil {")
		if g.optionalToPtr && fn.Returns.Optional {
			line(b, 2, "if _res == nil {")
			line(b, 3, "return nil, nil")
			line(b, 2, "}")
		}
		line(b, 2, fmt.Sprintf("_cast, _ok := _res.(%s)", retType))
		line(b, 2, "if !_ok {")
		line(b, 3, "_t := reflect.TypeOf(_res)")
		line(b, 3, `_msg := fmt.Sprintf("`+method+` returned invalid type: %v", _t)`)
		line(b, 3, fmt.Sprintf("return %s, &barrister.JsonRpcError{Code: -32000, Message: _msg}", zeroVal))
		line(b, 2, "}")
		line(b, 2, "return _cast, nil")
		line(b, 1, "}")
		line(b, 1, fmt.Sprintf("return %s, _err", zeroVal))
		line(b, 0, "}\n")
	}
}

func comment(b *bytes.Buffer, level int, comment string) {
	if comment != "" {
		for _, ln := range strings.Split(comment, "\n") {
			line(b, level, fmt.Sprintf("// %s", ln))
		}
	}
}

func line(b *bytes.Buffer, level int, s string) {
	for i := 0; i < level; i++ {
		b.WriteString("\t")
	}
	b.WriteString(s)
	b.WriteString("\n")
}

func sortedKeys(m map[string][]Function) []string {
	mk := make([]string, len(m))
	i := 0
	for k, _ := range m {
		mk[i] = k
		i++
	}
	sort.Strings(mk)
	return mk
}
