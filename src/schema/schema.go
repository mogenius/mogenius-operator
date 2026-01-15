package schema

import (
	"encoding/json"
	"fmt"
	"mogenius-operator/src/assert"
	"reflect"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
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
	SchemaTypeAny             SchemaType = "any"
)

type Schema struct {
	TypeInfo      *TypeInfo               `json:"typeInfo,omitempty"`
	StructLayouts map[string]StructLayout `json:"structs,omitempty"`
}

type TypeInfo struct {
	Type    SchemaType `json:"type"`
	Pointer bool       `json:"pointer,omitempty"`

	// Type == Array
	ElementType *TypeInfo `json:"elementType,omitempty"`

	// Type == Struct
	StructRef string `json:"structRef,omitempty"`

	// Type == Map
	KeyType   *TypeInfo `json:"keyType,omitempty"`
	ValueType *TypeInfo `json:"valueType,omitempty"`
}

func (self *TypeInfo) StructLayout(schema *Schema) (*StructLayout, error) {
	if self.Type != SchemaTypeStruct {
		return nil, fmt.Errorf("not a struct")
	}

	layout, ok := schema.StructLayouts[self.StructRef]
	if !ok {
		return nil, fmt.Errorf("layout not found")
	}

	return &layout, nil
}

type StructLayout struct {
	Name          string               `json:"name,omitempty"`
	Properties    map[string]*TypeInfo `json:"properties"`
	reflectedType reflect.Type         `json:"-"`
}

func (self *StructLayout) IsAnonymous() bool {
	return self.Name == ""
}

func (self *StructLayout) Equals(other *StructLayout) bool {
	if self.Name != other.Name {
		return false
	}

	if self.reflectedType.NumField() != other.reflectedType.NumField() {
		return false
	}

	fieldAmount := self.reflectedType.NumField()
	for n := 0; n < fieldAmount; n++ {
		selfReflectedStructField := self.reflectedType.Field(n)
		otherReflectedStructField := other.reflectedType.Field(n)
		if selfReflectedStructField.Name != otherReflectedStructField.Name {
			return false
		}
		if selfReflectedStructField.PkgPath != otherReflectedStructField.PkgPath {
			return false
		}
		if selfReflectedStructField.Type != otherReflectedStructField.Type {
			return false
		}
	}

	return true
}

func (self *Schema) Json() string {
	bytes, err := json.Marshal(self)
	assert.Assert(err == nil, err)
	return string(bytes)
}

func (self *Schema) Yaml() string {
	// this is going through json to inherit the json tag config
	jsonData := self.Json()

	var data any
	err := json.Unmarshal([]byte(jsonData), &data)
	assert.Assert(err == nil, err)

	bytes, err := yaml.Marshal(data)
	assert.Assert(err == nil, err)

	return string(bytes)
}

func Generate(input any) *Schema {
	schema := &Schema{}
	schema.StructLayouts = map[string]StructLayout{}
	typeInfo, err := parseSchema(schema, reflect.TypeOf(input), 0)
	assert.Assert(err == nil, "schema generation failed", err)
	schema.TypeInfo = typeInfo
	return schema
}

func TryGenerate(input any) (*Schema, error) {
	schema := &Schema{}
	schema.StructLayouts = map[string]StructLayout{}
	typeInfo, err := parseSchema(schema, reflect.TypeOf(input), 0)
	if err != nil {
		return nil, err
	}
	schema.TypeInfo = typeInfo
	return schema, nil
}

func parseSchema(schema *Schema, input reflect.Type, depth int) (*TypeInfo, error) {
	if depth > 4096 {
		return nil, fmt.Errorf("exceeded max recursion depth of 4096")
	}

	typeInfo := &TypeInfo{}
	typeInfo.Pointer = false

	if input == nil {
		typeInfo.Type = SchemaTypeAny
		typeInfo.Pointer = true
		return typeInfo, nil
	}

	if input.Kind() == reflect.Pointer {
		input = input.Elem()
		typeInfo.Pointer = true
	}

	switch input.Kind() {
	case reflect.Bool:
		typeInfo.Type = SchemaTypeBoolean
		return typeInfo, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		typeInfo.Type = SchemaTypeInteger
		return typeInfo, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		typeInfo.Type = SchemaTypeUnsignedInteger
		return typeInfo, nil
	case reflect.Float32, reflect.Float64:
		typeInfo.Type = SchemaTypeFloat
		return typeInfo, nil
	case reflect.String:
		typeInfo.Type = SchemaTypeString
		return typeInfo, nil
	case reflect.Func:
		typeInfo.Type = SchemaTypeFunction
		return typeInfo, nil
	case reflect.Interface:
		typeInfo.Type = SchemaTypeAny
		typeInfo.Pointer = true
		return typeInfo, nil
	case reflect.Map:
		typeInfo.Type = SchemaTypeMap
		keyType, err := parseSchema(schema, input.Key(), depth+1)
		if err != nil {
			return nil, err
		}
		typeInfo.KeyType = keyType
		valueType, err := parseSchema(schema, input.Elem(), depth+1)
		if err != nil {
			return nil, err
		}
		typeInfo.ValueType = valueType
		return typeInfo, nil
	case reflect.Slice, reflect.Array:
		typeInfo.Type = SchemaTypeArray
		elems, err := parseSchema(schema, input.Elem(), depth+1)
		if err != nil {
			return nil, err
		}
		typeInfo.ElementType = elems
		return typeInfo, nil
	case reflect.Struct:
		typeInfo.Type = SchemaTypeStruct
		structDeclaration := &StructLayout{}
		anonymous := input.PkgPath() == "" && input.Name() == ""
		if !anonymous {
			structDeclaration.Name = input.PkgPath() + "." + input.Name()
		}
		structDeclaration.Properties = map[string]*TypeInfo{}
		structDeclaration.reflectedType = input

		// early exit if type was already added to schema
		for key, existingDeclaration := range schema.StructLayouts {
			if existingDeclaration.Equals(structDeclaration) {
				typeInfo.StructRef = key
				return typeInfo, nil
			}
		}

		// create a new struct declaration and reference it
		var structId string
		if anonymous {
			structId = fmt.Sprintf("ANON_STRUCT_%d", len(schema.StructLayouts))
		} else {
			structId = structDeclaration.Name
		}
		typeInfo.StructRef = structId
		schema.StructLayouts[structId] = *structDeclaration

		fieldAmount := input.NumField()
		for n := 0; n < fieldAmount; n++ {
			inputStructField := input.Field(n)
			// skip private fields
			firstRune := rune(inputStructField.Name[0])
			if !unicode.IsLetter(firstRune) || !unicode.IsUpper(firstRune) {
				continue
			}
			fieldName := strings.Split(inputStructField.Tag.Get("json"), ",")[0]
			if fieldName == "-" { // special syntax for the json serializer to hide a field so we do the same
				continue
			}
			if fieldName == "" { // use the variables name directly as fallback if json annotation is missing
				fieldName = inputStructField.Name
			}
			assert.Assert(fieldName != "")
			fieldType, err := parseSchema(schema, inputStructField.Type, depth+1)
			if err != nil {
				return nil, err
			}
			structDeclaration.Properties[fieldName] = fieldType
		}

		return typeInfo, nil
	default:
		assert.Assert(false, "Unreachable: Unhandled Type", input.Kind().String())
		panic(1)
	}
}

func String() *Schema {
	typeInfo := &TypeInfo{}
	typeInfo.Type = SchemaTypeString
	typeInfo.Pointer = false

	schema := &Schema{}
	schema.TypeInfo = typeInfo
	schema.StructLayouts = map[string]StructLayout{}

	return schema
}

func StringPointer() *Schema {
	typeInfo := &TypeInfo{}
	typeInfo.Type = SchemaTypeString
	typeInfo.Pointer = true

	schema := &Schema{}
	schema.TypeInfo = typeInfo
	schema.StructLayouts = map[string]StructLayout{}

	return schema
}

func Integer() *Schema {
	typeInfo := &TypeInfo{}
	typeInfo.Type = SchemaTypeInteger
	typeInfo.Pointer = false

	schema := &Schema{}
	schema.TypeInfo = typeInfo
	schema.StructLayouts = map[string]StructLayout{}

	return schema
}

func IntegerPointer() *Schema {
	typeInfo := &TypeInfo{}
	typeInfo.Type = SchemaTypeInteger
	typeInfo.Pointer = true

	schema := &Schema{}
	schema.TypeInfo = typeInfo
	schema.StructLayouts = map[string]StructLayout{}

	return schema
}

func UnsignedInteger() *Schema {
	typeInfo := &TypeInfo{}
	typeInfo.Type = SchemaTypeUnsignedInteger
	typeInfo.Pointer = false

	schema := &Schema{}
	schema.TypeInfo = typeInfo
	schema.StructLayouts = map[string]StructLayout{}

	return schema
}

func UnsignedIntegerPointer() *Schema {
	typeInfo := &TypeInfo{}
	typeInfo.Type = SchemaTypeUnsignedInteger
	typeInfo.Pointer = true

	schema := &Schema{}
	schema.TypeInfo = typeInfo
	schema.StructLayouts = map[string]StructLayout{}

	return schema
}

func Float() *Schema {
	typeInfo := &TypeInfo{}
	typeInfo.Type = SchemaTypeFloat
	typeInfo.Pointer = false

	schema := &Schema{}
	schema.TypeInfo = typeInfo
	schema.StructLayouts = map[string]StructLayout{}

	return schema
}

func FloatPointer() *Schema {
	typeInfo := &TypeInfo{}
	typeInfo.Type = SchemaTypeFloat
	typeInfo.Pointer = true

	schema := &Schema{}
	schema.TypeInfo = typeInfo
	schema.StructLayouts = map[string]StructLayout{}

	return schema
}

func Boolean() *Schema {
	typeInfo := &TypeInfo{}
	typeInfo.Type = SchemaTypeBoolean
	typeInfo.Pointer = false

	schema := &Schema{}
	schema.TypeInfo = typeInfo
	schema.StructLayouts = map[string]StructLayout{}

	return schema
}

func BooleanPointer() *Schema {
	typeInfo := &TypeInfo{}
	typeInfo.Type = SchemaTypeBoolean
	typeInfo.Pointer = true

	schema := &Schema{}
	schema.TypeInfo = typeInfo
	schema.StructLayouts = map[string]StructLayout{}

	return schema
}

func Interface() *Schema {
	typeInfo := &TypeInfo{}
	typeInfo.Type = SchemaTypeAny
	typeInfo.Pointer = true

	schema := &Schema{}
	schema.TypeInfo = typeInfo
	schema.StructLayouts = map[string]StructLayout{}

	return schema
}

func Any() *Schema {
	typeInfo := &TypeInfo{}
	typeInfo.Type = SchemaTypeAny
	typeInfo.Pointer = true

	schema := &Schema{}
	schema.TypeInfo = typeInfo
	schema.StructLayouts = map[string]StructLayout{}

	return schema
}

func Error() *Schema {
	typeInfo := &TypeInfo{}
	typeInfo.Type = SchemaTypeAny
	typeInfo.Pointer = true

	schema := &Schema{}
	schema.TypeInfo = typeInfo
	schema.StructLayouts = map[string]StructLayout{}

	return schema
}
