package assert

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"testing"
)

var logger *log.Logger = log.Default()

func init() {
	logger.SetOutput(os.Stderr)
}

// In some cases it is better to stop the World.
//
// Examples:
//
//   - Init functions haven't been called.
//   - Configuration values are invalid.
//   - `nil` values where they should never happen. For example injected functions.
func Assert(condition bool, messages ...any) {
	if !condition {
		logger.Println("== ASSERTION FAILURE ==")

		if len(messages) > 0 {
			logger.Println("Messages:")
			for _, message := range messages {
				logger.Printf("  -> %v\n", tryStringify(message))
			}
		}

		stack := make([]uintptr, 5)
		length := runtime.Callers(2, stack)
		frames := runtime.CallersFrames(stack[:length])
		logger.Println()
		logger.Println("Location:")
		for {
			frame, more := frames.Next()
			logger.Printf("%s\n", frame.Function)
			logger.Printf("    %s:%d\n", frame.File, frame.Line)
			if !more {
				break
			}
			if frame.Function == "main.main" {
				break
			}
		}

		os.Exit(1)
	}
}

func AssertT(t *testing.T, condition bool, messages ...any) {
	if !condition {
		t.Log("== ASSERTION FAILURE ==")

		if len(messages) > 0 {
			t.Log("Messages:")
			for _, message := range messages {
				t.Logf("  -> %v\n", tryStringify(message))
			}
		}

		stack := make([]uintptr, 5)
		length := runtime.Callers(2, stack)
		frames := runtime.CallersFrames(stack[:length])
		t.Log()
		t.Log("Location:")
		for {
			frame, more := frames.Next()
			if frame.Function == "testing.tRunner" {
				break
			}
			t.Log(frame.Function)
			t.Logf("    %s:%d", frame.File, frame.Line)
			if !more {
				break
			}
		}

		t.FailNow()
	}
}

func tryStringify(data any) any {
	err, ok := data.(error)
	if ok {
		return err
	}
	stringer, ok := data.(fmt.Stringer)
	if ok {
		return stringer.String()
	}
	return data
}
