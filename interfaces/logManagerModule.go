package interfaces

import "log/slog"

type LogManagerModule interface {
	// Absolute path to the logfile of a component
	ComponentLogPath(componentId string) (string, error)
	// Get the pointer to an existing logger by its componentId
	GetLogger(componentId string) (*slog.Logger, error)
	// Create a new logger with a unique componentId
	CreateLogger(componentId string) *slog.Logger
	// Set a log level. Valid are: "debug", "info", "warn" or "error"
	SetLogLevel(level string) error
	// Set a log filter. A comma-separated list of component names.
	//
	// If filter == "": all components are printed.
	// Else: only listed components are printed.
	//
	// Example: filter="cmd,iac"
	SetLogFilter(filter string) error
}
