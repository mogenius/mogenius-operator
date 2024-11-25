package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mogenius-k8s-manager/src/shell"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/mattn/go-isatty"
	"github.com/nwidger/jsoncolor"
)

type MogeniusSlogHandler struct {
	logLevel          *slog.Level
	logFilter         *string
	loggerHandlerLock *sync.RWMutex
	enableStderr      *atomic.Bool
	stderr            *os.File
	inner             *slog.JSONHandler
	attrs             []slog.Attr
	group             string
}

func NewMogeniusSlogHandler(logLevel *slog.Level, logFilter *string, loggerHandlerLock *sync.RWMutex, enableStderr *atomic.Bool, writers ...io.Writer) slog.Handler {
	return &MogeniusSlogHandler{
		logLevel:          logLevel,
		logFilter:         logFilter,
		loggerHandlerLock: loggerHandlerLock,
		enableStderr:      enableStderr,
		stderr:            os.Stderr,
		attrs:             []slog.Attr{},
		group:             "",
		inner: slog.NewJSONHandler(io.MultiWriter(writers...), &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
			ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
				if attr.Value.Kind() == slog.KindString {
					val := attr.Value.String()
					val = eraseSecrets(val)
					attr.Value = slog.AnyValue(val)
				}
				return attr
			},
		}),
	}
}

func (h *MogeniusSlogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *MogeniusSlogHandler) Handle(ctx context.Context, record slog.Record) error {
	err := h.inner.Handle(ctx, record)
	if err != nil {
		return err
	}

	if !h.enableStderr.Load() {
		return nil
	}

	h.loggerHandlerLock.RLock()
	slogLevel := *h.logLevel
	logFilter := *h.logFilter
	h.loggerHandlerLock.RUnlock()
	logFilterComponents := strings.Split(logFilter, ",")

	var recordLevel slog.Level
	level := record.Level.String()
	switch level {
	case "DEBUG":
		recordLevel = slog.LevelDebug
	case "INFO":
		recordLevel = slog.LevelInfo
	case "WARN":
		recordLevel = slog.LevelWarn
	case "ERROR":
		recordLevel = slog.LevelError
	default:
		panic(fmt.Errorf("unsupported error level: %s", level))
	}

	// Apply LOG_LEVEL
	if int(recordLevel) < int(slogLevel) {
		return nil
	}

	component, err := h.getComponent()
	if err != nil {
		panic("The LogManager enforces an component attribute to exist: " + err.Error())
	}

	// Apply LOG_FILTER
	if logFilter != "" && !slices.Contains(logFilterComponents, component) {
		return nil
	}

	source, err := getSourceString(record)
	if err != nil {
		panic("Source string should always be parsable within this handler: " + err.Error())
	}

	message := record.Message

	payload := getPayload(record)

	err = printLogLine(h.stderr, isatty.IsTerminal(h.stderr.Fd()), level, component, source, message, payload)
	if err != nil {
		return err
	}

	return nil
}

func printLogLine(
	writer io.Writer,
	enableColor bool,
	level string,
	component string,
	source string,
	message string,
	payload map[string]any,
) error {
	payloadString := ""
	if enableColor {
		switch level {
		case "DEBUG":
			level = shell.Cyan + level + shell.Reset
		case "INFO":
			level = shell.Green + level + shell.Reset
		case "WARN":
			level = shell.Yellow + level + shell.Reset
		case "ERROR":
			level = shell.Red + level + shell.Reset
		default:
			panic(fmt.Errorf("unsupported error level: %s", level))
		}
		component = shell.Magenta + component + shell.Reset
		source = shell.Faint + source + shell.Reset
		message = shell.Normal + message + shell.Reset

		if len(payload) > 0 {
			data, err := jsoncolor.Marshal(payload)
			if err != nil {
				panic(fmt.Errorf("failed to marshal json: %s\n", err.Error()))
			}
			payloadString = string(data)
		}
	} else {
		if len(payload) > 0 {
			data, err := json.Marshal(payload)
			if err != nil {
				panic(fmt.Errorf("failed to marshal json: %s\n", err.Error()))
			}
			payloadString = string(data)
		}
	}

	_, err := writer.Write([]byte(fmt.Sprintf(
		"%s %s %s %s %s",
		level,
		component,
		source,
		message,
		payloadString,
	) + "\n"))
	if err != nil {
		return err
	}

	return nil
}

func (h *MogeniusSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.inner = h.inner.WithAttrs(attrs).(*slog.JSONHandler)
	h.attrs = append(h.attrs, attrs...)
	return h
}

func (h *MogeniusSlogHandler) WithGroup(name string) slog.Handler {
	h.inner = h.inner.WithGroup(name).(*slog.JSONHandler)
	h.group = name
	return h
}

func (h *MogeniusSlogHandler) getComponent() (string, error) {
	for _, attr := range h.attrs {
		if attr.Key == "component" {
			return attr.Value.String(), nil
		}
	}
	return "", fmt.Errorf("failed to find record component")
}

func getSourceString(record slog.Record) (string, error) {
	frame, _ := runtime.CallersFrames([]uintptr{record.PC}).Next()
	file := frame.File

	if strings.Contains(file, "mogenius-k8s-manager/") {
		file = strings.SplitAfterN(file, "mogenius-k8s-manager/", 2)[1]
	}

	return fmt.Sprintf("%s:%d", file, frame.Line), nil
}

func getPayload(record slog.Record) map[string]any {
	attrs := make(map[string]any)
	record.Attrs(func(a slog.Attr) bool {
		errorData, ok := a.Value.Any().(error)
		if ok {
			attrs[a.Key] = errorData.Error()
			return true
		}
		stringerData, ok := a.Value.Any().(fmt.Stringer)
		if ok {
			attrs[a.Key] = stringerData.String()
			return true
		}
		attrs[a.Key] = a.Value.Any()
		return true
	})

	return attrs
}

// Feature: rewrite log stream to [REDACT] known secrets
func eraseSecrets(data string) string {
	for _, b := range SecretArray() {
		data = strings.ReplaceAll(data, b, REDACTED)
	}
	return data
}
