package schema_test

import (
	"mogenius-operator/src/assert"
	"mogenius-operator/src/schema"
	"testing"
)

func TestString(t *testing.T) {
	s, err := schema.TryGenerate("foo")
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.TypeInfo.Type == schema.SchemaTypeString)
	assert.AssertT(t, !s.TypeInfo.Pointer)
}

func TestStringPointer(t *testing.T) {
	data := "foo"
	s, err := schema.TryGenerate(&data)
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.TypeInfo.Type == schema.SchemaTypeString)
	assert.AssertT(t, s.TypeInfo.Pointer)
}

func TestFunctionPointer(t *testing.T) {
	s, err := schema.TryGenerate(func() {})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.TypeInfo.Type == schema.SchemaTypeFunction)
}

func TestBoolean(t *testing.T) {
	s, err := schema.TryGenerate(false)
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.TypeInfo.Type == schema.SchemaTypeBoolean)
}

func TestInteger(t *testing.T) {
	var n int = 0
	s, err := schema.TryGenerate(n)
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.TypeInfo.Type == schema.SchemaTypeInteger)
}

func TestUnsignedInteger(t *testing.T) {
	var n uint = 0
	s, err := schema.TryGenerate(n)
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.TypeInfo.Type == schema.SchemaTypeUnsignedInteger)
}

func TestFloat(t *testing.T) {
	s, err := schema.TryGenerate(0.0)
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.TypeInfo.Type == schema.SchemaTypeFloat)
}

func TestSliceInteger(t *testing.T) {
	s, err := schema.TryGenerate([]int{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.TypeInfo.Type == schema.SchemaTypeArray)
	assert.AssertT(t, s.TypeInfo.ElementType.Type == schema.SchemaTypeInteger)
}

func TestSliceUnsignedInteger(t *testing.T) {
	s, err := schema.TryGenerate([]uint{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.TypeInfo.Type == schema.SchemaTypeArray)
	assert.AssertT(t, s.TypeInfo.ElementType.Type == schema.SchemaTypeUnsignedInteger)
}

func TestSliceFloat(t *testing.T) {
	s, err := schema.TryGenerate([]float64{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.TypeInfo.Type == schema.SchemaTypeArray)
	assert.AssertT(t, s.TypeInfo.ElementType.Type == schema.SchemaTypeFloat)
}

func TestSliceString(t *testing.T) {
	s, err := schema.TryGenerate([]string{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.TypeInfo.Type == schema.SchemaTypeArray)
	assert.AssertT(t, s.TypeInfo.ElementType.Type == schema.SchemaTypeString)
}

func TestArrayInteger(t *testing.T) {
	s, err := schema.TryGenerate([4]int{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.TypeInfo.Type == schema.SchemaTypeArray)
	assert.AssertT(t, s.TypeInfo.ElementType.Type == schema.SchemaTypeInteger)
}

func TestArrayUnsignedInteger(t *testing.T) {
	s, err := schema.TryGenerate([4]uint{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.TypeInfo.Type == schema.SchemaTypeArray)
	assert.AssertT(t, s.TypeInfo.ElementType.Type == schema.SchemaTypeUnsignedInteger)
}

func TestArrayFloat(t *testing.T) {
	s, err := schema.TryGenerate([4]float64{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.TypeInfo.Type == schema.SchemaTypeArray)
	assert.AssertT(t, s.TypeInfo.ElementType.Type == schema.SchemaTypeFloat)
}

func TestArrayString(t *testing.T) {
	s, err := schema.TryGenerate([4]string{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.TypeInfo.Type == schema.SchemaTypeArray)
	assert.AssertT(t, s.TypeInfo.ElementType.Type == schema.SchemaTypeString)
}

func TestMapStringString(t *testing.T) {
	s, err := schema.TryGenerate(map[string]string{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.TypeInfo.Type == schema.SchemaTypeMap)
	assert.AssertT(t, s.TypeInfo.KeyType.Type == schema.SchemaTypeString)
	assert.AssertT(t, s.TypeInfo.ValueType.Type == schema.SchemaTypeString)
}

func TestMapStringFloat(t *testing.T) {
	s, err := schema.TryGenerate(map[string]float64{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.TypeInfo.Type == schema.SchemaTypeMap)
	assert.AssertT(t, !s.TypeInfo.Pointer)
	assert.AssertT(t, s.TypeInfo.KeyType.Type == schema.SchemaTypeString)
	assert.AssertT(t, s.TypeInfo.ValueType.Type == schema.SchemaTypeFloat)
}

func TestStruct(t *testing.T) {
	type Data struct{}
	s, err := schema.TryGenerate(Data{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.TypeInfo.Type == schema.SchemaTypeStruct)
	assert.AssertT(t, !s.TypeInfo.Pointer)
}

func TestStructPointer(t *testing.T) {
	type Data struct{}
	s, err := schema.TryGenerate(&Data{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.TypeInfo.Type == schema.SchemaTypeStruct)
	assert.AssertT(t, s.TypeInfo.Pointer)
}

func TestRecursiveStruct(t *testing.T) {
	type ExampleStruct struct {
		RecursionTrap    *ExampleStruct `json:"recursionTrap,omitempty"`
		UnnamedStructure struct {
			RecursionTrap *ExampleStruct `json:"recursionTrap,omitempty"`
		}
	}
	s, err := schema.TryGenerate(ExampleStruct{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.TypeInfo.Type == schema.SchemaTypeStruct)
	assert.AssertT(t, len(s.StructLayouts) == 2, len(s.StructLayouts))
}

func TestLookupStruct(t *testing.T) {
	type ExampleStruct struct {
		RecursionTrap string `json:"recursionTrap,omitempty"`
	}

	s, err := schema.TryGenerate(ExampleStruct{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	layout, err := s.TypeInfo.StructLayout(s)
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, layout != nil, "layout should exist")
	assert.AssertT(t, layout.Name != "", "layout name should be set")
	assert.AssertT(t, !layout.IsAnonymous(), "layout should know it is not anonymous")
	assert.AssertT(t, len(layout.Properties) == 1, "a single property should be found")
}

func TestLookupAnonStruct(t *testing.T) {
	s, err := schema.TryGenerate(struct {
		RecursionTrap string `json:"recursionTrap,omitempty"`
	}{})
	t.Logf("%s", s.Json())
	t.Logf("\n%s", s.Yaml())
	assert.AssertT(t, err == nil, err)
	layout, err := s.TypeInfo.StructLayout(s)
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, layout != nil, "layout should exist")
	assert.AssertT(t, layout.Name == "", "layout name should not be set")
	assert.AssertT(t, layout.IsAnonymous(), "layout should know it is anonymous")
	assert.AssertT(t, len(layout.Properties) == 1, "a single property should be found")
}
