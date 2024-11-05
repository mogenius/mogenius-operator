package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"strings"

	"github.com/TylerBrock/colorjson"
	"github.com/go-git/go-git/v5/plumbing/color"
)

type MogeniusSlogHandler struct {
	inner *slog.JSONHandler
	attrs []slog.Attr
	group string
}

func NewMogeniusSlogHandler(writers ...io.Writer) slog.Handler {
	return &MogeniusSlogHandler{
		attrs: []slog.Attr{},
		group: "",
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

	level := record.Level.String()
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

	component, err := h.getComponent()
	if err != nil {
		panic(fmt.Errorf("handler did not receive a component: %#v", err))
	}
	component = color.Magenta + component + color.Reset

	source, err := getSourceString(record)
	if err != nil {
		panic(err)
	}
	source = color.Faint + source + color.Reset

	message := record.Message

	logLine := fmt.Sprintf("%s %s %s %s", level, component, source, message)

	payload := getPayload(record)
	if len(payload) > 0 {
		// using stdlib marshal and unmarshal to normalize the object before passing it to colorjson
		// reason: colorjson does not marshal all values from the payload object
		var jsonObj interface{}
		data, err := json.Marshal(payload)
		if err != nil {
			panic(fmt.Errorf("failed to marshal payload: %s\n", err.Error()))
		}
		err = json.Unmarshal(data, &jsonObj)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal payload: %s\n", err.Error()))
		}
		prettyData, err := colorjson.Marshal(jsonObj)
		if err != nil {
			panic(fmt.Errorf("failed to prettify json: %s\n", err.Error()))
		}
		prettyStringData := string(prettyData)
		logLine = fmt.Sprintf("%s %s", logLine, prettyStringData)
	}

	fmt.Printf("%s\n", logLine)

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
	clonedRecord := record.Clone()
	clonedRecord.Attrs(func(a slog.Attr) bool {
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
