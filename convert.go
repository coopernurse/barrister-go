package barrister

import (
	"fmt"
	"reflect"
	"strings"
)

type TypeError struct {
	// string that describes location of value in the
	// param or return value graph.  e.g. param[0].Addresses[0].Street1
	path string

	msg string
}

func (e *TypeError) Error() string {
	return fmt.Sprintf("barrister: type error: %s: %s", e.path, e.msg)
}

type Convert struct {
	idl       *Idl
	field     *Field
	desired   reflect.Type
	desirePtr bool
	actual    interface{}
	converted interface{}
	path      string
}

func NewConvert(idl *Idl, field *Field, desired reflect.Type, actual interface{}, path string) *Convert {
	return &Convert{idl, field, desired, false, actual, actual, path}
}

func (c *Convert) Run() (reflect.Value, error) {
	kind := c.desired.Kind()

	if c.actual == nil {
		if kind == reflect.Ptr {
			if c.field.Optional {
				return reflect.ValueOf(nil), nil
			} else {
				return zeroVal, &TypeError{c.path, "null not allowed"}
			}
		} else {
			return zeroVal, &TypeError{c.path, "Unable to convert nil to non-pointer"}
		}
	}

	actType := reflect.TypeOf(c.actual)

	goal := fmt.Sprintf("convert: %v - %v to %v", c.path,
		actType.Kind().String(), c.desired)

	//fmt.Printf("%s\n", goal)

	if actType == c.desired {
		return reflect.ValueOf(c.actual), nil
	}

	if kind == reflect.Ptr {
		c.desirePtr = true
		c.desired = c.desired.Elem()
		kind = c.desired.Kind()
	}

	if actType.Kind() == kind {
		//fmt.Printf("%v is assignable to %v\n", actType, desired)
		v := reflect.New(c.desired).Elem()
		switch kind {
		case reflect.String:
			s, ok := c.actual.(string)
			if ok {
				v.SetString(s)

				if c.field.Type != "string" {
					enum, ok := c.idl.Enums[c.field.Type]
					if ok {
						for _, enumVal := range enum {
							if enumVal.Value == s {
								return checkPointer(v, c.desirePtr)
							}
						}

						msg := fmt.Sprintf("Value %s not in enum values: %v", s, enum)
						return zeroVal, &TypeError{path: c.path, msg: msg}
					}
				}

				return c.returnVal("string")
			}
		}
	} else {
		//fmt.Printf("%v is NOT assignable to %v\n", actType, desired)
	}

	switch kind {
	case reflect.String:
		_, ok := c.actual.(string)
		if ok {
			return c.returnVal("string")
		}
	case reflect.Int:
		s, ok := c.actual.(int)
		if ok {
			c.converted = int64(s)
			return c.returnVal("int")
		}
		_, ok = c.actual.(int64)
		if ok {
			return c.returnVal("int")
		}
	case reflect.Int64:
		s, ok := c.actual.(int64)
		if ok {
			c.converted = s
			return c.returnVal("int")
		}
		s2, ok := c.actual.(int)
		if ok {
			c.converted = int64(s2)
			return c.returnVal("int")
		}
		s3, ok := c.actual.(float64)
		if ok {
			s4 := int64(s3)
			if float64(s4) == s3 {
				c.converted = s4
				return c.returnVal("int")
			}
		}
	case reflect.Float32:
		s, ok := c.actual.(float32)
		if ok {
			c.converted = float64(s)
			return c.returnVal("float")
		}
	case reflect.Float64:
		s, ok := c.actual.(float64)
		if ok {
			c.converted = s
			return c.returnVal("float")
		}
		s2, ok := c.actual.(float32)
		if ok {
			c.converted = float64(s2)
			return c.returnVal("float")
		}
	case reflect.Bool:
		_, ok := c.actual.(bool)
		if ok {
			return c.returnVal("bool")
		}
	case reflect.Slice:
		actVal := reflect.ValueOf(c.actual)
		actType := actVal.Type()
		if actType.Kind() == reflect.Slice {
			return c.convertSlice()
		}
	case reflect.Struct:
		m, ok := c.actual.(map[string]interface{})
		if ok {
			return c.convertStruct(m)
		}
	}

	return zeroVal, &TypeError{c.path, "Unable to " + goal}
}

func (c *Convert) convertSlice() (reflect.Value, error) {
	actVal := reflect.ValueOf(c.actual)
	sliceType := c.desired.Elem()
	sliceV := reflect.New(c.desired)
	slice := sliceV.Elem()

	elemField := &Field{Name: c.field.Name, Type: c.field.Type,
		Optional: c.field.Optional, IsArray: false}

	for x := 0; x < actVal.Len(); x++ {
		el := actVal.Index(x)
		elemPath := fmt.Sprintf("%s[%d]", c.path, x)
		elemConv := NewConvert(c.idl, elemField, sliceType,
			el.Interface(), elemPath)
		conv, err := elemConv.Run()
		if err != nil {
			return zeroVal, err
		}
		slice = reflect.Append(slice, conv)
	}
	return slice, nil
}

func (c *Convert) convertStruct(m map[string]interface{}) (reflect.Value, error) {

	idlStruct, ok := c.idl.Structs[c.field.Type]

	if !ok {
		msg := fmt.Sprintf("Struct not found in IDL: %s", c.field.Type)
		return zeroVal, &TypeError{path: c.path, msg: msg}
	}

	val := reflect.New(c.desired)

	for fname, sField := range idlStruct.computed {
		goName := fname
		structField, ok := c.desired.FieldByName(fname)
		if !ok {
			goName = capitalize(fname)
			structField, ok = c.desired.FieldByName(goName)
			if !ok {
				msg := fmt.Sprintf("Struct: %v is missing required field: %s",
					c.desired, fname)
				return zeroVal, &TypeError{path: c.path, msg: msg}
			}
		}

		mval, ok := m[fname]

		if !ok && !sField.Optional {
			msg := fmt.Sprintf("Struct value: %s is missing required field: %s",
				c.field.Type, fname)
			return zeroVal, &TypeError{path: c.path, msg: msg}
		}

		if ok {

			fieldConv := NewConvert(c.idl, &sField, structField.Type, mval,
				c.path+"."+fname)
			conv, err := fieldConv.Run()
			if err != nil {
				return zeroVal, err
			}

			f := val.Elem().FieldByName(goName)
			if f == zeroVal {
				msg := fmt.Sprintf("Instance: %s is missing required field: %s",
					c.field.Type, goName)
				return zeroVal, &TypeError{path: c.path, msg: msg}
			}

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
	return checkPointer(val.Elem(), c.desirePtr)
}

func (c *Convert) returnVal(convertedType string) (reflect.Value, error) {
	if c.field.Type != convertedType {
		msg := fmt.Sprintf("Type mismatch for '%s' - Expected: %s Got: %v",
			c.path, c.field.Type, convertedType)
		return zeroVal, &TypeError{path: c.path, msg: msg}
	}

	return checkPointer(reflect.ValueOf(c.converted), c.desirePtr)
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

func checkPointer(val reflect.Value, desirePtr bool) (reflect.Value, error) {
	if desirePtr {
		return val.Addr(), nil
	}
	return val, nil
}
