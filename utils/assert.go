package utils

// In some cases it is better to stop the World.
//
// Examples:
//
//   - Init functions haven't been called.
//   - Configuration values are invalid.
//   - `nil` values where they should never happen. For example injected functions.
func Assert(condition bool, message any) {
	if !condition {
		panic(message)
	}
}
