package assert

import (
	"fmt"
	"log"
)

var logger *log.Logger = log.Default()

// In some cases it is better to stop the World.
//
// Examples:
//
//   - Init functions haven't been called.
//   - Configuration values are invalid.
//   - `nil` values where they should never happen. For example injected functions.
func Assert(condition bool, message ...any) {
	if !condition {
		for _, msg := range message {
			logger.Println("ASSERTION FAILED: ", tryStringify(msg))
		}
		panic("ASSERTION FAILED")
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
