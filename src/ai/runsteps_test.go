package ai

import (
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"mogenius-operator/src/valkeyclient"

	"github.com/stretchr/testify/assert"
)

// fakeStepValkey implements only the ValkeyClient methods the step store
// touches; everything else panics via the embedded nil interface.
type fakeStepValkey struct {
	valkeyclient.ValkeyClient
	store map[string]string
}

func newFakeStepValkey() *fakeStepValkey {
	return &fakeStepValkey{store: map[string]string{}}
}

func (f *fakeStepValkey) Set(value string, _ time.Duration, keys ...string) error {
	f.store[strings.Join(keys, ":")] = value
	return nil
}

func (f *fakeStepValkey) Get(keys ...string) (string, error) {
	return f.store[strings.Join(keys, ":")], nil
}

func (f *fakeStepValkey) List(limit int, keys ...string) ([]string, error) {
	prefix := strings.TrimSuffix(strings.Join(keys, ":"), "*:*:*") // ai_tasks:
	var items []string
	for key, value := range f.store {
		if strings.HasPrefix(key, prefix) {
			items = append(items, value)
		}
	}
	return items, nil
}

func (f *fakeStepValkey) DeleteSingle(key ...string) error {
	delete(f.store, strings.Join(key, ":"))
	return nil
}

func newStepTestManager(t *testing.T) (*aiManager, *fakeStepValkey) {
	t.Helper()
	fake := newFakeStepValkey()
	return &aiManager{
		logger:       slog.New(slog.DiscardHandler),
		valkeyClient: fake,
	}, fake
}

func TestStepRecorderSequencingAndTruncation(t *testing.T) {
	ai, _ := newStepTestManager(t)

	record := ai.newStepRecorder("run-1")
	record(AiRunStep{Kind: AI_RUN_STEP_REASON, Label: strings.Repeat("r", maxStepLabelLen+50)})
	record(AiRunStep{Kind: AI_RUN_STEP_ACT, Label: "list pods", Tool: "list_kubernetes_resources", Args: strings.Repeat("a", maxStepArgsLen+50), Result: strings.Repeat("x", maxStepResultLen+50)})

	steps := ai.getRunSteps("run-1")
	assert.Len(t, steps, 2)
	assert.Equal(t, 1, steps[0].Seq)
	assert.Equal(t, 2, steps[1].Seq)
	assert.Equal(t, AI_RUN_STEP_REASON, steps[0].Kind)
	assert.Equal(t, AI_RUN_STEP_ACT, steps[1].Kind)
	assert.Len(t, steps[0].Label, maxStepLabelLen+len("…"))
	assert.Len(t, steps[1].Args, maxStepArgsLen+len("…"))
	assert.Len(t, steps[1].Result, maxStepResultLen+len("…"))
	assert.NotZero(t, steps[0].Timestamp)
}

func TestStepRecorderCapsRunaways(t *testing.T) {
	ai, _ := newStepTestManager(t)

	record := ai.newStepRecorder("run-cap")
	for i := 0; i < maxRunSteps+25; i++ {
		record(AiRunStep{Kind: AI_RUN_STEP_ACT, Label: "step"})
	}

	steps := ai.getRunSteps("run-cap")
	assert.Len(t, steps, maxRunSteps)
	last := steps[len(steps)-1]
	assert.Equal(t, AI_RUN_STEP_ERROR, last.Kind)
	assert.Equal(t, stepLimitExceeded, last.Label)
}

func TestGetRunStepsMissingRunIsEmpty(t *testing.T) {
	ai, _ := newStepTestManager(t)

	steps := ai.getRunSteps("does-not-exist")
	assert.NotNil(t, steps)
	assert.Empty(t, steps)
}

func TestGetRunAssemblesPrimaryTaskAndFindings(t *testing.T) {
	ai, fake := newStepTestManager(t)

	primaryID := "ai_tasks:Agent:mogenius:cleaner-run-1700000000"
	primary := AiTask{
		ID:         primaryID,
		State:      AI_TASK_STATE_PROPOSED,
		AgentRef:   "cleaner",
		Trigger:    "cron",
		Model:      "claude-sonnet-5",
		TokensUsed: 1234,
		RunID:      primaryID,
		CreatedAt:  1700000000,
	}
	finding := AiTask{ID: primaryID + "-f2", RunID: primaryID, State: AI_TASK_STATE_PROPOSED}
	unrelated := AiTask{ID: "ai_tasks:Agent:mogenius:other-run-5", RunID: "ai_tasks:Agent:mogenius:other-run-5"}
	for _, task := range []AiTask{primary, finding, unrelated} {
		payload, err := json.Marshal(task)
		assert.NoError(t, err)
		fake.store[task.ID] = string(payload)
	}

	record := ai.newStepRecorder(primaryID)
	record(AiRunStep{Kind: AI_RUN_STEP_ACT, Label: "list jobs", Tool: "list_kubernetes_resources"})

	run, err := ai.GetRun(primaryID)
	assert.NoError(t, err)
	assert.Equal(t, primaryID, run.ID)
	assert.Equal(t, "cleaner", run.AgentRef)
	assert.Equal(t, "cron", run.Trigger)
	assert.Equal(t, int64(1234), run.TokensUsed)
	assert.Len(t, run.Steps, 1)
	assert.ElementsMatch(t, []string{primaryID, primaryID + "-f2"}, run.TaskIDs)
}

func TestGetRunUnknownIdFails(t *testing.T) {
	ai, _ := newStepTestManager(t)

	_, err := ai.GetRun("nope")
	assert.ErrorContains(t, err, "no ai run")
}
