package config_test

import (
	"mogenius-operator/src/config"
	"mogenius-operator/src/utils"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

// compile time check
func TestSlogManagerAdheresToLogManagerInterface(t *testing.T) {
	t.Parallel()
	testfunc := func(w config.ConfigModule) {}
	testfunc(config.NewConfig()) // this checks if the typesystem allows to call it
}

func TestSetUndeclaredValue(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	c := config.NewConfig()

	assert.Panics(func() { c.Set("foo", "bar") }, "cant set value of undeclared variable")
}

func TestTrySetUndeclaredValue(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	c := config.NewConfig()

	err := c.TrySet("foo", "bar")
	assert.Error(err, "cant set value of undeclared variable")
}

func TestGetUndeclaredPanics(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	c := config.NewConfig()

	assert.Panics(func() { c.Get("foo") }, "cant get value of undeclared variable")
}

func TestTryGetUndeclaredPanics(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	c := config.NewConfig()

	val, err := c.TryGet("foo")
	assert.Error(err)
	assert.Equal("", val)
}

func TestGetUninitializedPanics(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	c := config.NewConfig()

	c.Declare(config.ConfigDeclaration{
		Key: "foo",
	})
	assert.Panics(func() { c.Get("foo") }, "cant get value of undeclared variable")
}

func TestCallbackWorks(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	callbackCallCounter := 0

	cb := func(key string, value string, isSecret bool) {
		callbackCallCounter += 1
	}

	c := config.NewConfig()
	c.OnChanged([]string{}, cb)

	assert.Equal(0, callbackCallCounter)

	c.Declare(config.ConfigDeclaration{
		Key:          "foo",
		DefaultValue: utils.Pointer("bar"),
	})
	assert.Equal(1, callbackCallCounter)

	c.Declare(config.ConfigDeclaration{
		Key:          "bacon",
		DefaultValue: utils.Pointer("ipsum"),
	})
	assert.Equal(2, callbackCallCounter)
}

func TestSetAndGetMultiple(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	c := config.NewConfig()

	c.Declare(config.ConfigDeclaration{
		Key:          "foo",
		DefaultValue: utils.Pointer("bar"),
	})
	assert.Equal("bar", c.Get("foo"), "the first call to get('someKey') should work")
	assert.Equal("bar", c.Get("foo"), "a second call to get('someKey') should work")

	c.Declare(config.ConfigDeclaration{
		Key:          "bacon",
		DefaultValue: utils.Pointer("ipsum"),
	})
	assert.Equal("ipsum", c.Get("bacon"), "the first call to get('someOtherKey') should work")
	assert.Equal("ipsum", c.Get("bacon"), "a second call to get('someOtherKey') should work")

	assert.Equal("bar", c.Get("foo"), "the third call to get('someKey') should still work after another key was set")
}

func TestUsageSummaryEmpty(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	c := config.NewConfig()

	usage := c.GetUsage()
	assert.Equal(0, len(usage))
}

func TestUsageSummaryUninitialized(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	c := config.NewConfig()

	c.Declare(config.ConfigDeclaration{
		Key: "foo",
	})

	usage := c.GetUsage()

	assert.Equal(1, len(usage))
	assert.True(slices.Contains(usage, config.Usage{Key: "foo", Initialized: false, SetCalls: 0, GetCalls: 0}))
}

func TestUsageSummaryWorks(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	c := config.NewConfig()

	usage := c.GetUsage()
	assert.Equal(0, len(usage))

	c.Declare(config.ConfigDeclaration{
		Key:          "foo",
		DefaultValue: utils.Pointer("bar"),
	})
	err := c.TrySet("foo", "baz")
	assert.NoError(err)

	c.Declare(config.ConfigDeclaration{
		Key:          "bacon",
		DefaultValue: utils.Pointer("ipsum"),
	})

	assert.Equal("baz", c.Get("foo"))
	assert.Equal("ipsum", c.Get("bacon"))

	usage = c.GetUsage()

	assert.Equal(2, len(usage))
	assert.True(slices.Contains(usage, config.Usage{Key: "foo", Initialized: true, SetCalls: 2, GetCalls: 1}))
	assert.True(slices.Contains(usage, config.Usage{Key: "bacon", Initialized: true, SetCalls: 1, GetCalls: 1}))
}
