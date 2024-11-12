package interfaces

import (
	"github.com/spf13/cobra"
)

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
	OnAfterChange(cb func(key string, value string, isSecret bool))

	// Initialize the config object.
	// This loads env variables and, if a cobra cmd is set, registers CLI flags.
	Init()

	// Export all configs in a format for .env files
	AsEnvs() string

	// Provide a cobra cmd to utilize cobra's CLI. Required for `ConfigDeclaration.Cobra` to work.
	WithCobraCmd(cmd *cobra.Command)

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
	// (optional) Cobra command variable to lookup while in Init()
	Cobra *ConfigCobraFlags
	// (optional) Validation to check if user provided values are valid
	Validate func(value string) error
}

type ConfigCobraFlags struct {
	// (required) Long cli Flag: --example
	Name string
	// (optional) Short cli Flag: -e
	Short *string
	// given to cobra to parse into
	CobraValue *string
}

type ConfigVariable struct {
	Key      string
	Value    string
	IsSecret bool
}
