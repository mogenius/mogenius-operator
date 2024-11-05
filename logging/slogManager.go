package logging

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

const logfileMaxBackups int = 10
const logfileMaxSize int = 10
const logfileCompress bool = true

type SlogManager struct {
	logPath           string
	combinedLogWriter io.Writer
	activeLoggers     map[string]*slog.Logger
}

func NewSlogManager() SlogManager {
	slogManager := SlogManager{
		logPath: "logs",
		combinedLogWriter: &lumberjack.Logger{
			Filename:   "logs/full.log",   // Path to log file
			MaxSize:    logfileMaxSize,    // Max size in megabytes before rotation
			MaxBackups: logfileMaxBackups, // Max number of old log files to keep
			Compress:   logfileCompress,   // Compress old log files
		},
		activeLoggers: make(map[string]*slog.Logger),
	}
	err := slogManager.createLogdir()
	if err != nil {
		panic(err)
	}
	return slogManager
}

type LoggerOptions struct {
	Component string
}

func logFileWriter(logPath string, component string) io.Writer {
	return &lumberjack.Logger{
		Filename:   fmt.Sprintf("%s/%s.log", logPath, component),
		MaxSize:    logfileMaxSize,
		MaxBackups: logfileMaxBackups,
		Compress:   logfileCompress,
	}
}

func (m *SlogManager) LogDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	path := filepath.Join(cwd, m.logPath)
	path, err = filepath.Abs(path)
	if err != nil {
		log.Fatal(err)
	}
	return path
}

func (m *SlogManager) CombinedLogPath() string {
	return filepath.Join("logs/full.log")
}

func (m *SlogManager) GetLogger(componentId string) (*slog.Logger, error) {
	logger := m.activeLoggers[componentId]
	if logger != nil {
		return logger, nil
	}
	return nil, fmt.Errorf("logger '%s' does not exist", componentId)
}

func (m *SlogManager) CreateLogger(componentId string) *slog.Logger {
	if m.activeLoggers[componentId] != nil {
		panic(fmt.Errorf("logger was requested multiple times: %s", componentId))
	}
	err := os.MkdirAll(m.logPath, os.ModePerm)
	if err != nil {
		panic(fmt.Errorf("failed to create log with logPath('%s'): %+v", m.logPath, err))
	}
	logger := slog.New(NewMogeniusSlogHandler(m.combinedLogWriter, logFileWriter(m.logPath, componentId)))
	logger = logger.With("component", componentId)
	m.activeLoggers[componentId] = logger
	return logger
}

func (m *SlogManager) createLogdir() error {
	err := os.MkdirAll(m.logPath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create log directory('%s'): %+v", m.logPath, err)
	}
	return nil
}
