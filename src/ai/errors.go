package ai

// BudgetExhaustedError is returned when a run hits its per-run tool-call or
// token budget. Retrying won't help until the user raises the limit in the
// model or agent settings.
type BudgetExhaustedError struct {
	Msg string
}

func (e *BudgetExhaustedError) Error() string { return e.Msg }

// CompactionError is returned when the conversation history grew too large and
// the compaction call itself failed. Retrying the run is unlikely to help since
// the context size won't change between attempts.
type CompactionError struct {
	Cause error
}

func (e *CompactionError) Error() string { return "conversation compaction failed: " + e.Cause.Error() }
func (e *CompactionError) Unwrap() error { return e.Cause }
