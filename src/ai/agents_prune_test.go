package ai

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"testing"

	"mogenius-operator/src/valkeyclient"
	"mogenius-operator/src/websocket"

	"github.com/stretchr/testify/assert"
)

// fakePruneValkey implements only the ValkeyClient methods run pruning
// touches; everything else panics via the embedded nil interface.
type fakePruneValkey struct {
	valkeyclient.ValkeyClient
	store map[string]string
}

func (f *fakePruneValkey) Keys(pattern string) ([]string, error) {
	var keys []string
	for key := range f.store {
		if ok, _ := path.Match(pattern, key); ok {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

func (f *fakePruneValkey) Get(keys ...string) (string, error) {
	return f.store[strings.Join(keys, ":")], nil
}

func (f *fakePruneValkey) DeleteSingle(key ...string) error {
	delete(f.store, strings.Join(key, ":"))
	return nil
}

// fakePruneEventClient swallows the delete events pruning emits.
type fakePruneEventClient struct {
	websocket.WebsocketClient
}

func (f *fakePruneEventClient) WriteJSON(any) error { return nil }

func newPruneTestManager(t *testing.T) (*aiManager, *fakePruneValkey) {
	t.Helper()
	fake := &fakePruneValkey{store: map[string]string{}}
	return &aiManager{
		logger:       slog.New(slog.DiscardHandler),
		valkeyClient: fake,
		eventClient:  &fakePruneEventClient{},
	}, fake
}

func (f *fakePruneValkey) putTask(t *testing.T, task AiTask) {
	t.Helper()
	payload, err := json.Marshal(task)
	assert.NoError(t, err)
	f.store[task.ID] = string(payload)
}

func TestGroupAgentRunKeys(t *testing.T) {
	keys := []string{
		"ai_tasks:Agent:default:cleaner-run-200",
		"ai_tasks:Agent:other:cleaner-run-200-f2", // legacy finding, re-homed namespace
		"ai_tasks:Agent:default:cleaner-run-100",
		"ai_tasks:Agent:default:cleaner-run-300",
		"ai_tasks:Pod:default:some-event-task", // no -run-<ts> segment
	}

	groups := groupAgentRunKeys(keys)

	assert.Len(t, groups, 3)
	assert.Equal(t, int64(300), groups[0].timestamp)
	assert.Equal(t, int64(200), groups[1].timestamp)
	assert.Equal(t, int64(100), groups[2].timestamp)
	assert.ElementsMatch(t, []string{
		"ai_tasks:Agent:default:cleaner-run-200",
		"ai_tasks:Agent:other:cleaner-run-200-f2",
	}, groups[1].keys)
}

func TestGroupAgentRunKeysAnchorsTimestampAtKeyEnd(t *testing.T) {
	// An agent legally named "x-run-2" must group by the trailing run
	// timestamp, not the "-run-2" inside its name.
	groups := groupAgentRunKeys([]string{"ai_tasks:Agent:default:x-run-2-run-300"})

	assert.Len(t, groups, 1)
	assert.Equal(t, int64(300), groups[0].timestamp)
}

func TestPruneAgentRunsToLimitKeepsNewestAndOpenRuns(t *testing.T) {
	ai, fake := newPruneTestManager(t)

	// 13 completed runs, ts 1000..1012. Beyond the cap of 10: 1000-1002.
	for ts := 1000; ts <= 1012; ts++ {
		fake.putTask(t, AiTask{
			ID:       fmt.Sprintf("ai_tasks:Agent:default:cleaner-run-%d", ts),
			State:    AI_TASK_STATE_COMPLETED,
			AgentRef: "cleaner",
		})
	}
	// Run 1001 has a legacy finding task still awaiting a decision — the
	// whole run must survive even though it is beyond the cap.
	fake.putTask(t, AiTask{
		ID:       "ai_tasks:Agent:other:cleaner-run-1001-f2",
		State:    AI_TASK_STATE_PROPOSED,
		AgentRef: "cleaner",
	})
	// The pruned run 1002 owns a step timeline that must go with it.
	fake.store[runStepsKey("ai_tasks:Agent:default:cleaner-run-1002")] = "[]"
	// Another agent's ancient run is out of scope.
	fake.putTask(t, AiTask{
		ID:       "ai_tasks:Agent:default:other-agent-run-1",
		State:    AI_TASK_STATE_COMPLETED,
		AgentRef: "other-agent",
	})

	ai.pruneAgentRunsToLimit("cleaner")

	assert.NotContains(t, fake.store, "ai_tasks:Agent:default:cleaner-run-1000")
	assert.NotContains(t, fake.store, "ai_tasks:Agent:default:cleaner-run-1002")
	assert.NotContains(t, fake.store, runStepsKey("ai_tasks:Agent:default:cleaner-run-1002"))
	assert.Contains(t, fake.store, "ai_tasks:Agent:default:cleaner-run-1001")
	assert.Contains(t, fake.store, "ai_tasks:Agent:other:cleaner-run-1001-f2")
	for ts := 1003; ts <= 1012; ts++ {
		assert.Contains(t, fake.store, fmt.Sprintf("ai_tasks:Agent:default:cleaner-run-%d", ts))
	}
	assert.Contains(t, fake.store, "ai_tasks:Agent:default:other-agent-run-1")
}

func TestPruneAgentRunsToLimitLeavesRunsWithinCapAlone(t *testing.T) {
	ai, fake := newPruneTestManager(t)

	for ts := 1000; ts < 1000+maxAgentRunHistory; ts++ {
		fake.putTask(t, AiTask{
			ID:       fmt.Sprintf("ai_tasks:Agent:default:cleaner-run-%d", ts),
			State:    AI_TASK_STATE_COMPLETED,
			AgentRef: "cleaner",
		})
	}

	ai.pruneAgentRunsToLimit("cleaner")

	assert.Len(t, fake.store, maxAgentRunHistory)
}

func TestPruneAgentRunsToLimitEmptyAgentIsNoop(t *testing.T) {
	// An empty agent name must return before touching Valkey — the nil
	// client in a bare aiManager would panic otherwise.
	ai := &aiManager{logger: slog.New(slog.DiscardHandler)}
	ai.pruneAgentRunsToLimit("")
}
