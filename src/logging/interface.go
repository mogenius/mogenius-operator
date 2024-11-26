package logging

import (
	"fmt"
	"io"
	"log/slog"
	"testing"
)

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

// TODO: replace with a mocking framework like: https://github.com/uber-go/mock
//
// Since this is only a logger we san simply always provide a default logger from golangs stdlib
type MockSlogManager struct {
	writer io.Writer
}

type testWriter struct {
	t *testing.T
}

func (w *testWriter) Write(p []byte) (n int, err error) {
	w.t.Log(string(p))
	return len(p), nil
}

func NewMockSlogManager(t *testing.T) *MockSlogManager {
	return &MockSlogManager{
		writer: &testWriter{t: t},
	}
}

func (m *MockSlogManager) ComponentLogPath(componentId string) (string, error) {
	return "", fmt.Errorf("cant get component log path of mock slog manager")
}

func (m *MockSlogManager) GetLogger(componentId string) (*slog.Logger, error) {
	return slog.New(slog.NewJSONHandler(m.writer, nil)).With("component", componentId), nil
}

func (m *MockSlogManager) CreateLogger(componentId string) *slog.Logger {
	return slog.New(slog.NewJSONHandler(m.writer, nil)).With("component", componentId)
}

func (m *MockSlogManager) SetLogLevel(componentId string) error {
	return nil
}

func (m *MockSlogManager) SetLogFilter(componentId string) error {
	return nil
}
