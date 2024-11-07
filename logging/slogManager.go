package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

const combinedLogComponentName = "all"
const logfileMaxBackups int = 10
const logfileMaxSize int = 10
const logfileCompress bool = true

type SlogManager struct {
	logDir            string
	combinedLogWriter io.Writer
	activeLoggers     map[string]*slog.Logger
}

func NewSlogManager(logDir string) SlogManager {
	absLogDir, err := filepath.Abs(logDir)
	assert(err == nil, fmt.Errorf("failed to resolve absolut logDir('%s'): %s", logDir, err))
	slogManager := SlogManager{
		logDir: absLogDir,
		combinedLogWriter: &lumberjack.Logger{
			Filename:   filepath.Join(absLogDir, combinedLogComponentName+".log"), // Path to log file
			MaxSize:    logfileMaxSize,                                            // Max size in megabytes before rotation
			MaxBackups: logfileMaxBackups,                                         // Max number of old log files to keep
			Compress:   logfileCompress,                                           // Compress old log files
		},
		activeLoggers: make(map[string]*slog.Logger),
	}

	return slogManager
}

type LoggerOptions struct {
	Component string
}

func logFileWriter(logDir string, componentId string) io.Writer {
	return &lumberjack.Logger{
		Filename:   filepath.Join(logDir, componentId+".log"),
		MaxSize:    logfileMaxSize,
		MaxBackups: logfileMaxBackups,
		Compress:   logfileCompress,
	}
}

func (m *SlogManager) getLogDir() string {
	return m.logDir
}

func (m *SlogManager) ComponentLogPath(componentId string) (string, error) {
	if strings.Contains(componentId, "/") {
		return "", fmt.Errorf("componentId may not contain '/': %s", componentId)
	}
	return filepath.Join(m.getLogDir(), componentId+".log"), nil
}

func (m *SlogManager) GetLogger(componentId string) (*slog.Logger, error) {
	logger := m.activeLoggers[componentId]
	if logger != nil {
		return logger, nil
	}

	return nil, fmt.Errorf("logger '%s' does not exist", componentId)
}

func (m *SlogManager) CreateLogger(componentId string) *slog.Logger {
	assert(componentId != combinedLogComponentName, fmt.Errorf("the componentId '%s' is not disallowed because it is reserved", combinedLogComponentName))
	assert(m.activeLoggers[componentId] == nil, fmt.Errorf("logger was requested multiple times: %s", componentId))
	m.createLogdir()

	err := os.MkdirAll(m.getLogDir(), os.ModePerm)
	assert(err == nil, fmt.Errorf("failed to create log with logDir('%s'): %#v", m.getLogDir(), err))

	logger := slog.New(NewMogeniusSlogHandler(m.combinedLogWriter, logFileWriter(m.getLogDir(), componentId)))
	logger = logger.With("component", componentId)

	m.activeLoggers[componentId] = logger

	return logger
}

func (m *SlogManager) createLogdir() {
	err := os.MkdirAll(m.getLogDir(), os.ModePerm)
	assert(err == nil, fmt.Errorf("failed to create log directory('%s'): %#v", m.logDir, err))
}

func assert(condition bool, message any) {
	if !condition {
		panic(message)
	}
}
