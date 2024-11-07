package interfaces

import "log/slog"

type LogManagerModule interface {
	// Absolute path to the logfile of a component
	ComponentLogPath(componentId string) (string, error)
	// Get the pointer to an existing logger by its componentId
	GetLogger(componentId string) (*slog.Logger, error)
	// Create a new logger with a unique componentId
	CreateLogger(componentId string) *slog.Logger
}
