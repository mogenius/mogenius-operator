package interfaces

import "log/slog"

type LogManager interface {
	LogDir() string
	CombinedLogPath() string
	// Get the pointer to an existing logger by its componentId
	GetLogger(componentId string) (*slog.Logger, error)
	// Create a new logger with a unique componentId
	CreateLogger(componentId string) *slog.Logger
}
