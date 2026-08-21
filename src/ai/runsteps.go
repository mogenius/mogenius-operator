package ai

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"mogenius-operator/src/structs"
)

const DB_AI_BUCKET_RUN_STEPS = "ai_run_steps"

// Step budget per run: an agent run is capped at maxToolCalls (default 50)
// tool calls, so 200 steps only trips on pathological reason/act interleaving.
const (
	maxRunSteps = 200
	// The label carries the model's reasoning — the readable core of the
	// timeline, not a "truncated excerpt" like args/result. Keep a generous cap
	// so normal reasoning is shown in full while still guarding against
	// pathological output.
	maxStepLabelLen   = 8000
	maxStepArgsLen    = 600
	stepLimitExceeded = "step limit reached — further steps of this run are not recorded"
)

type AiRunStepKind string

const (
	AI_RUN_STEP_REASON     AiRunStepKind = "reason"     // assistant free text between tool calls
	AI_RUN_STEP_ACT        AiRunStepKind = "act"        // one tool call, result attached
	AI_RUN_STEP_COMPACTION AiRunStepKind = "compaction" // a compaction step (e.g. failed compaction)
)

type AiRunStepStatus string

const (
	AiRunStepStatusRunning  AiRunStepStatus = "running"
	AiRunStepStatusFinished AiRunStepStatus = "finished"
	AiRunStepStatusErrored  AiRunStepStatus = "errored"
)

// StepFinalizer closes a step with its terminal status and optional result text.
type StepFinalizer func(status AiRunStepStatus, label string, result string)

// AiRunStep is one recorded step of an agent run's ReAct loop. Args and
// Result are truncated excerpts for the timeline — the audit log keeps the
// authoritative trail.
type AiRunStep struct {
	Seq       int             `json:"seq"`
	Kind      AiRunStepKind   `json:"kind"`
	Status    AiRunStepStatus `json:"status,omitempty"`
	Label     string          `json:"label"`
	Tool      string          `json:"tool,omitempty"`
	Args      string          `json:"args,omitempty"`
	Result    string          `json:"result,omitempty"`
	Timestamp int64           `json:"timestamp"`
}

// StepRecorder records a step as "running" and returns a StepFinalizer that
// sets the terminal status and optional result. A nil recorder is valid (chat /
// tests) — the returned finalizer is always safe to call.
type StepRecorder interface {
	ToolCall(tool string, args string) StepFinalizer
	Reason(label string) StepFinalizer
	Compaction(label string) StepFinalizer
	// Error writes a step directly as "errored" in a single persist — use when
	// there is no preceding "running" phase (e.g. task-level failures).
	Error(label string)
}

// AiRun is the assembled view of one agent run: metadata from the primary
// task (whose ID is the run id) plus the recorded steps and the IDs of all
// finding tasks spawned by the run. Tasks stay the single source of truth —
// nothing here is stored twice.
type AiRun struct {
	ID              string                `json:"id"`
	AgentRef        string                `json:"agentRef,omitempty"`
	Trigger         string                `json:"trigger,omitempty"`
	TriggeredByUser *structs.User         `json:"triggeredByUser,omitempty"`
	Model           string                `json:"model"`
	State           AiTaskState           `json:"state"`
	TokensUsed      int64                 `json:"tokensUsed"`
	TimeUsedInMs    int                   `json:"timeUsedInMs"`
	CreatedAt       int64                 `json:"createdAt"`
	UpdatedAt       int64                 `json:"updatedAt"`
	Error           string                `json:"error,omitempty"`
	CurrentActivity string                `json:"currentActivity,omitempty"`
	Steps           []AiRunStep           `json:"steps"`
	TaskIDs         []string              `json:"taskIds"`
	ToolApprovals   []ToolApprovalRequest `json:"toolApprovals,omitempty"`
}

type ToolApprovalRequest struct {
	ToolName string         `json:"toolName"`
	Args     map[string]any `json:"args"`
}

func runStepsKey(runID string) string {
	return DB_AI_BUCKET_RUN_STEPS + ":" + runID
}

// truncateStepText caps value at max runes and appends an ellipsis. max counts
// runes (not bytes) so multibyte UTF-8 is never cut mid-character.
func truncateStepText(value string, max int) string {
	if max <= 0 {
		return value
	}

	// Fast path: byte length is an upper bound on rune count, so if it fits in
	// bytes it fits in runes.
	if len(value) <= max {
		return value
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max]) + "…"
}

// aiStepRecorder implements StepRecorder backed by Valkey. The whole step list
// is rewritten per append — runs are budget-capped, so the list stays small
// and a crash loses at most the final write.
//
// Every step is persisted twice: first as "running" when recorded, then again
// with a terminal status ("finished" or "errored") and optional result when the
// returned StepFinalizer is called.
//
// steps holds pointers so that each StepFinalizer closure captures a direct
// reference to its step — the pointer remains valid even after r.steps is
// reallocated by append, and correctly targets the original step regardless of
// how many nested steps are recorded before the finalizer is called.
type aiStepRecorder struct {
	ai    *aiManager
	runID string
	steps []*AiRunStep
	mu    sync.Mutex
}

func (r *aiStepRecorder) persist() {
	payload, err := json.Marshal(r.steps)
	if err != nil {
		r.ai.logger.Warn("Failed to marshal AI run steps", "runID", r.runID, "error", err)
		return
	}
	if err := r.ai.valkeyClient.Set(string(payload), ValkeyAiTTL, runStepsKey(r.runID)); err != nil {
		r.ai.logger.Warn("Failed to persist AI run steps", "runID", r.runID, "error", err)
	}
}

func (r *aiStepRecorder) record(kind AiRunStepKind, label, tool, args string) StepFinalizer {
	noop := StepFinalizer(func(AiRunStepStatus, string, string) {})

	r.mu.Lock()

	if len(r.steps) >= maxRunSteps {
		r.mu.Unlock()
		return noop
	}
	if len(r.steps) == maxRunSteps-1 {
		// Write the sentinel directly as errored — it has no running phase.
		r.steps = append(r.steps, &AiRunStep{
			Kind:      AI_RUN_STEP_COMPACTION,
			Label:     stepLimitExceeded,
			Status:    AiRunStepStatusErrored,
			Seq:       len(r.steps) + 1,
			Timestamp: time.Now().UnixMilli(),
		})
		r.persist()
		r.mu.Unlock()
		return noop
	}

	maxStepResultLen, err := r.ai.config.TryGetInt("MO_AI_RESPONSE_MAX_LENGTH")
	if err != nil {
		maxStepResultLen = 1000
	}
	maxLen := int(maxStepResultLen)

	step := &AiRunStep{
		Kind:      kind,
		Label:     truncateStepText(label, maxStepLabelLen),
		Tool:      tool,
		Args:      truncateStepText(args, maxStepArgsLen),
		Status:    AiRunStepStatusRunning,
		Seq:       len(r.steps) + 1,
		Timestamp: time.Now().UnixMilli(),
	}
	r.steps = append(r.steps, step)
	r.persist()
	r.mu.Unlock()

	return func(status AiRunStepStatus, label string, result string) {
		r.mu.Lock()
		defer r.mu.Unlock()
		step.Status = status
		if label != "" {
			step.Label = truncateStepText(label, maxStepLabelLen)
		}
		step.Result = truncateStepText(result, maxLen)
		r.persist()
	}
}

func (r *aiStepRecorder) ToolCall(tool, args string) StepFinalizer {
	return r.record(AI_RUN_STEP_ACT, tool, tool, args)
}

func (r *aiStepRecorder) Reason(label string) StepFinalizer {
	return r.record(AI_RUN_STEP_REASON, label, "", "")
}

func (r *aiStepRecorder) Compaction(label string) StepFinalizer {
	return r.record(AI_RUN_STEP_COMPACTION, label, "", "")
}

func (r *aiStepRecorder) Error(label string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.steps) >= maxRunSteps {
		return
	}
	if len(r.steps) == maxRunSteps-1 {
		label = stepLimitExceeded
	}
	r.steps = append(r.steps, &AiRunStep{
		Kind:      AI_RUN_STEP_REASON,
		Label:     truncateStepText(label, maxStepLabelLen),
		Status:    AiRunStepStatusErrored,
		Seq:       len(r.steps) + 1,
		Timestamp: time.Now().UnixMilli(),
	})
	r.persist()
}

func (ai *aiManager) newStepRecorder(runID string) StepRecorder {
	return &aiStepRecorder{
		ai:    ai,
		runID: runID,
		steps: make([]*AiRunStep, 0, 16),
	}
}

// noopStepRecorder is a StepRecorder that discards all steps. Use it when
// recording is disabled (e.g. tests, chat mode).
type noopStepRecorder struct{}

func (noopStepRecorder) ToolCall(string, string) StepFinalizer {
	return func(AiRunStepStatus, string, string) {}
}
func (noopStepRecorder) Reason(string) StepFinalizer { return func(AiRunStepStatus, string, string) {} }
func (noopStepRecorder) Compaction(string) StepFinalizer {
	return func(AiRunStepStatus, string, string) {}
}
func (noopStepRecorder) Error(string) {}

// NoopStepRecorder returns a StepRecorder that silently discards all steps.
func NoopStepRecorder() StepRecorder { return noopStepRecorder{} }

func (ai *aiManager) getRunSteps(runID string) []AiRunStep {
	item, err := ai.valkeyClient.Get(runStepsKey(runID))
	if err != nil || item == "" {
		return []AiRunStep{}
	}
	var steps []AiRunStep
	if err := json.Unmarshal([]byte(item), &steps); err != nil {
		ai.logger.Warn("Failed to unmarshal AI run steps", "runID", runID, "error", err)
		return []AiRunStep{}
	}
	return steps
}

// GetRun assembles the run view for a run id (the primary task's ID): task
// metadata, recorded steps and the IDs of every finding task of the run.
func (ai *aiManager) GetRun(runID string) (*AiRun, error) {
	primary, err := ai.getTaskByKey(runID)
	if err != nil {
		return nil, err
	}
	if primary == nil {
		return nil, fmt.Errorf("no ai run with the specified id has been found: %s", runID)
	}

	approvals := &[]ToolApprovalRequest{}
	if primary.Response != nil && primary.Response.ToolRequests != nil {
		for _, toolRequest := range primary.Response.ToolRequests {
			*approvals = append(*approvals, ToolApprovalRequest{
				ToolName: toolRequest.Name,
				Args:     toolRequest.Args,
			})
		}
	}

	taskIDs := []string{primary.ID}
	if all, err := ai.GetAllAiTasks(); err == nil {
		for _, task := range all {
			if task.RunID == runID && task.ID != primary.ID {
				taskIDs = append(taskIDs, task.ID)
			}
		}
	} else {
		ai.logger.Warn("Failed to list tasks while assembling AI run", "runID", runID, "error", err)
	}

	return &AiRun{
		ID:              primary.ID,
		AgentRef:        primary.AgentRef,
		Trigger:         primary.Trigger,
		TriggeredByUser: primary.TriggeredByUser,
		Model:           primary.Model,
		State:           primary.State,
		TokensUsed:      primary.TokensUsed,
		TimeUsedInMs:    primary.TimeUsedInMs,
		CreatedAt:       primary.CreatedAt,
		UpdatedAt:       primary.UpdatedAt,
		Error:           primary.Error,
		CurrentActivity: primary.CurrentActivity,
		Steps:           ai.getRunSteps(runID),
		TaskIDs:         taskIDs,
		ToolApprovals:   *approvals,
	}, nil
}
