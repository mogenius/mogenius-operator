package interfaces

type ConfigModule interface {
	// `Declare()` a config value without an initial value.
	Declare(opts ConfigDeclaration)

	// Same as `TryGet()`. Panics if it fails.
	Get(key string) string

	// Get a List of all Variables
	GetAll() []ConfigVariable

	// `Try` to `Get` a config value.
	//
	// Fails if the `key` was not initialized.
	TryGet(key string) (string, error)

	// Same as `TrySet()`. Panics if it fails.
	Set(key string, value string)

	// `Try` to `Set` the value for a `key`.
	//
	// Fails if:
	//   - key has not been declared
	//   - a validation was provided and failed
	TrySet(key string, value string) error

	// Register a callback for whenever a `value` is `Set()`.
	//
	// Providing `keys == nil` normalizes to `keys = []string{}`.
	//
	// - if `len(keys) == 0`: trigger on **all** changes
	// - else: trigger only when the provided keys change
	OnChanged(keys []string, cb func(key string, value string, isSecret bool))

	// Load ENVs for declared configs.
	LoadEnvs()

	// Export all configs in a format for .env files
	AsEnvs() string

	// Check all values are initialized. Exits the program if issues have been found.
	Validate()
}

type ConfigDeclaration struct {
	// (required) Key of the config value
	Key string
	// (optional) Initial value
	DefaultValue *string
	// (optional) Human readable description
	Description *string
	// (optional) Declare the variable as confidential
	IsSecret bool
	// (optional) Declare the variable as read-only
	ReadOnly bool
	// (optional) List of ENV variables to lookup while in Init()
	Envs []string
	// (optional) Validation to check if user provided values are valid
	Validate func(value string) error
}

type ConfigVariable struct {
	Key      string
	Value    string
	IsSecret bool
}
