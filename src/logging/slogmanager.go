package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/shell"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	jsoniter "github.com/json-iterator/go"
	"github.com/nwidger/jsoncolor"
	"gopkg.in/natefinch/lumberjack.v2"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

const combinedLogComponentName = "all"
const logfileMaxBackups int = 10
const logfileMaxSize int = 10
const logfileCompress bool = true

type SlogManager interface {
	// Get the pointer to an existing logger by its componentId
	GetLogger(componentId string) (*slog.Logger, error)
	// Create a new logger with a unique componentId
	CreateLogger(componentId string) *slog.Logger

	CombinedLogPath() (string, error)
	ComponentLogPath(componentId string) (string, error)
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

func (m *MockSlogManager) CombinedLogPath() (string, error) {
	return "", fmt.Errorf("cant get component log path of mock slog manager")
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

type slogManager struct {
	opts SlogManagerOpts

	activeLoggers     map[string]*slog.Logger
	resolvedLogDir    *string
	combinedLogWriter io.Writer
}

type SlogManagerOpts struct {
	LogLevel           slog.Level
	AdditionalHandlers []slog.Handler
	LogFileOpts        *SlogManagerOptsLogFile
	MessageReplace     func(msg string) string // filter function which gets each string field in log messages and allows to alter their content
}

type SlogManagerOptsLogFile struct {
	LogDir             *string
	EnableCombinedLog  bool // write all json logs to a single log file called "all.log" within LogDir
	EnableComponentLog bool // write json logs for each component into a dedicated file called "${component}.log" within LogDir
}

func NewSlogManager(opts SlogManagerOpts) SlogManager {
	self := slogManager{}

	self.opts = opts
	self.activeLoggers = map[string]*slog.Logger{}
	self.combinedLogWriter = nil

	if opts.AdditionalHandlers == nil {
		opts.AdditionalHandlers = []slog.Handler{}
	}

	if opts.LogFileOpts != nil {
		assert.Assert(opts.LogFileOpts.LogDir != nil)
		resolvedLogDir, err := filepath.Abs(*opts.LogFileOpts.LogDir)
		assert.Assert(err == nil, err)
		self.resolvedLogDir = &resolvedLogDir

		if opts.LogFileOpts.EnableCombinedLog {
			self.combinedLogWriter = &lumberjack.Logger{
				Filename:   filepath.Join(*self.resolvedLogDir, combinedLogComponentName+".log"), // Path to log file
				MaxSize:    logfileMaxSize,                                                       // Max size in megabytes before rotation
				MaxBackups: logfileMaxBackups,                                                    // Max number of old log files to keep
				Compress:   logfileCompress,                                                      // Compress old log files
			}
		}
	}

	return &self
}

func (m *slogManager) GetLogger(componentId string) (*slog.Logger, error) {
	logger := m.activeLoggers[componentId]
	if logger != nil {
		return logger, nil
	}

	return nil, fmt.Errorf("logger '%s' does not exist", componentId)
}

func (self *slogManager) CreateLogger(componentId string) *slog.Logger {
	assert.Assert(componentId != combinedLogComponentName, fmt.Errorf("the componentId '%s' is not disallowed because it is reserved", combinedLogComponentName))
	assert.Assert(self.activeLoggers[componentId] == nil, fmt.Errorf("logger was requested multiple times: %s", componentId))

	multiHandler := NewSlogMultiHandler()

	if self.opts.LogFileOpts != nil {
		logFileWriters := []io.Writer{}

		if self.opts.LogFileOpts.EnableCombinedLog {
			logFileWriters = append(logFileWriters, self.combinedLogWriter)
		}

		if self.opts.LogFileOpts.EnableComponentLog {
			logFileWriters = append(logFileWriters, &lumberjack.Logger{
				Filename:   filepath.Join(*self.resolvedLogDir, componentId+".log"),
				MaxSize:    logfileMaxSize,
				MaxBackups: logfileMaxBackups,
				Compress:   logfileCompress,
			})
		}

		handler := slog.NewJSONHandler(io.MultiWriter(logFileWriters...), &slog.HandlerOptions{
			AddSource: true,
			Level:     self.opts.LogLevel,
			ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
				if attr.Value.Kind() == slog.KindString {
					val := attr.Value.String()
					val = self.opts.MessageReplace(val)
					attr.Value = slog.AnyValue(val)
				}
				return attr
			},
		})

		multiHandler.AddHandler(handler)
	}

	for _, handler := range self.opts.AdditionalHandlers {
		multiHandler.AddHandler(handler)
	}

	logger := slog.New(multiHandler)

	logger = logger.With("component", componentId)

	self.activeLoggers[componentId] = logger

	return logger
}

func (self *slogManager) CombinedLogPath() (string, error) {
	if self.resolvedLogDir != nil {
		return path.Join(*self.resolvedLogDir, "all.log"), nil
	}

	return "", fmt.Errorf("logfiles are not enabled")
}

func (self *slogManager) ComponentLogPath(componentId string) (string, error) {
	if self.resolvedLogDir != nil {
		return path.Join(*self.resolvedLogDir, componentId+".log"), nil
	}

	return "", fmt.Errorf("logfiles are not enabled")
}

func ParseLogLevel(level string) (slog.Level, error) {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return slog.LevelDebug, nil
	case "INFO":
		return slog.LevelInfo, nil
	case "WARN":
		return slog.LevelWarn, nil
	case "ERROR":
		return slog.LevelError, nil
	}

	return slog.LevelInfo, fmt.Errorf("Unknown LogLevel")
}

type LogLine struct {
	Level     string
	Component string
	Scope     *string
	Source    string
	Message   string
	Payload   map[string]any
}

func (self *LogLine) ToJson() string {
	data, err := json.Marshal(self)
	assert.Assert(err == nil, err)
	return string(data)
}

// SlogMultiHandler

type SlogMultiHandler struct {
	inner []slog.Handler
}

func NewSlogMultiHandler() *SlogMultiHandler {
	self := &SlogMultiHandler{}
	self.inner = []slog.Handler{}

	return self
}

func (self *SlogMultiHandler) AddHandler(handler slog.Handler) {
	self.inner = append(self.inner, handler)
}

func (self *SlogMultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	var enabled *bool
	for _, handler := range self.inner {
		if enabled != nil {
			assert.Assert(*enabled == handler.Enabled(ctx, level))
			continue
		}

		handlerEnabled := handler.Enabled(ctx, level)
		enabled = &handlerEnabled
	}

	if enabled != nil {
		return *enabled
	}

	return false
}

func (self *SlogMultiHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, handler := range self.inner {
		err := handler.Handle(ctx, record)
		if err != nil {
			return err
		}
	}

	return nil
}

func (self *SlogMultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newMultiHandler := &SlogMultiHandler{}
	newMultiHandler.inner = []slog.Handler{}

	for _, handler := range self.inner {
		newMultiHandler.inner = append(newMultiHandler.inner, handler.WithAttrs(attrs))
	}

	return newMultiHandler
}

func (self *SlogMultiHandler) WithGroup(group string) slog.Handler {
	newMultiHandler := &SlogMultiHandler{}
	newMultiHandler.inner = []slog.Handler{}

	for _, handler := range self.inner {
		newMultiHandler.inner = append(newMultiHandler.inner, handler.WithGroup(group))
	}

	return newMultiHandler
}

// PrettyPrintHandler

type PrettyPrintHandler struct {
	out        io.Writer
	colors     bool
	logLevel   slog.Level
	logFilter  []string
	attrs      []slog.Attr
	group      string
	filterFunc func(msg string) string
}

func NewPrettyPrintHandler(
	out io.Writer,
	enableColors bool,
	logLevel slog.Level,
	logFilter []string,
	filterFunc func(msg string) string,
) slog.Handler {
	self := &PrettyPrintHandler{}

	self.out = out
	self.colors = enableColors
	self.logLevel = logLevel
	self.logFilter = logFilter
	self.attrs = []slog.Attr{}
	self.group = ""
	self.filterFunc = filterFunc

	return self
}

func (self *PrettyPrintHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= self.logLevel.Level()
}

func (self *PrettyPrintHandler) Handle(ctx context.Context, record slog.Record) error {
	component, err := self.getComponent()
	assert.Assert(err == nil, "the SlogManager enforces an component attribute to exist", err)

	// Apply LOG_FILTER
	if len(self.logFilter) > 0 && !slices.Contains(self.logFilter, component) {
		return nil
	}

	logLine := LogLine{}

	logLine.Level = record.Level.String()
	logLine.Component = component
	logLine.Scope = self.tryGetScope()
	logLine.Source = slogRecordToSourceString(record)
	logLine.Message = record.Message
	logLine.Payload = slogRecordToPayload(record, self.filterFunc)

	return self.printLogLine(
		self.out,
		self.colors,
		logLine,
	)
}

func (self *PrettyPrintHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	other := &PrettyPrintHandler{}

	other.out = self.out
	other.colors = self.colors
	other.logLevel = self.logLevel
	other.logFilter = self.logFilter
	other.attrs = append(self.attrs, attrs...)
	other.group = self.group
	other.filterFunc = self.filterFunc

	return other
}

func (self *PrettyPrintHandler) WithGroup(group string) slog.Handler {
	other := &PrettyPrintHandler{}

	other.out = self.out
	other.colors = self.colors
	other.logLevel = self.logLevel
	other.logFilter = self.logFilter
	other.attrs = self.attrs
	other.group = group
	other.filterFunc = self.filterFunc

	return other
}

func (self *PrettyPrintHandler) getComponent() (string, error) {
	for _, attr := range self.attrs {
		if attr.Key == "component" {
			return attr.Value.String(), nil
		}
	}
	return "", fmt.Errorf("failed to find record component")
}

func (self *PrettyPrintHandler) tryGetScope() *string {
	for _, attr := range self.attrs {
		if attr.Key == "scope" {
			scope := attr.Value.String()
			return &scope
		}
	}
	return nil
}

func (self *PrettyPrintHandler) printLogLine(
	writer io.Writer,
	enableColor bool,
	logLine LogLine,
) error {
	payloadString := ""
	if enableColor {
		switch logLine.Level {
		case "DEBUG":
			logLine.Level = shell.Cyan + logLine.Level + shell.Reset
		case "INFO":
			logLine.Level = shell.Green + logLine.Level + shell.Reset
		case "WARN":
			logLine.Level = shell.Yellow + logLine.Level + shell.Reset
		case "ERROR":
			logLine.Level = shell.Red + logLine.Level + shell.Reset
		default:
			panic(fmt.Errorf("unsupported error level: %s", logLine.Level))
		}
		logLine.Component = shell.Magenta + logLine.Component + shell.Reset
		if logLine.Scope != nil {
			mscope := shell.FaintYellow + *logLine.Scope + shell.Reset
			logLine.Component = logLine.Component + shell.Faint + "{" + shell.Reset + mscope + shell.Faint + "}" + shell.Reset
		}
		logLine.Source = shell.Faint + logLine.Source + shell.Reset
		logLine.Message = shell.Normal + logLine.Message + shell.Reset

		if len(logLine.Payload) > 0 {
			data, err := jsoncolor.Marshal(logLine.Payload)
			if err != nil {
				panic(fmt.Errorf("failed to marshal json: %s\n", err.Error()))
			}
			payloadString = string(data)
		}
	} else {
		if len(logLine.Payload) > 0 {
			data, err := json.Marshal(logLine.Payload)
			if err != nil {
				panic(fmt.Errorf("failed to marshal json: %s\n", err.Error()))
			}
			payloadString = string(data)
		}
	}

	_, err := writer.Write([]byte(fmt.Sprintf(
		"%s %s %s %s %s",
		logLine.Level,
		logLine.Component,
		logLine.Source,
		logLine.Message,
		payloadString,
	) + "\n"))
	if err != nil {
		return err
	}

	return nil
}

// RecordChannelHandler

type RecordChannelHandler struct {
	recordChannelTx chan LogLine
	buffersize      int
	logLevel        slog.Level
	logFilter       []string
	attrs           []slog.Attr
	group           string
	filterFunc      func(msg string) string
}

func NewRecordChannelHandler(buffersize int, logLevel slog.Level, filterFunc func(msg string) string) *RecordChannelHandler {
	self := RecordChannelHandler{}

	self.recordChannelTx = make(chan LogLine, buffersize)
	self.logLevel = logLevel
	self.filterFunc = filterFunc
	self.buffersize = buffersize

	return &self
}

func (self *RecordChannelHandler) GetRecordChannel() chan LogLine {
	return self.recordChannelTx
}

func (self *RecordChannelHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= self.logLevel.Level()
}

func (self *RecordChannelHandler) Handle(ctx context.Context, record slog.Record) error {
	record.Attrs(func(attr slog.Attr) bool {
		str, ok := attr.Value.Any().(string)
		if ok && self.filterFunc != nil {
			attr.Value = slog.StringValue(self.filterFunc(str))
		}
		return true
	})

	component, err := self.getComponent()
	assert.Assert(err == nil, "the SlogManager enforces an component attribute to exist", err)

	logLine := LogLine{}

	logLine.Level = record.Level.String()
	logLine.Component = component
	logLine.Scope = self.tryGetScope()
	logLine.Source = slogRecordToSourceString(record)
	logLine.Message = record.Message
	logLine.Payload = slogRecordToPayload(record, self.filterFunc)

	self.recordChannelTx <- logLine

	return nil
}

func (self *RecordChannelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	other := &RecordChannelHandler{}

	other.recordChannelTx = self.recordChannelTx
	other.logLevel = self.logLevel
	other.logFilter = self.logFilter
	other.attrs = append(self.attrs, attrs...)
	other.group = self.group
	other.filterFunc = self.filterFunc

	return other
}

func (self *RecordChannelHandler) WithGroup(group string) slog.Handler {
	other := &RecordChannelHandler{}

	other.recordChannelTx = self.recordChannelTx
	other.logLevel = self.logLevel
	other.logFilter = self.logFilter
	other.attrs = self.attrs
	other.group = group
	other.filterFunc = self.filterFunc

	return other
}

func (self *RecordChannelHandler) getComponent() (string, error) {
	for _, attr := range self.attrs {
		if attr.Key == "component" {
			return attr.Value.String(), nil
		}
	}
	return "", fmt.Errorf("failed to find record component")
}

func (self *RecordChannelHandler) tryGetScope() *string {
	for _, attr := range self.attrs {
		if attr.Key == "scope" {
			scope := attr.Value.String()
			return &scope
		}
	}
	return nil
}

func slogRecordToSourceString(record slog.Record) string {
	frame, _ := runtime.CallersFrames([]uintptr{record.PC}).Next()
	file := frame.File
	return fmt.Sprintf("%s:%d", file, frame.Line)
}

func slogRecordToPayload(record slog.Record, filterFunc func(data string) string) map[string]any {
	attrs := make(map[string]any)

	record.Attrs(func(attr slog.Attr) bool {
		str, ok := attr.Value.Any().(string)
		if ok && filterFunc != nil {
			filteredStr := filterFunc(str)
			attrs[attr.Key] = filteredStr
			return true
		}
		errorData, ok := attr.Value.Any().(error)
		if ok {
			attrs[attr.Key] = errorData.Error()
			return true
		}
		stringerData, ok := attr.Value.Any().(fmt.Stringer)
		if ok {
			attrs[attr.Key] = stringerData.String()
			return true
		}
		attrs[attr.Key] = attr.Value.Any()
		return true
	})

	return attrs
}
