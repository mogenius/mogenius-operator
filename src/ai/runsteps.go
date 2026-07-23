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
	maxRunSteps       = 200
	maxStepLabelLen   = 600
	maxStepArgsLen    = 600
	maxStepResultLen  = 1000
	stepLimitExceeded = "step limit reached — further steps of this run are not recorded"
)

type AiRunStepKind string

const (
	AI_RUN_STEP_REASON   AiRunStepKind = "reason"   // assistant free text between tool calls
	AI_RUN_STEP_ACT      AiRunStepKind = "act"      // one tool call, result attached
	AI_RUN_STEP_FINDINGS AiRunStepKind = "findings" // submit_analysis accepted findings
	AI_RUN_STEP_ERROR    AiRunStepKind = "error"    // the run ended with an error
)

// AiRunStep is one recorded step of an agent run's ReAct loop. Args and
// Result are truncated excerpts for the timeline — the audit log keeps the
// authoritative trail.
type AiRunStep struct {
	Seq       int           `json:"seq"`
	Kind      AiRunStepKind `json:"kind"`
	Label     string        `json:"label"`
	Tool      string        `json:"tool,omitempty"`
	Args      string        `json:"args,omitempty"`
	Result    string        `json:"result,omitempty"`
	Timestamp int64         `json:"timestamp"`
}

// StepRecorder records one step of an agent run; a nil recorder disables
// recording (chat and tests). Seq, Timestamp and truncation are applied by
// the recorder, callers only fill Kind/Label/Tool/Args/Result.
type StepRecorder func(step AiRunStep)

// AiRun is the assembled view of one agent run: metadata from the primary
// task (whose ID is the run id) plus the recorded steps and the IDs of all
// finding tasks spawned by the run. Tasks stay the single source of truth —
// nothing here is stored twice.
type AiRun struct {
	ID              string        `json:"id"`
	AgentRef        string        `json:"agentRef,omitempty"`
	Trigger         string        `json:"trigger,omitempty"`
	TriggeredByUser *structs.User `json:"triggeredByUser,omitempty"`
	Model           string        `json:"model"`
	State           AiTaskState   `json:"state"`
	TokensUsed      int64         `json:"tokensUsed"`
	TimeUsedInMs    int           `json:"timeUsedInMs"`
	CreatedAt       int64         `json:"createdAt"`
	UpdatedAt       int64         `json:"updatedAt"`
	Error           string        `json:"error,omitempty"`
	CurrentActivity string        `json:"currentActivity,omitempty"`
	Steps           []AiRunStep   `json:"steps"`
	TaskIDs         []string      `json:"taskIds"`
}

func runStepsKey(runID string) string {
	return DB_AI_BUCKET_RUN_STEPS + ":" + runID
}

func truncateStepText(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max] + "…"
}

// newStepRecorder returns a recorder that appends steps of one run to Valkey
// (same 7-day TTL as the tasks). The whole list is rewritten per step — runs
// are budget-capped, so the list stays small and a crash loses at most the
// final append.
func (ai *aiManager) newStepRecorder(runID string) StepRecorder {
	steps := make([]AiRunStep, 0, 16)
	var mu sync.Mutex
	return func(step AiRunStep) {
		mu.Lock()
		defer mu.Unlock()
		if len(steps) >= maxRunSteps {
			return
		}
		if len(steps) == maxRunSteps-1 {
			step = AiRunStep{Kind: AI_RUN_STEP_ERROR, Label: stepLimitExceeded}
		}
		step.Seq = len(steps) + 1
		step.Timestamp = time.Now().UnixMilli()
		step.Label = truncateStepText(step.Label, maxStepLabelLen)
		step.Args = truncateStepText(step.Args, maxStepArgsLen)
		step.Result = truncateStepText(step.Result, maxStepResultLen)
		steps = append(steps, step)

		payload, err := json.Marshal(steps)
		if err != nil {
			ai.logger.Warn("Failed to marshal AI run steps", "runID", runID, "error", err)
			return
		}
		if err := ai.valkeyClient.Set(string(payload), ValkeyAiTTL, runStepsKey(runID)); err != nil {
			ai.logger.Warn("Failed to persist AI run steps", "runID", runID, "error", err)
		}
	}
}

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
	}, nil
}
