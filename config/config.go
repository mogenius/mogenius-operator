package config

import (
	"fmt"
	"mogenius-k8s-manager/assert"
	"mogenius-k8s-manager/interfaces"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/spf13/cobra"
)

type Config struct {
	data                     map[string]*configValue
	dataLock                 sync.RWMutex
	onChangedCallbacks       []onChangeCallbackConfig
	onChangedCallbacksLock   sync.RWMutex
	onFinalizedCallbacks     []func()
	onFinalizedCallbacksLock sync.RWMutex
	cobraCmd                 *cobra.Command
}

type configValue struct {
	value       *string
	declaration interfaces.ConfigDeclaration
	getCounter  atomic.Uint64
	setCounter  atomic.Uint64
}

type onChangeCallbackConfig struct {
	onKeys   []string
	callback func(key string, value string, isSecret bool)
}

func NewConfig() *Config {
	return &Config{
		data:                     make(map[string]*configValue),
		dataLock:                 sync.RWMutex{},
		onChangedCallbacks:       []onChangeCallbackConfig{},
		onChangedCallbacksLock:   sync.RWMutex{},
		onFinalizedCallbacks:     []func(){},
		onFinalizedCallbacksLock: sync.RWMutex{},
	}
}

func (c *Config) Validate() {
	errs := []error{}
	func() {
		c.dataLock.RLock()
		defer c.dataLock.RUnlock()

		for key, cv := range c.data {
			if cv.value == nil {
				errs = append(errs, fmt.Errorf("Value for Key '%s' is not initialized.", key))
				continue
			}
			if cv.declaration.Validate != nil {
				err := cv.declaration.Validate(*cv.value)
				if err != nil {
					errs = append(errs, fmt.Errorf("Validation for Key '%s' failed: %s", key, err.Error()))
					continue
				}
			}
		}
	}()

	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Printf("ERROR: %s\n", err.Error())
		}
		fmt.Printf("Found %d error(s) when validating configuration values.\n", len(errs))
		fmt.Println()
		fmt.Println("Configuration Values")
		fmt.Println()
		fmt.Println("```env")
		fmt.Print(c.AsEnvs())
		fmt.Println("```")
		fmt.Println()
		os.Exit(1)
	}
}

func (c *Config) WithCobraCmd(cmd *cobra.Command) {
	assert.Assert(cmd != nil)
	assert.Assert(c.cobraCmd == nil)
	c.cobraCmd = cmd
	cobra.OnInitialize(c.loadCobraArgs)
}

func (c *Config) loadCobraArgs() {
	func() {
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
	}()
	c.runOnFinalizedCallbacks()
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
		c.runOnChangedCallbacks(opts.Key, *opts.DefaultValue, opts.IsSecret)
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
	isSecret := c.isSecret(key)
	c.runOnChangedCallbacks(key, value, isSecret)

	return nil
}

func (c *Config) isSecret(key string) bool {
	c.dataLock.RLock()
	defer c.dataLock.RUnlock()

	for _, cv := range c.data {
		if cv.declaration.Key == key {
			return cv.declaration.IsSecret
		}
	}

	return false
}

func (c *Config) set(key string, value string) error {
	c.dataLock.Lock()
	defer c.dataLock.Unlock()

	cv, ok := c.data[key]
	if !ok {
		return fmt.Errorf("key '%s' has to be declared before a value can be set", key)
	}

	if cv.declaration.ReadOnly {
		return fmt.Errorf("tried to set config value for Read-Only key: %s", key)
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

func (c *Config) runOnChangedCallbacks(key string, value string, isSecret bool) {
	c.onChangedCallbacksLock.RLock()
	defer c.onChangedCallbacksLock.RUnlock()

	for _, callbackConfig := range c.onChangedCallbacks {
		assert.Assert(callbackConfig.onKeys != nil, "the API is expected to prevent callbackConfig.onKeys being nil")
		// trigger if `onKeys` is empty
		if len(callbackConfig.onKeys) == 0 {
			callbackConfig.callback(key, value, isSecret)
			continue
		}
		// trigger if `onKeys` contains the changed key
		if slices.Contains(callbackConfig.onKeys, key) {
			callbackConfig.callback(key, value, isSecret)
			continue
		}
	}
}

func (c *Config) OnChanged(keys []string, callback func(key string, value string, isSecret bool)) {
	assert.Assert(callback != nil)
	c.onChangedCallbacksLock.Lock()
	defer c.onChangedCallbacksLock.Unlock()

	if keys == nil {
		keys = []string{}
	}

	callbackConfig := onChangeCallbackConfig{
		onKeys:   keys,
		callback: callback,
	}

	c.onChangedCallbacks = append(c.onChangedCallbacks, callbackConfig)
}

func (c *Config) runOnFinalizedCallbacks() {
	c.onFinalizedCallbacksLock.RLock()
	defer c.onFinalizedCallbacksLock.RUnlock()

	for _, callback := range c.onFinalizedCallbacks {
		assert.Assert(callback != nil, "the API is expected to prevent callback being nil")
		callback()
	}
}

func (c *Config) OnFinalized(callback func()) {
	assert.Assert(callback != nil)
	c.onFinalizedCallbacksLock.Lock()
	defer c.onFinalizedCallbacksLock.Unlock()

	c.onFinalizedCallbacks = append(c.onFinalizedCallbacks, callback)
}

type Usage struct {
	Key         string
	Initialized bool
	SetCalls    uint64
	GetCalls    uint64
}

func (c *Config) GetUsage() []Usage {
	c.dataLock.RLock()
	defer c.dataLock.RUnlock()

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
	triggerFinalizedCallbacks := false
	func() {
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

		triggerFinalizedCallbacks = c.cobraCmd == nil
	}()

	if triggerFinalizedCallbacks {
		c.runOnFinalizedCallbacks()
	}
}

func (c *Config) AsEnvs() string {
	c.dataLock.RLock()
	defer c.dataLock.RUnlock()

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
			defaultValue := strings.ReplaceAll(*cv.declaration.DefaultValue, "\n", "\\n")
			if defaultValue == "" {
				defaultValue = `""`
			}
			data = data + "## Default: " + defaultValue + "\n"
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

func (c *Config) GetAll() []interfaces.ConfigVariable {
	configVariables := []interfaces.ConfigVariable{}
	for key, cv := range c.data {
		if cv.value != nil {
			configVariables = append(configVariables, interfaces.ConfigVariable{
				Key:      key,
				Value:    *cv.value,
				IsSecret: cv.declaration.IsSecret,
			})
		}
	}
	return configVariables
}
