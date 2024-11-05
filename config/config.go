package config

import (
	"fmt"
	"mogenius-k8s-manager/assert"
	"mogenius-k8s-manager/interfaces"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/spf13/cobra"
)

type Config struct {
	data     map[string]*configValue
	dataLock sync.RWMutex
	cbs      []func(key string, value string)
	cbsLock  sync.RWMutex
	cobraCmd *cobra.Command
}

type configValue struct {
	value       *string
	declaration interfaces.ConfigDeclaration
	getCounter  atomic.Uint64
	setCounter  atomic.Uint64
}

func NewConfig() *Config {
	return &Config{
		data:     make(map[string]*configValue),
		dataLock: sync.RWMutex{},
		cbs:      []func(key string, value string){},
		cbsLock:  sync.RWMutex{},
	}
}

func (c *Config) WithCobraCmd(cmd *cobra.Command) {
	if cmd == nil {
		return
	}
	c.cobraCmd = cmd
	cobra.OnInitialize(c.loadCobraArgs)
}

func (c *Config) loadCobraArgs() {
	c.dataLock.Lock()
	defer c.dataLock.Unlock()

	if c.cobraCmd == nil {
		return
	}
	for _, cv := range c.data {
		if cv.declaration.Cobra == nil {
			continue
		}
		if cv.declaration.Cobra.CobraValue == nil {
			continue
		}
		if !c.cobraCmd.Flags().Changed(cv.declaration.Cobra.Name) {
			continue
		}
		if cv.declaration.Validate != nil {
			err := cv.declaration.Validate(*cv.declaration.Cobra.CobraValue)
			assert.Assert(err == nil, fmt.Errorf("Validation failed for '%s' while parsing cli argument '%s' -> %s", cv.declaration.Key, "--"+cv.declaration.Cobra.Name, err))
		}
		cv.value = cv.declaration.Cobra.CobraValue
	}
}

func (c *Config) Declare(opts interfaces.ConfigDeclaration) {
	func() {
		c.dataLock.Lock()
		defer c.dataLock.Unlock()

		cv := configValue{
			value:       nil,
			declaration: opts,
			getCounter:  atomic.Uint64{},
			setCounter:  atomic.Uint64{},
		}

		assert.Assert(opts.Key != "", fmt.Errorf("'Key' in 'interfaces.ConfigDeclaration' cant be '\"\"': %#v", opts))
		assert.Assert(!strings.Contains(opts.Key, "\n"), fmt.Errorf("'Key' in 'interfaces.ConfigDeclaration' may not contain newlines: %#v", opts))
		key := opts.Key
		_, ok := c.data[key]
		assert.Assert(!ok, fmt.Errorf("a declaration with key '%s' already exists", key))

		if opts.Description != nil {
			assert.Assert(!strings.Contains(*opts.Description, "\n"), fmt.Errorf("'Description' in 'interfaces.ConfigDeclaration' may not contain newlines: %#v", opts))
		}

		if opts.Envs != nil {
			for _, env := range opts.Envs {
				assert.Assert(!strings.Contains(env, "\n"), fmt.Errorf("'Envs' in 'interfaces.ConfigDeclaration' may not contain newlines: %#v", opts))
			}
		}

		if opts.Cobra != nil {
			assert.Assert(!strings.Contains(opts.Cobra.Name, "\n"), fmt.Errorf("'Cobra.Name' in 'interfaces.ConfigDeclaration' may not contain newlines: %#v", opts))
			if opts.Cobra.Short != nil {
				assert.Assert(!strings.Contains(*opts.Cobra.Short, "\n"), fmt.Errorf("'Cobra.Short' in 'interfaces.ConfigDeclaration' may not contain newlines: %#v", opts))
			}
		}

		if opts.DefaultValue != nil {
			cv.value = opts.DefaultValue
			cv.setCounter.Add(1)
		}

		c.data[key] = &cv
	}()
	if opts.DefaultValue != nil {
		c.runCallbacks(opts.Key, *opts.DefaultValue)
	}
}

func (c *Config) Get(key string) string {
	value, err := c.TryGet(key)
	if err != nil {
		panic(err)
	}

	return value
}

func (c *Config) TryGet(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("key cant be empty")
	}

	c.dataLock.RLock()
	defer c.dataLock.RUnlock()

	cv, ok := c.data[key]
	if !ok {
		return "", fmt.Errorf("undeclared config value '%s' cant be accessed", key)
	}
	if cv.value == nil {
		return "", fmt.Errorf("uninitialized config value '%s' cant be accessed", key)
	}

	cv.getCounter.Add(1)

	return *cv.value, nil
}

func (c *Config) Set(key string, value string) {
	err := c.TrySet(key, value)
	if err != nil {
		panic(err)
	}
}

func (c *Config) TrySet(key string, value string) error {
	err := c.set(key, value)
	if err != nil {
		return err
	}
	c.runCallbacks(key, value)

	return nil
}

func (c *Config) set(key string, value string) error {
	c.dataLock.Lock()
	defer c.dataLock.Unlock()

	cv, ok := c.data[key]
	if !ok {
		return fmt.Errorf("key '%s' has to be declared before a value can be set", key)
	}

	if cv.declaration.Validate != nil {
		err := cv.declaration.Validate(value)
		if err != nil {
			return fmt.Errorf("Validation failed for '%s' while validating value provided by `Set()` -> %s", cv.declaration.Key, err.Error())
		}
	}

	cv.value = &value
	cv.setCounter.Add(1)

	return nil
}

func (c *Config) runCallbacks(key string, value string) {
	c.cbsLock.RLock()
	defer c.cbsLock.RUnlock()

	for _, cb := range c.cbs {
		cb(key, value)
	}
}

func (c *Config) OnAfterChange(cb func(key string, value string)) {
	c.cbsLock.Lock()
	defer c.cbsLock.Unlock()
	c.cbs = append(c.cbs, cb)
}

type Usage struct {
	Key         string
	Initialized bool
	SetCalls    uint64
	GetCalls    uint64
}

func (c *Config) GetUsage() []Usage {
	c.dataLock.Lock()
	defer c.dataLock.Unlock()

	usages := []Usage{}
	for key, value := range c.data {
		usages = append(usages, Usage{
			Key:         key,
			Initialized: value.value != nil,
			SetCalls:    value.setCounter.Load(),
			GetCalls:    value.getCounter.Load(),
		})
	}

	sort.Slice(usages, func(i, j int) bool {
		return usages[i].Key < usages[j].Key
	})

	return usages
}

func (c *Config) Init() {
	c.dataLock.Lock()
	defer c.dataLock.Unlock()

	// Load ENV variables
	for key, cv := range c.data {
		value, ok := os.LookupEnv(key)
		if ok {
			if cv.declaration.Validate != nil {
				err := cv.declaration.Validate(value)
				assert.Assert(err == nil, fmt.Errorf("Validation failed for '%s' while parsing env '%s' -> %s", cv.declaration.Key, key, err))
			}
			cv.value = &value
			continue
		}
		for _, envAlias := range cv.declaration.Envs {
			value, ok := os.LookupEnv(envAlias)
			if ok {
				if cv.declaration.Validate != nil {
					err := cv.declaration.Validate(value)
					assert.Assert(err == nil, fmt.Errorf("Validation failed for '%s' while parsing env '%s' -> %s", cv.declaration.Key, envAlias, err))
				}
				cv.value = &value
				break
			}
		}
	}

	// Initialize cobra args
	if c.cobraCmd != nil {
		for _, cv := range c.data {
			if cv.declaration.Cobra != nil {
				v := ""
				cv.declaration.Cobra.CobraValue = &v
				name := cv.declaration.Cobra.Name
				value := ""
				if cv.declaration.DefaultValue != nil {
					value = *cv.declaration.DefaultValue
				}
				usage := fmt.Sprintf(`(env "%s")`, cv.declaration.Key)
				if cv.declaration.Description != nil {
					usage = *cv.declaration.Description + fmt.Sprintf(` (env "%s")`, cv.declaration.Key)
				}
				switch cv.declaration.Cobra.Short == nil {
				case true:
					c.cobraCmd.PersistentFlags().StringVar(cv.declaration.Cobra.CobraValue, name, value, usage)
				case false:
					c.cobraCmd.PersistentFlags().StringVarP(cv.declaration.Cobra.CobraValue, name, *cv.declaration.Cobra.Short, value, usage)
				}
			}
		}
	}
}

func (c *Config) AsEnvs() string {
	c.dataLock.Lock()
	defer c.dataLock.Unlock()

	keys := []string{}
	for key := range c.data {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	data := ""
	for _, key := range keys {
		cv, ok := c.data[key]
		assert.Assert(ok, "key has to exist since this has exclusive access to the map and it was just extracted from the map")

		data = data + "## Key: " + cv.declaration.Key + "\n"
		if cv.declaration.Description != nil {
			data = data + "## Description: " + *cv.declaration.Description + "\n"
		}

		if cv.declaration.DefaultValue != nil {
			data = data + "## Default: " + strings.ReplaceAll(*cv.declaration.DefaultValue, "\n", "\\n") + "\n"
		}

		data = data + "## Has Validation: " + strconv.FormatBool(cv.declaration.Validate != nil) + "\n"

		if cv.declaration.Envs != nil {
			data = data + fmt.Sprintf("## Envs: %#v", cv.declaration.Envs) + "\n"
		} else {
			data = data + "## Envs: []string{}\n"
		}

		if cv.declaration.Cobra != nil {
			data = data + "## CLI Args:\n"
			data = data + "##   Long: --" + cv.declaration.Cobra.Name + "\n"
			if cv.declaration.Cobra.Short != nil {
				data = data + "##   Short: -" + *cv.declaration.Cobra.Short + "\n"
			}
		} else {
			data = data + "## CLI Args: None\n"
		}

		value := ""
		if cv.value != nil {
			value = *cv.value
		}
		data = data + key + "=" + strings.ReplaceAll(value, "\n", "\\n") + "\n\n"
	}
	data = strings.TrimSpace(data) + "\n"

	return data
}
