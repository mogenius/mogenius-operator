package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/shell"
	"os"
	"path"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nwidger/jsoncolor"
)

const combinedLogComponentName = "all"

type SlogManager interface {
	// Get the pointer to an existing logger by its componentId
	GetLogger(componentId string) (*slog.Logger, error)
	// Create a new logger with a unique componentId
	CreateLogger(componentId string) *slog.Logger

	CombinedLogPath() (string, error)
	ComponentLogPath(componentId string) (string, error)
}

type slogManager struct {
	logLevel slog.Level
	handlers []slog.Handler

	activeLoggers     map[string]*slog.Logger
	resolvedLogDir    *string
	combinedLogWriter io.Writer
}

var _ SlogManager = &slogManager{}

func NewSlogManager(logLevel slog.Level, handlers []slog.Handler) SlogManager {
	self := slogManager{}

	self.logLevel = logLevel
	if handlers == nil {
		handlers = []slog.Handler{}
	}
	self.handlers = handlers
	self.activeLoggers = map[string]*slog.Logger{}
	self.combinedLogWriter = nil

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

	for _, handler := range self.handlers {
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
	Time      time.Time
	Payload   map[string]any
}

func (self *LogLine) ToJson() string {
	data, err := json.Marshal(self)
	assert.Assert(err == nil, err)
	return string(data)
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

// ########################
// # +------------------+ #
// # | SlogMultiHandler | #
// # +------------------+ #
// ########################

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
	for _, handler := range self.inner {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (self *SlogMultiHandler) Handle(ctx context.Context, record slog.Record) error {
	errors := []error{}
	errorLock := sync.Mutex{}

	var wg sync.WaitGroup
	for _, handler := range self.inner {
		if handler.Enabled(ctx, record.Level) {
			wg.Go(func() {
				err := handler.Handle(ctx, record)
				if err != nil {
					errorLock.Lock()
					errors = append(errors, err)
					errorLock.Unlock()
				}
			})
		}
	}
	wg.Wait()

	if len(errors) > 0 {
		errorMessages := []string{}
		for _, err := range errors {
			errorMessages = append(errorMessages, err.Error())
		}
		return fmt.Errorf("Failed to dispatch all log messages. Enountered %d errors: %#v", len(errors), errorMessages)
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

// ##########################
// # +--------------------+ #
// # | PrettyPrintHandler | #
// # +--------------------+ #
// ##########################

type PrettyPrintHandler struct {
	out          io.Writer
	colors       bool
	logLevel     slog.Level
	logFilter    []string
	attrs        []slog.Attr
	group        string
	logMessageTx chan string
	filterFunc   func(msg string) string
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
	self.logMessageTx = make(chan string)

	self.startDebouncedPrinter()

	return self
}

func (self *PrettyPrintHandler) startDebouncedPrinter() {
	type MessageWithCount struct {
		message string
		count   uint32
	}
	go func() {
		printedMessages := []string{}
		messageQueue := []string{}

		flush := time.NewTicker(1 * time.Second)
		defer flush.Stop()

		for {
			select {
			case msg := <-self.logMessageTx:
				if !slices.Contains(printedMessages, msg) {
					_, err := self.out.Write([]byte(msg))
					assert.Assert(err == nil, err)
					printedMessages = append(printedMessages, msg)
					continue
				}
				messageQueue = append(messageQueue, msg)
			case <-flush.C:
				printedMessages = []string{}

				messagesWithCount := []MessageWithCount{}
				for _, msg := range messageQueue {
					found := false
					for idx := range messagesWithCount {
						if msg == messagesWithCount[idx].message {
							messagesWithCount[idx].count = messagesWithCount[idx].count + 1
							found = true
							break
						}
					}
					if !found {
						messagesWithCount = append(messagesWithCount, MessageWithCount{
							message: msg,
							count:   1,
						})
					}
				}
				messageQueue = []string{}

				for _, messageWithCount := range messagesWithCount {
					_, err := self.out.Write([]byte("(" + strconv.FormatUint(uint64(messageWithCount.count), 10) + "x)" + messageWithCount.message))
					assert.Assert(err == nil, err)
				}
			}
		}
	}()
}

func (self *PrettyPrintHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= self.logLevel.Level()
}

func (self *PrettyPrintHandler) Handle(ctx context.Context, record slog.Record) error {
	if !self.Enabled(ctx, record.Level) {
		return nil
	}
	component, err := self.getComponent()
	assert.Assert(err == nil, "the SlogManager enforces an component attribute to exist", err)

	// Apply LOG_FILTER
	if len(self.logFilter) > 0 && !slices.Contains(self.logFilter, component) {
		return nil
	}

	logLine := LogLine{}

	logLine.Level = strings.Split(record.Level.String(), "+")[0]
	logLine.Component = component
	logLine.Scope = self.tryGetScope()
	logLine.Source = slogRecordToSourceString(record)
	logLine.Message = record.Message
	logLine.Time = record.Time
	logLine.Payload = slogRecordToPayload(record, self.filterFunc)

	logMessage := self.formatLogMessage(logLine, self.colors)
	self.logMessageTx <- logMessage

	return nil
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
	other.logMessageTx = make(chan string)
	other.startDebouncedPrinter()

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
	other.logMessageTx = make(chan string)
	other.startDebouncedPrinter()

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

func (self *PrettyPrintHandler) formatLogMessage(
	logLine LogLine,
	enableColor bool,
) string {
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

	logMessage := fmt.Sprintf(
		"%s %s %s %s %s",
		logLine.Level,
		logLine.Component,
		logLine.Source,
		logLine.Message,
		payloadString,
	) + "\n"

	return logMessage
}

// ############################
// # +----------------------+ #
// # | RecordChannelHandler | #
// # +----------------------+ #
// ############################

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
	assert.Assert(buffersize >= 2, "log message buffer needs a size of at least 2")

	self := RecordChannelHandler{}

	self.buffersize = buffersize
	self.recordChannelTx = make(chan LogLine, buffersize)
	self.logLevel = logLevel
	self.filterFunc = filterFunc

	return &self
}

func (self *RecordChannelHandler) GetRecordChannel() chan LogLine {
	return self.recordChannelTx
}

func (self *RecordChannelHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= self.logLevel.Level()
}

func (self *RecordChannelHandler) Handle(ctx context.Context, record slog.Record) error {
	if !self.Enabled(ctx, record.Level) {
		return nil
	}
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
	logLine.Time = record.Time
	logLine.Payload = slogRecordToPayload(record, self.filterFunc)

	if len(self.recordChannelTx) >= self.buffersize {
		fmt.Fprintf(os.Stderr, "[WARNING] Logline buffer exhausted. Dropping the oldest entry.\n")
		<-self.recordChannelTx
	}
	self.recordChannelTx <- logLine

	return nil
}

func (self *RecordChannelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	other := &RecordChannelHandler{}

	other.recordChannelTx = self.recordChannelTx
	other.buffersize = self.buffersize
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
	other.buffersize = self.buffersize
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
