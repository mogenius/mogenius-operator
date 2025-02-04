package logging

import (
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"gopkg.in/natefinch/lumberjack.v2"
)

const combinedLogComponentName = "all"
const logfileMaxBackups int = 10
const logfileMaxSize int = 10
const logfileCompress bool = true

type SlogManager struct {
	loggerHandlerLock sync.RWMutex
	logLevel          slog.Level
	logFilter         string
	logDir            *string
	enableStderr      atomic.Bool
	combinedLogWriter io.Writer
	activeLoggers     map[string]*slog.Logger
}

func NewSlogManager(logDir *string) *SlogManager {
	var err error
	if logDir != nil {
		*logDir, err = filepath.Abs(*logDir)
		assert(err == nil, fmt.Errorf("failed to resolve absolute logDir('%s'): %s", *logDir, err))
	}

	self := SlogManager{}
	self.loggerHandlerLock = sync.RWMutex{}
	self.logLevel = slog.LevelDebug
	self.logFilter = ""
	self.logDir = logDir
	self.enableStderr = atomic.Bool{}
	self.enableStderr.Store(true)
	if logDir == nil {
		self.combinedLogWriter = nil
	} else {
		self.combinedLogWriter = &lumberjack.Logger{
			Filename:   filepath.Join(*logDir, combinedLogComponentName+".log"), // Path to log file
			MaxSize:    logfileMaxSize,                                          // Max size in megabytes before rotation
			MaxBackups: logfileMaxBackups,                                       // Max number of old log files to keep
			Compress:   logfileCompress,                                         // Compress old log files
		}
	}
	self.activeLoggers = map[string]*slog.Logger{}

	return &self
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
	if m.logDir == nil {
		return ""
	}

	return *m.logDir
}

func (m *SlogManager) ComponentLogPath(componentId string) (string, error) {
	if strings.Contains(componentId, "/") {
		return "", fmt.Errorf("componentId may not contain '/': %s", componentId)
	}
	return filepath.Join(m.getLogDir(), componentId+".log"), nil
}

func (m *SlogManager) SetLogLevel(level string) error {
	m.loggerHandlerLock.Lock()
	defer m.loggerHandlerLock.Unlock()

	switch level {
	case "debug":
		m.logLevel = slog.LevelDebug
	case "info":
		m.logLevel = slog.LevelInfo
	case "warn":
		m.logLevel = slog.LevelWarn
	case "error":
		m.logLevel = slog.LevelError
	default:
		return fmt.Errorf("Unknown LogLevel: %s", level)
	}

	return nil
}

func (m *SlogManager) SetLogFilter(filter string) error {
	if strings.Contains(filter, "\n") {
		return fmt.Errorf("newlines are disallowed in log-filter")
	}

	m.loggerHandlerLock.Lock()
	defer m.loggerHandlerLock.Unlock()

	m.logFilter = filter

	return nil
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

	var handler slog.Handler
	if m.combinedLogWriter == nil {
		handler = NewMogeniusSlogHandler(
			&m.logLevel,
			&m.logFilter,
			&m.loggerHandlerLock,
			&m.enableStderr,
		)
	} else {
		handler = NewMogeniusSlogHandler(
			&m.logLevel,
			&m.logFilter,
			&m.loggerHandlerLock,
			&m.enableStderr,
			m.combinedLogWriter,
			logFileWriter(m.getLogDir(), componentId),
		)
	}
	logger := slog.New(handler)
	logger = logger.With("component", componentId)

	m.activeLoggers[componentId] = logger

	return logger
}

func (m *SlogManager) SetStderr(enabled bool) {
	m.enableStderr.Store(enabled)
}

func assert(condition bool, message any) {
	if !condition {
		panic(message)
	}
}
