package logging

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
)

var logPath = "logs"
var combinedLogComponent = "full"

type LoggerOptions struct {
	Component string
}

type LoggingProxyWriter struct {
	outputWriters io.Writer
}

func (w *LoggingProxyWriter) Write(p []byte) (n int, err error) {
	p = eraseSecrets(p)
	return w.outputWriters.Write(p)
}

// Feature: rewrite log stream to [REDACT] known secrets
func eraseSecrets(data []byte) []byte {
	redacted := []byte(REDACTED)
	for _, b := range SecretBytesArray() {
		data = bytes.ReplaceAll(data, b, redacted)
	}
	return data
}

func logFileWriter(logPath string, component string) io.Writer {
	filename := fmt.Sprintf("%s/%s.log", logPath, component)
	fileMain, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(fmt.Errorf("failed to open log file with filename('%s'): %+v", filename, err))
	}
	return fileMain
}

func CreateLogger(component string) *slog.Logger {

	err := os.MkdirAll(logPath, os.ModePerm)
	if err != nil {
		panic(fmt.Errorf("failed to create log with logPath('%s'): %+v", logPath, err))
	}

	writer := &LoggingProxyWriter{
		outputWriters: io.MultiWriter(
			os.Stderr,
			logFileWriter(logPath, combinedLogComponent),
			logFileWriter(logPath, component),
		),
	}

	handlerOptions := slog.HandlerOptions{
		AddSource:   false, // this enables source file location in outputs
		Level:       slog.LevelInfo,
		ReplaceAttr: nil,
	}

	logger := slog.New(slog.NewJSONHandler(writer, &handlerOptions))

	logger = logger.With("component", component)

	return logger
}

func MainLogPath() string {
	return fmt.Sprintf("%s/%s.log", logPath, combinedLogComponent)
}
