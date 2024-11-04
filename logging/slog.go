package logging

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/TylerBrock/colorjson"
	"github.com/go-git/go-git/v5/plumbing/color"
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

type BufferedLogWriter struct {
	combinedLogWriter  io.Writer
	componentLogWriter io.Writer
	buffer             *bytes.Buffer
}

func newBufferedLogWriter(combinedLogWriter io.Writer, componentLogWriter io.Writer) BufferedLogWriter {
	return BufferedLogWriter{
		combinedLogWriter:  combinedLogWriter,
		componentLogWriter: componentLogWriter,
		buffer:             &bytes.Buffer{},
	}
}

func isJson(str string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(str), &js) == nil
}

func (bw *BufferedLogWriter) Write(p []byte) (n int, err error) {
	n, err = bw.buffer.Write(p)
	if err != nil {
		return n, err
	}

	scanner := bufio.NewScanner(bw.buffer)
	for scanner.Scan() {
		line := scanner.Text()

		if !isJson(line) {
			f, err := os.OpenFile("log_errors.txt", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
			if err != nil {
				panic(err)
			}
			defer f.Close()
			if _, err = f.WriteString(line + "\n\n"); err != nil {
				panic(err)
			}

			continue
		}

		line = eraseSecrets(line)

		go prettyPrintSlogLine(line)

		_, err := bw.combinedLogWriter.Write([]byte(line + "\n"))
		if err != nil {
			return n, err
		}
		_, err = bw.componentLogWriter.Write([]byte(line + "\n"))
		if err != nil {
			return n, err
		}
	}

	return n, err
}

// Feature: rewrite log stream to [REDACT] known secrets
func eraseSecrets(data string) string {
	for _, b := range SecretArray() {
		data = strings.ReplaceAll(data, b, REDACTED)
	}
	return data
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

	writer := newBufferedLogWriter(m.combinedLogWriter, logFileWriter(m.logPath, componentId))

	handlerOptions := slog.HandlerOptions{
		AddSource:   true,
		Level:       slog.LevelDebug,
		ReplaceAttr: nil,
	}

	logger := slog.New(slog.NewJSONHandler(&writer, &handlerOptions))

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

func prettyPrintSlogLine(line string) {
	var data map[string]interface{}
	err := json.Unmarshal([]byte(line), &data)
	if err != nil {
		panic(err)
	}

	// unused fields
	delete(data, "key")
	delete(data, "time")

	// extract source and create a sourceline format the IDE understands
	source, ok := data["source"].(map[string]interface{})
	if !ok {
		panic("failed to cast source location")
	}
	file := source["file"].(string)
	if strings.Contains(file, "mogenius-k8s-manager/") {
		file = strings.SplitAfterN(file, "mogenius-k8s-manager/", 2)[1]
	}
	sourceLine := source["line"].(float64)
	sourceLocation := fmt.Sprintf("%s%s:%.0f%s", color.Faint, file, sourceLine, color.Reset)
	delete(data, "source")

	// extract and colorize component
	component := data["component"].(string)
	component = color.Magenta + component + color.Reset
	delete(data, "component")

	// extract and colorize level
	level := data["level"].(string)
	switch level {
	case "DEBUG":
		level = color.Cyan + level + color.Reset
	case "INFO":
		level = color.Green + level + color.Reset
	case "WARN":
		level = color.Yellow + level + color.Reset
	case "ERROR":
		level = color.Red + level + color.Reset
	default:
		panic(fmt.Errorf("unsupported error level: %s", level))
	}
	delete(data, "level")

	// extract the logmessage
	message := data["msg"].(string)
	delete(data, "msg")

	// create a colored single-line json string for all remaining data (aka. every additonal datapoint passed to slog)
	prettyData, err := colorjson.Marshal(data)
	if err != nil {
		panic(err)
	}

	// print the output
	output := fmt.Sprintf("%s %s %s %s", level, component, sourceLocation, message)
	if !bytes.Equal(prettyData, []byte("{}")) {
		output = fmt.Sprintf("%s %s", output, prettyData)
	}
	fmt.Println(output)
}
