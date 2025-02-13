package schema

import (
	"fmt"
	"mogenius-k8s-manager/src/assert"
	"reflect"
	"strings"

	jsoniter "github.com/json-iterator/go"
)

type SchemaType string

const (
	SchemaTypeBoolean         SchemaType = "bool"
	SchemaTypeInteger         SchemaType = "int"
	SchemaTypeUnsignedInteger SchemaType = "uint"
	SchemaTypeFloat           SchemaType = "float"
	SchemaTypeString          SchemaType = "string"
	SchemaTypeStruct          SchemaType = "struct"
	SchemaTypeArray           SchemaType = "array"
	SchemaTypeMap             SchemaType = "map"
	SchemaTypeFunction        SchemaType = "function"
	SchemaTypeInterface       SchemaType = "interface"
	SchemaTypeSchema          SchemaType = "schema" // prevent recursion
)

type Schema struct {
	Type    SchemaType `json:"type"`
	Pointer bool       `json:"pointer,omitempty"`
	// Type == Array
	ElementType *Schema `json:"elementType,omitempty"`
	// Type == Struct
	Properties map[string]*Schema `json:"properties,omitempty"`
	// Type == Map
	KeyType   *Schema `json:"keyType,omitempty"`
	ValueType *Schema `json:"valueType,omitempty"`
}

var typeOfSchema = reflect.TypeOf(Schema{})

func (self *Schema) Json() string {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	bytes, err := json.Marshal(self)
	assert.Assert(err == nil, err)
	return string(bytes)
}

func Generate(input any) *Schema {
	s, err := parseSchema(reflect.TypeOf(input), 0)
	assert.Assert(err == nil, "schema generation failed", err)
	return s
}

func TryGenerate(input any) (*Schema, error) {
	s, err := parseSchema(reflect.TypeOf(input), 0)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func parseSchema(input reflect.Type, depth int) (*Schema, error) {
	if depth > 4096 {
		return nil, fmt.Errorf("exceeded max recursion depth of 4096")
	}

	s := &Schema{}
	s.Pointer = false

	if input.Kind() == reflect.Ptr {
		input = input.Elem()
		s.Pointer = true
	}

	switch input.Kind() {
	case reflect.Bool:
		s.Type = SchemaTypeBoolean
		return s, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		s.Type = SchemaTypeInteger
		return s, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		s.Type = SchemaTypeUnsignedInteger
		return s, nil
	case reflect.Float32, reflect.Float64:
		s.Type = SchemaTypeFloat
		return s, nil
	case reflect.String:
		s.Type = SchemaTypeString
		return s, nil
	case reflect.Func:
		s.Type = SchemaTypeFunction
		return s, nil
	case reflect.Interface:
		s.Type = SchemaTypeInterface
		s.Pointer = true
		return s, nil
	case reflect.Struct:
		if typeOfSchema == input {
			s.Type = SchemaTypeSchema
			return s, nil
		}
		s.Type = SchemaTypeStruct
		s.Properties = map[string]*Schema{}
		fieldAmount := input.NumField()
		for n := 0; n < fieldAmount; n++ {
			f := input.Field(n)
			fieldName := strings.Split(f.Tag.Get("json"), ",")[0]
			if fieldName == "" {
				fieldName = f.Name
			}
			assert.Assert(fieldName != "")
			fieldType, err := parseSchema(f.Type, depth+1)
			if err != nil {
				return nil, err
			}
			s.Properties[fieldName] = fieldType
		}
		return s, nil
	case reflect.Map:
		s.Type = SchemaTypeMap
		keyType, err := parseSchema(input.Key(), depth+1)
		if err != nil {
			return nil, err
		}
		s.KeyType = keyType
		valueType, err := parseSchema(input.Elem(), depth+1)
		if err != nil {
			return nil, err
		}
		s.ValueType = valueType
		return s, nil
	case reflect.Slice, reflect.Array:
		s.Type = SchemaTypeArray
		elems, err := parseSchema(input.Elem(), depth+1)
		if err != nil {
			return nil, err
		}
		s.ElementType = elems
		return s, nil
	default:
		assert.Assert(false, "Unreachable: Unhandled Type", input.Kind().String())
		panic(1)
	}
}

func NewSchema(stype SchemaType, pointer bool) *Schema {
	return &Schema{
		Type:    SchemaTypeBoolean,
		Pointer: pointer,
	}
}

func String() *Schema {
	return NewSchema(SchemaTypeString, false)
}

func StringPointer() *Schema {
	return NewSchema(SchemaTypeString, true)
}

func Integer() *Schema {
	return NewSchema(SchemaTypeInteger, false)
}

func IntegerPointer() *Schema {
	return NewSchema(SchemaTypeInteger, true)
}

func UnsignedInteger() *Schema {
	return NewSchema(SchemaTypeUnsignedInteger, false)
}

func UnsignedIntegerPointer() *Schema {
	return NewSchema(SchemaTypeUnsignedInteger, true)
}

func Float() *Schema {
	return NewSchema(SchemaTypeFloat, false)
}

func FloatPointer() *Schema {
	return NewSchema(SchemaTypeFloat, true)
}

func Boolean() *Schema {
	return NewSchema(SchemaTypeBoolean, false)
}

func BooleanPointer() *Schema {
	return NewSchema(SchemaTypeBoolean, true)
}

func Interface() *Schema {
	return NewSchema(SchemaTypeInterface, true)
}

func Any() *Schema {
	return NewSchema(SchemaTypeInterface, true)
}

func Error() *Schema {
	return NewSchema(SchemaTypeInterface, true)
}
