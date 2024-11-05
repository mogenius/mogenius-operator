package assert

import (
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
func Assert(condition bool, message any) {
	if !condition {
		logger.Println("ASSERTION FAILED: ", message)
		panic("ASSERTION FAILED")
	}
}
