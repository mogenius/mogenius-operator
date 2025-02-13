package schema_test

import (
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/schema"
	"testing"
)

func TestString(t *testing.T) {
	s, err := schema.TryGenerate("foo")
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.Type == schema.SchemaTypeString)
	assert.AssertT(t, !s.Pointer)
}

func TestStringPointer(t *testing.T) {
	data := "foo"
	s, err := schema.TryGenerate(&data)
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.Type == schema.SchemaTypeString)
	assert.AssertT(t, s.Pointer)
}

func TestFunctionPointer(t *testing.T) {
	s, err := schema.TryGenerate(func() {})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.Type == schema.SchemaTypeFunction)
}

func TestBoolean(t *testing.T) {
	s, err := schema.TryGenerate(false)
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.Type == schema.SchemaTypeBoolean)
}

func TestInteger(t *testing.T) {
	var n int = 0
	s, err := schema.TryGenerate(n)
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.Type == schema.SchemaTypeInteger)
}

func TestUnsignedInteger(t *testing.T) {
	var n uint = 0
	s, err := schema.TryGenerate(n)
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.Type == schema.SchemaTypeUnsignedInteger)
}

func TestFloat(t *testing.T) {
	s, err := schema.TryGenerate(0.0)
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.Type == schema.SchemaTypeFloat)
}

func TestSliceInteger(t *testing.T) {
	s, err := schema.TryGenerate([]int{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.Type == schema.SchemaTypeArray)
	assert.AssertT(t, s.ElementType.Type == schema.SchemaTypeInteger)
}

func TestSliceUnsignedInteger(t *testing.T) {
	s, err := schema.TryGenerate([]uint{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.Type == schema.SchemaTypeArray)
	assert.AssertT(t, s.ElementType.Type == schema.SchemaTypeUnsignedInteger)
}

func TestSliceFloat(t *testing.T) {
	s, err := schema.TryGenerate([]float64{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.Type == schema.SchemaTypeArray)
	assert.AssertT(t, s.ElementType.Type == schema.SchemaTypeFloat)
}

func TestSliceString(t *testing.T) {
	s, err := schema.TryGenerate([]string{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.Type == schema.SchemaTypeArray)
	assert.AssertT(t, s.ElementType.Type == schema.SchemaTypeString)
}

func TestArrayInteger(t *testing.T) {
	s, err := schema.TryGenerate([4]int{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.Type == schema.SchemaTypeArray)
	assert.AssertT(t, s.ElementType.Type == schema.SchemaTypeInteger)
}

func TestArrayUnsignedInteger(t *testing.T) {
	s, err := schema.TryGenerate([4]uint{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.Type == schema.SchemaTypeArray)
	assert.AssertT(t, s.ElementType.Type == schema.SchemaTypeUnsignedInteger)
}

func TestArrayFloat(t *testing.T) {
	s, err := schema.TryGenerate([4]float64{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.Type == schema.SchemaTypeArray)
	assert.AssertT(t, s.ElementType.Type == schema.SchemaTypeFloat)
}

func TestArrayString(t *testing.T) {
	s, err := schema.TryGenerate([4]string{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.Type == schema.SchemaTypeArray)
	assert.AssertT(t, s.ElementType.Type == schema.SchemaTypeString)
}

func TestMapStringString(t *testing.T) {
	s, err := schema.TryGenerate(map[string]string{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.Type == schema.SchemaTypeMap)
	assert.AssertT(t, s.KeyType.Type == schema.SchemaTypeString)
	assert.AssertT(t, s.ValueType.Type == schema.SchemaTypeString)
}

func TestMapStringFloat(t *testing.T) {
	s, err := schema.TryGenerate(map[string]float64{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.Type == schema.SchemaTypeMap)
	assert.AssertT(t, !s.Pointer)
	assert.AssertT(t, s.KeyType.Type == schema.SchemaTypeString)
	assert.AssertT(t, s.ValueType.Type == schema.SchemaTypeFloat)
}

func TestStruct(t *testing.T) {
	type Data struct{}
	s, err := schema.TryGenerate(Data{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.Type == schema.SchemaTypeStruct)
	assert.AssertT(t, !s.Pointer)
}

func TestStructPointer(t *testing.T) {
	type Data struct{}
	s, err := schema.TryGenerate(&Data{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.Type == schema.SchemaTypeStruct)
	assert.AssertT(t, s.Pointer)
}

func TestCustomStruct(t *testing.T) {
	type ExampleStructTwo struct {
		RecursionTrap *schema.Schema `json:"recursionTrap,omitempty"`
	}
	type ExampleStruct struct {
		ExampleEmbeddedStruct struct {
			FieldOne string
			FieldTwo string `json:"fieldTwo,omitempty"`
		} `json:"exampleEmbeddedStruct,omitempty"`
		EmptyDataStruct struct {
		} `json:"emptyDataStruct,omitempty"`
		DataMap map[string]ExampleStructTwo `json:"dataMap,omitempty"`
	}
	s, err := schema.TryGenerate(ExampleStruct{})
	t.Logf("%s", s.Json())
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, s.Type == schema.SchemaTypeStruct)
	sEmbedded, ok := s.Properties["exampleEmbeddedStruct"]
	assert.AssertT(t, ok)
	assert.AssertT(t, sEmbedded.Type == schema.SchemaTypeStruct)
	_, ok = sEmbedded.Properties["FieldOne"]
	assert.AssertT(t, ok, `"FieldOne" does not have an associated json tag to rename it`)
	_, ok = sEmbedded.Properties["fieldTwo"]
	assert.AssertT(t, ok, `"FieldTwo" has an associated json tag to rename it into "fieldTwo"`)
	emptyDataStruct, ok := s.Properties["emptyDataStruct"]
	assert.AssertT(t, ok)
	assert.AssertT(t, emptyDataStruct.Type == schema.SchemaTypeStruct)
	assert.AssertT(t, len(emptyDataStruct.Properties) == 0)
}
