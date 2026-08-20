package ai

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/metrics"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"mogenius-operator/src/websocket"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"encoding/json"

	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/valkey-io/valkey-go"

	"github.com/anthropics/anthropic-sdk-go"
	anthropic_option "github.com/anthropics/anthropic-sdk-go/option"

	ollama "github.com/ollama/ollama/api"
)

const (
	DB_AI_BUCKET_TASKS              = "ai_tasks"
	DB_AI_BUCKET_TASKS_LATEST       = "ai_tasks_latest"
	DB_AI_BUCKET_TOKENS             = "ai_tokens"
	DB_AI_LATEST_TASK_KEY           = "latest-task"
	DB_AI_LATEST_NAMESPACE_TASK_KEY = "latest-namespace-task"
)

// valkeyBatchSize caps how many GETs are pipelined per round trip when reading
// a scan result.
const valkeyBatchSize = 500

var ValkeyAiTTL = time.Hour * 24 * 7 // 7 days

// approvalTimeout is the maximum time an agent run will wait for a human to
// approve or reject a proposed tool call before the run is abandoned.
const approvalTimeout = 7 * 24 * time.Hour

// maxConcurrentRuns caps how many agent runs execute in parallel on this
// replica. Runs blocked on approval hold a slot but don't stall the queue.
const maxConcurrentRuns = 10

type AiTaskState string
type AiTask struct {
	ID         string                       `json:"id"`
	Prompt     string                       `json:"prompt"`
	Response   *AiResponse                  `json:"response"`
	State      AiTaskState                  `json:"state"`
	Controller *utils.WorkloadSingleRequest `json:"controller,omitempty"`
	TokensUsed int64                        `json:"tokensUsed"`
	// Retries counts failed processing attempts; tasks at maxAiTaskRetries are
	// ignored instead of re-running the whole analysis loop again.
	Retries int `json:"retries,omitempty"`
	// CurrentActivity is the live "what is the agent doing right now" line for
	// the UI (tool being called with its key arguments); empty when idle.
	CurrentActivity string `json:"currentActivity,omitempty"`
	// RunID groups all finding tasks of one multi-finding run (the primary
	// task's ID); the UI renders such tasks as a single report.
	RunID               string                      `json:"runId,omitempty"`
	Model               string                      `json:"model"`
	TimeUsedInMs        int                         `json:"timeUsedInMs"`
	CreatedAt           int64                       `json:"createdAt"`
	UpdatedAt           int64                       `json:"updatedAt"`
	ReferencingResource utils.WorkloadSingleRequest `json:"referencingResource"` // the resource that triggered this task (empty for whole-scope runs)
	TriggeredBy         AiFilter                    `json:"triggeredBy"`         // legacy: the AI-Insights event filter that matched; kept for tasks persisted before the Agents rewrite
	ReadByUsers         []ReadBy                    `json:"readByUsers"`
	Error               string                      `json:"error"`

	AgentRef        string        `json:"agentRef,omitempty"`        // name of the Agent CR this task belongs to
	Trigger         string        `json:"trigger,omitempty"`         // "event", "cron" or "manual"
	TriggeredByUser *structs.User `json:"triggeredByUser,omitempty"` // set for manual triggers

	// ScopeNamespaces snapshots the agent's resolved scope at enqueue time.
	// Workspace visibility of whole-scope runs is decided from this data, not
	// from the task's storage key (see agentTaskVisibleInNamespaces).
	ScopeNamespaces []string `json:"scopeNamespaces,omitempty"`
	// ScopeAllNamespaces marks a wildcard ("*") scope: the run is visible to
	// every workspace, including ones whose namespaces did not exist yet when
	// the run was enqueued.
	ScopeAllNamespaces bool `json:"scopeAllNamespaces,omitempty"`

	// BaseResourceVersion is the target resource's resourceVersion at proposal
	// time; approval refuses to execute when the resource changed since.
	BaseResourceVersion string          `json:"baseResourceVersion,omitempty"`
	Approval            *ApprovalRecord `json:"approval,omitempty"`
	ExecutionResult     string          `json:"executionResult,omitempty"`
}

// ApprovalRecord attributes an approve/reject decision to a user.
type ApprovalRecord struct {
	User     structs.User `json:"user"`
	At       time.Time    `json:"at"`
	Rejected bool         `json:"rejected,omitempty"`
	Reason   string       `json:"reason,omitempty"`
}

type AiTaskLatest struct {
	Task   *AiTask         `json:"task,omitempty"`
	Status AiManagerStatus `json:"status"`
}

type ReadBy struct {
	User   structs.User `json:"user"`
	ReadAt time.Time    `json:"readAt"`
}

// state enums
const (
	AI_TASK_STATE_PENDING     AiTaskState = "pending"
	AI_TASK_STATE_IN_PROGRESS AiTaskState = "in-progress"
	AI_TASK_STATE_COMPLETED   AiTaskState = "completed"
	AI_TASK_STATE_FAILED      AiTaskState = "failed"
	AI_TASK_STATE_IGNORED     AiTaskState = "ignored"
	AI_TASK_STATE_SOLVED      AiTaskState = "solved"

	// cancel lifecycle: a user abort flips an in-progress run to "cancelling"
	// the moment the request arrives (immediate UI feedback) and to the
	// terminal "canceled" once the processing loop has unwound.
	AI_TASK_STATE_CANCELLING AiTaskState = "cancelling"
	AI_TASK_STATE_CANCELED   AiTaskState = "canceled"

	// proposal lifecycle: an analysis with an actionable proposed operation
	// becomes "proposed" and waits for a user decision; approval executes the
	// operation with the approving user's permissions.
	AI_TASK_STATE_PROPOSED         AiTaskState = "proposed"
	AI_TASK_STATE_REJECTED         AiTaskState = "rejected"
	AI_TASK_STATE_EXECUTING        AiTaskState = "executing"
	AI_TASK_STATE_EXECUTED         AiTaskState = "executed"
	AI_TASK_STATE_EXECUTION_FAILED AiTaskState = "execution-failed"
)

// maxAiTaskRetries caps how often a failed task is re-attempted. Every retry
// re-runs the full analysis loop (the complete exploration, tens of thousands
// of tokens), so failures that survived the in-conversation repair turns are
// almost certainly systematic — give up instead of burning the token budget.
const maxAiTaskRetries = 2

// AiFilter is the legacy AI-Insights event filter. Filters no longer trigger
// anything — event triggers live on Agent CRs — but tasks persisted before the
// Agents rewrite still carry one in TriggeredBy and the UI renders its
// name/description, so the shape is kept as data only.
type AiFilter struct {
	Id          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Kind        string            `json:"kind"`
	Contains    map[string]string `json:"contains"` // {"Running": "status.phase"}, {"ImagePullBackOff": "status.phase.ContainerStatuses.state.waiting.reason"}
	Excludes    map[string]string `json:"excludes"` // {"Succeeded": "status.phase"}, {"Completed": "status.phase"}
	Prompt      string            `json:"prompt"`
	For         *time.Duration    `json:"for,omitempty"` // optional duration for which the condition should be met
	IsActive    bool              `json:"isActive"`
}

// AiPromptConfig carries the platform-injected system prompt for agent runs.
// The legacy filters/userFilters arrays still sent by older platform versions
// are ignored on unmarshal.
type AiPromptConfig struct {
	Id           string `json:"id"`
	Name         string `json:"name"`
	SystemPrompt string `json:"systemPrompt"`
}

type AiPrompts struct {
	ChatSystemPrompt   string `json:"chatSystemPrompt"`
	GithubSystemPrompt string `json:"githubSystemPrompt"`
}

type AiManagerStatus struct {
	SdkType                     AiSdkType `json:"sdkType"`
	TokenLimit                  int64     `json:"tokenLimit"`
	TokensUsed                  int64     `json:"tokensUsed"`
	Model                       string    `json:"model"`
	MaxToolCalls                int       `json:"maxToolCalls"`
	ApiUrl                      string    `json:"apiUrl"`
	IsAiPromptConfigInitialized bool      `json:"isAiPromptConfigInitialized"`
	IsAiModelConfigInitialized  bool      `json:"isAiModelConfigInitialized"`
	TodaysProcessedTasks        int       `json:"todaysProcessedTasks"`
	TotalDbEntries              int       `json:"totalDbEntries"`
	UnprocessedDbEntries        int       `json:"unprocessedDbEntries"`
	IgnoredDbEntries            int       `json:"ignoredDbEntries"`
	NumberOfUnreadTasks         int       `json:"numberOfUnreadTasks,omitempty"`
	Error                       string    `json:"error,omitempty"`
	Warning                     string    `json:"warning,omitempty"`
	NextTokenResetTime          string    `json:"nextTokenResetTime,omitempty"`

	// Models is the per-AiModel usage breakdown (daily budgets live on the
	// model CRs). TokensUsed/TokenLimit above stay populated for backward
	// compatibility: cluster-wide total and the default model's limit.
	Models []AiModelUsageInfo `json:"models,omitempty"`
}

// AiModelUsageInfo reports one AiModel's daily budget consumption.
type AiModelUsageInfo struct {
	Name            string `json:"name"`
	DisplayName     string `json:"displayName,omitempty"`
	Model           string `json:"model"`
	Default         bool   `json:"default,omitempty"`
	TokensUsedToday int64  `json:"tokensUsedToday"`
	// DailyTokenLimit is the effective daily budget; 0 means unlimited.
	DailyTokenLimit int64 `json:"dailyTokenLimit"`
	Exceeded        bool  `json:"exceeded,omitempty"`
}

// AiResponse carries the pending tool calls of a PROPOSED task — the change an
// agent wants to make and a user has to approve. It is nil for every other
// state: a run's own text output is not persisted (the timeline steps are).
type AiResponse struct {
	ToolRequests []ToolRequest `json:"toolRequests"`
}

type ToolRequest struct {
	Name     string         `json:"toolCallName,omitempty"`
	Args     map[string]any `json:"toolCallArgs,omitempty"`
	Sessions []string       `json:"toolCallMcpSessions,omitempty"`
}
type UsedToken struct {
	Timestamp    time.Time `json:"timestamp"`
	TokensUsed   int64     `json:"tokensUsed"`
	IsIgnored    bool      `json:"isIgnored"`
	Key          string    `json:"key"`
	Model        string    `json:"model"`
	TimeUsedInMs int       `json:"timeUsedInMs"`
	// ModelRef is the AiModel CR name the usage counts against (per-model
	// daily budgets). Entries written before per-model accounting have it
	// empty and count into the cluster total only.
	ModelRef string `json:"modelRef,omitempty"`
}

type ModelsRequest struct {
	Sdk    string  `json:"SDK,omitempty"`
	ApiKey *string `json:"API_KEY,omitempty"`
	ApiUrl string  `json:"API_URL,omitempty"`

	// Alternative to API_KEY: reference to a Secret in the operator namespace
	// holding the key — the AiModel UI only knows the reference, never the key
	// value, so model listing resolves it server-side. API_KEY wins when both
	// are set; the key name defaults to DefaultApiKeySecretKey.
	ApiKeySecretName string `json:"API_KEY_SECRET_NAME,omitempty"`
	ApiKeySecretKey  string `json:"API_KEY_SECRET_KEY,omitempty"`
}
type AiManager interface {
	ProcessObject(obj *unstructured.Unstructured, eventType string, resource utils.ResourceDescriptor) // eventType can be "add", "update", "delete"
	Run()
	UpdateTaskState(taskID string, newState AiTaskState) error
	UpdateTaskReadState(taskID string, user *structs.User) error
	GetAllAiTasks() ([]AiTask, error)
	GetAiTasksForWorkspace(workspace string) ([]AiTask, error)
	GetAiTasksForResource(resourceReq utils.WorkloadSingleRequest) ([]AiTask, error)
	GetLatestTask(workspace *string) (*AiTaskLatest, error)
	// GetRun assembles one agent run (metadata from the primary task, the
	// recorded ReAct steps and the IDs of all finding tasks of the run).
	GetRun(runID string) (*AiRun, error)
	InjectAiPromptConfig(prompt AiPromptConfig, aiPrompts *AiPrompts)
	GetStatus(workspace *string) AiManagerStatus
	// ResetTokenUsageForModel zeroes today's recorded token usage of one
	// AiModel (by CR name) and returns the number of tokens cleared. Called
	// by the AiModel reconciler when the reset-usage annotation changes.
	ResetTokenUsageForModel(modelCrName string) (int64, error)
	DeleteAllAiData() error
	GetAvailableModels(request *ModelsRequest) ([]string, error)
	TestAiModel(name string, spec *v1alpha1.AiModelSpec, apiKey string) (*AiModelTestResult, error)
	Chat(ctx context.Context, ch IOChatChannel) error

	ApproveTask(taskID string, user structs.User) (*AiTask, error)
	RejectTask(taskID string, user structs.User, reason string) (*AiTask, error)
	CancelTask(taskID string, user structs.User) (*AiTask, error)
	DeleteTask(taskID string, user structs.User) (*AiTask, error)
	TriggerAgent(agentName string, user *structs.User) (*AiTask, error)

	ResolveWorkspaceContext(userEmail string, workspaceName string) (*v1alpha1.WorkspaceSpec, *v1alpha1.GrantSpec)

	// NotifyMcpServerChanged reconnects the named McpServer CR's MCP server.
	// Called by the reconciler on create/update.
	NotifyMcpServerChanged(name string) error

	// NotifyMcpServerDeleted removes the session for the named McpServer CR. Called
	// by the reconciler on delete.
	NotifyMcpServerDeleted(name string)

	// RefreshAllMcpServerCRConnections (re)connects all McpServer CRs from the
	// operator namespace. Called once at startup.
	RefreshAllMcpServerCRConnections()

	// HasMcpSession reports whether a live session for the named server exists.
	// Used by the reconciler to choose between a full reconnect and a probe.
	HasMcpSession(name string) bool

	// ProbeMcpSession refreshes the tool list on an existing session via a
	// lightweight ListTools call without tearing down the connection.
	ProbeMcpSession(ctx context.Context, name string) error

	// GetMcpToolsWithPolicies returns every tool of the named McpServer together
	// with its effective execution policy. Used by the reconciler to populate
	// McpServerStatus.ToolsWithPolicies. Returns nil when no session exists.
	GetMcpToolsWithPolicies(serverName string) []v1alpha1.MCPToolWithPolicy
}

type SecretGetter func(namespace, name string) (*coreV1.Secret, error)

type aiManager struct {
	logger            *slog.Logger
	valkeyClient      valkeyclient.ValkeyClient
	config            cfg.ConfigModule
	promptConfigMu    sync.RWMutex // guards aiPromptConfig: injected via ConfigMap watch while the queue ticker reads it
	aiPromptConfig    *AiPromptConfig
	ownerCacheService store.OwnerCacheService
	eventClient       websocket.WebsocketClient
	secretGetter      SecretGetter
	stateMu           sync.Mutex // guards error+warning: written by the ticker goroutine, read by status requests
	error             string
	warning           string
	mcpManager        *mcpClientManager
	mcpConnectors     []MCPServerConnector

	// cron trigger state: last evaluation time per agent. In-memory only —
	// after a restart schedules re-anchor to the first ticker run.
	cronStateLock sync.Mutex
	lastCronRun   map[string]time.Time
	// lastAgentRun: when a run was last enqueued per agent (any trigger),
	// used as the change-trigger cooldown base. In-memory (leader only).
	lastAgentRun map[string]time.Time
	// isLeading gates cron evaluation to the leading replica; event-triggered
	// tasks are already deduplicated via their Valkey key.
	isLeading func() bool

	// taskQueueKick wakes the queue loop as soon as a task lands in pending
	// state, so new reports start immediately instead of waiting for the next
	// 1-minute tick. Buffered(1): kicks during a running pass coalesce into
	// exactly one follow-up pass.
	taskQueueKick chan struct{}

	// runCancels holds the context cancel func of every run executing on THIS
	// replica, keyed by task ID. A cancel request aborts the in-flight LLM
	// call immediately through it; the Valkey cancel marker stays as the
	// fallback that reaches runs on other replicas at the next turn boundary.
	runCancelMu sync.Mutex
	runCancels  map[string]context.CancelFunc

	// pendingApprovals holds one channel per PROPOSED task that was created
	// by an in-flight agent run. ApproveTask / RejectTask send the result on
	// the channel to resume the blocked provider goroutine.
	pendingApprovalsMu sync.Mutex
	pendingApprovals   map[string]chan approvalResult

	// runSem limits how many agent runs execute concurrently on this replica.
	// Runs waiting for approval hold a slot but do not block the queue loop.
	runSem chan struct{}

	// prompts
	chatPromptMu sync.RWMutex
	aiPrompts    AiPrompts

	// usage snapshot cache: one Valkey SCAN serves every budget check and
	// status build for usageSnapshotTTL instead of each caller scanning on
	// its own. Invalidated on bookings and resets.
	usageSnapshotMu   sync.Mutex
	usageSnapshot     *tokenUsageSnapshot
	usageSnapshotTime time.Time

	// enabled-agents cache: getEnabledAgents runs on every watch event in the
	// cluster (ProcessObject → triggerChangeAgents); without the cache each
	// event cost a full-keyspace Valkey SCAN (MOG-4518). Agent CR events
	// invalidate via invalidateAgentCache, agentCacheTTL bounds staleness.
	// agentCacheGen detects invalidations that race an in-flight fetch;
	// agentCacheFetching lets concurrent callers use the previous list
	// instead of blocking behind the store read.
	agentCacheMu        sync.Mutex
	cachedEnabledAgents []v1alpha1.Agent
	agentCacheTime      time.Time
	agentCacheValid     bool
	agentCacheFetching  bool
	agentCacheGen       uint64
}

// auditInsightToolCall writes a durable audit entry for every tool the
// unattended agent pipeline executes ("no unattributed actions"): unlike
// the chat path, this pipeline has no user whose audit trail would capture
// the call, so calls are attributed to the agent's synthetic user. The result
// is truncated — the entry documents WHAT was queried, not the full payload.
func (ai *aiManager) auditInsightToolCall(toolCtx *ToolContext, toolName string, args map[string]any, result string, toolErr error) {
	user := structs.User{FirstName: "AI", LastName: "Insights", Email: "ai-insights@system", Source: "ai-insights"}
	workspace := ""
	if toolCtx != nil {
		if toolCtx.User != nil {
			user = *toolCtx.User
		}
		workspace = toolCtx.Workspace
	}
	errStr := ""
	if toolErr != nil {
		errStr = toolErr.Error()
	}
	store.AddAiChatAuditLog(
		ai.logger,
		"ai/insight-tool",
		map[string]any{"tool": toolName, "args": args},
		truncateResult(result, 500),
		errStr,
		user,
		workspace,
	)
}

// setError/setWarning/statusStrings centralize access to the transient
// error/warning state; these fields were previously written from the ticker
// goroutine and read from concurrent status requests without a lock.
func (ai *aiManager) setError(msg string) {
	ai.stateMu.Lock()
	ai.error = msg
	ai.stateMu.Unlock()
}

func (ai *aiManager) setWarning(msg string) {
	ai.stateMu.Lock()
	ai.warning = msg
	ai.stateMu.Unlock()
}

// clearTokenLimitError resets the error only if it is the token-limit one, so
// unrelated errors are not wiped by a successful limit check.
func (ai *aiManager) clearTokenLimitError() {
	ai.stateMu.Lock()
	if strings.HasPrefix(ai.error, "Daily AI token limit") {
		ai.error = ""
	}
	ai.stateMu.Unlock()
}

// clearTokenLimitWarning resets the warning only if it is the
// approaching-limit one, so unrelated warnings survive.
func (ai *aiManager) clearTokenLimitWarning() {
	ai.stateMu.Lock()
	if strings.HasPrefix(ai.warning, "Approaching daily AI token limit") {
		ai.warning = ""
	}
	ai.stateMu.Unlock()
}

// taskFailureErrorPrefix marks status errors coming from a terminally failed
// task run, so a later successful run can clear exactly those and nothing else.
const taskFailureErrorPrefix = "AI task failed"

func (ai *aiManager) clearTaskFailureError() {
	ai.stateMu.Lock()
	if strings.HasPrefix(ai.error, taskFailureErrorPrefix) {
		ai.error = ""
	}
	ai.stateMu.Unlock()
}

func (ai *aiManager) statusStrings() (errMsg string, warnMsg string) {
	ai.stateMu.Lock()
	defer ai.stateMu.Unlock()
	return ai.error, ai.warning
}

func NewAiManager(logger *slog.Logger, valkeyClient valkeyclient.ValkeyClient, config cfg.ConfigModule, ownerCacheService store.OwnerCacheService, eventClient websocket.WebsocketClient, secretGetter SecretGetter, isLeading func() bool) AiManager {
	self := &aiManager{}

	self.logger = logger
	self.valkeyClient = valkeyClient
	self.config = config
	self.ownerCacheService = ownerCacheService
	self.eventClient = eventClient
	self.secretGetter = secretGetter
	self.mcpManager = newMCPClientManager(logger)
	self.lastCronRun = make(map[string]time.Time)
	self.lastAgentRun = make(map[string]time.Time)
	self.isLeading = isLeading
	self.taskQueueKick = make(chan struct{}, 1)
	self.runCancels = make(map[string]context.CancelFunc)
	self.pendingApprovals = make(map[string]chan approvalResult)
	self.runSem = make(chan struct{}, maxConcurrentRuns)

	// Register MCP server connectors
	self.mcpConnectors = []MCPServerConnector{
		newGitHubMCPConnector(self.getGitHubPat),
		// Add future MCP connectors here, e.g.:
		// newGitLabMCPConnector(...),
	}

	return self
}

func (ai *aiManager) ProcessObject(obj *unstructured.Unstructured, eventType string, resource utils.ResourceDescriptor) {
	if obj == nil {
		return
	}

	// Keep the enabled-agents cache in sync on every replica, before the
	// leader gate: followers use it too (e.g. after a leadership change).
	if resource.Kind == utils.AgentResource.Kind && resource.ApiVersion == utils.AgentResource.ApiVersion {
		ai.invalidateAgentCache()
	}

	// Change triggers enqueue whole-scope runs, which carry timestamped keys
	// that are NOT deduplicated across replicas — only the leader may fire.
	if ai.isLeading == nil || !ai.isLeading() {
		return
	}

	var changeType string
	switch eventType {
	case "add":
		changeType = "created"
	case "update":
		changeType = "updated"
	case "delete":
		changeType = "deleted"
	default:
		return
	}

	ai.triggerChangeAgents(obj, changeType)
}

// BACKGROUND PROCESSING
func (ai *aiManager) Run() {
	// On startup, reset any potentially orphaned in-progress tasks back to pending
	if err := ai.resetInProgressTasksOnStartup(); err != nil {
		ai.logger.Error("Failed resetting in-progress AI tasks on startup", "error", err)
	}

	// Connect to configured MCP servers (hard-coded connectors, e.g. GitHub)
	ai.connectMCPServers()

	// Connect to McpServer CR-defined servers available in the store at startup.
	ai.RefreshAllMcpServerCRConnections()

	ticker := time.NewTicker(1 * time.Minute)
	cleanupTicker := time.NewTicker(5 * time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				ai.runQueuePass(true)
			case <-ai.taskQueueKick:
				// A task just landed in the queue — start it right away
				// instead of waiting for the next tick. Cron evaluation
				// stays on its 1-minute cadence.
				ai.runQueuePass(false)
			case <-cleanupTicker.C:
				ai.cleanupOrphanedTasks()
			}
		}
	}()
}

// runQueuePass is one pass of the queue loop; includeCron additionally
// evaluates agent cron triggers (leader-only).
func (ai *aiManager) runQueuePass(includeCron bool) {
	if !ai.isAiPromptConfigInitialized() {
		return
	}
	if !ai.isAiModelConfigInitialized() {
		return
	}

	// Cron runs only on the leading replica — unlike event tasks,
	// their per-run keys are not deduplicated across replicas.
	if includeCron && ai.isLeading != nil && ai.isLeading() {
		ai.processAgentCronTriggers()
	}

	ai.setError("")
	ai.processAiTaskQueue(context.Background())
}

// kickTaskQueue wakes the queue loop without blocking; a pending kick is
// enough, extra ones coalesce.
func (ai *aiManager) kickTaskQueue() {
	if ai.taskQueueKick == nil {
		return
	}
	select {
	case ai.taskQueueKick <- struct{}{}:
	default:
	}
}

// cleanupOrphanedTasks removes AI tasks whose referenced resource no longer
// exists in the resource store. This handles cases where the operator missed
// a delete event (e.g., restart, connectivity issue) and the task would
// otherwise linger until its 7-day TTL expires.
func (ai *aiManager) cleanupOrphanedTasks() {
	keys, err := ai.getAllTaskKeys()
	if err != nil {
		ai.logger.Error("Error fetching AI task keys for orphan cleanup", "error", err)
		return
	}

	for _, key := range keys {
		item, err := ai.valkeyClient.Get(key)
		if err != nil {
			continue
		}

		var task AiTask
		if err := json.Unmarshal([]byte(item), &task); err != nil {
			continue
		}

		// Only clean up settled tasks — pending/in-progress/proposed/executing
		// tasks may reference resources that are about to be created, are
		// being processed, or await a user decision.
		switch task.State {
		case AI_TASK_STATE_COMPLETED, AI_TASK_STATE_FAILED, AI_TASK_STATE_REJECTED, AI_TASK_STATE_EXECUTED, AI_TASK_STATE_EXECUTION_FAILED:
		default:
			continue
		}

		ref := task.ReferencingResource
		if ref.ResourceName == "" {
			// Whole-scope agent runs reference no single resource; they only
			// expire via TTL.
			continue
		}
		_, err = store.GetResource(ai.valkeyClient, ref.ApiVersion, ref.Kind, ref.Namespace, ref.ResourceName, ai.logger)
		if err != nil {
			// Resource no longer in store — clean up the task
			if delErr := ai.valkeyClient.DeleteSingle(key); delErr != nil {
				ai.logger.Error("Error deleting orphaned AI task", "key", key, "error", delErr)
				continue
			}
			ai.sendAiDeleteEvent(key)
			ai.logger.Info("Cleaned up orphaned AI task (resource no longer exists)", "key", key, "kind", ref.Kind, "name", ref.ResourceName, "namespace", ref.Namespace)
		}
	}
}

// resetInProgressTasksOnStartup scans existing AI tasks and resets those left in
// "in-progress" state (e.g., due to an unclean shutdown) back to "pending" so
// they can be retried by the background processor. This should be called once
// on application startup.
func (ai *aiManager) resetInProgressTasksOnStartup() error {
	keys, err := ai.getAllTaskKeys()
	if err != nil {
		return err
	}

	for _, key := range keys {
		item, err := ai.valkeyClient.Get(key)
		if err != nil {
			ai.logger.Warn("Error fetching AI task during startup reset, skipping", "key", key, "error", err)
			continue
		}

		var task AiTask
		if err := json.Unmarshal([]byte(item), &task); err != nil {
			ai.logger.Warn("Error unmarshalling AI task during startup reset, skipping", "key", key, "error", err)
			continue
		}

		if task.State == AI_TASK_STATE_IN_PROGRESS {
			task.State = AI_TASK_STATE_PENDING
			task.Error = ""
			task.CurrentActivity = ""
			if err := ai.createOrUpdateAiTask(&task, key); err != nil {
				ai.logger.Error("Error updating AI task during startup reset", "taskID", task.ID, "error", err)
				continue
			}
			ai.logger.Info("Reset AI task from in-progress to pending on startup", "taskID", task.ID)
		}

		// A task caught mid-cancel is finalized as canceled — the user asked
		// for the run to stop, and the restart stopped it.
		if task.State == AI_TASK_STATE_CANCELLING {
			task.State = AI_TASK_STATE_CANCELED
			task.CurrentActivity = ""
			if task.Error == "" {
				task.Error = "canceled by user"
			}
			ai.clearTaskCancelRequest(task.ID)
			if err := ai.createOrUpdateAiTask(&task, key); err != nil {
				ai.logger.Error("Error updating AI task during startup reset", "taskID", task.ID, "error", err)
				continue
			}
			ai.logger.Info("Reset AI task from cancelling to canceled on startup", "taskID", task.ID)
		}

		// A task caught mid-execution must not be retried automatically: the
		// proposed operation may or may not have been applied before the
		// restart, so surface the uncertainty instead of re-executing.
		if task.State == AI_TASK_STATE_EXECUTING {
			task.State = AI_TASK_STATE_EXECUTION_FAILED
			task.Error = "operator restarted during execution; verify the target resource state manually"
			if err := ai.createOrUpdateAiTask(&task, key); err != nil {
				ai.logger.Error("Error updating AI task during startup reset", "taskID", task.ID, "error", err)
				continue
			}
			ai.logger.Warn("Reset AI task from executing to execution-failed on startup", "taskID", task.ID)
		}
	}

	return nil
}

// tokenUsageSnapshot aggregates today's token usage: cluster-wide totals plus
// a per-model breakdown keyed by AiModel CR name. Entries written before the
// per-model accounting (empty ModelRef) count into the totals only. Treat as
// read-only once built — the snapshot is shared between callers.
type tokenUsageSnapshot struct {
	TotalTokens int64
	TotalRuns   int
	PerModel    map[string]int64
}

// usageSnapshotTTL bounds how stale budget checks and status builds may be.
// Bookings and resets invalidate the snapshot immediately, so the TTL only
// matters for external writers (other replicas).
const usageSnapshotTTL = 30 * time.Second

// startOfTodayUnix returns local midnight of the given time as Unix seconds.
func startOfTodayUnix(now time.Time) int64 {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
}

// aggregateTokenUsage folds usage entries into a snapshot. Pure function,
// kept separate from the Valkey plumbing for testability.
func aggregateTokenUsage(entries []UsedToken, startOfDay int64) tokenUsageSnapshot {
	snapshot := tokenUsageSnapshot{PerModel: map[string]int64{}}
	for _, entry := range entries {
		if entry.Timestamp.Unix() < startOfDay || entry.IsIgnored {
			continue
		}
		snapshot.TotalTokens += entry.TokensUsed
		snapshot.TotalRuns++
		if entry.ModelRef != "" {
			snapshot.PerModel[entry.ModelRef] += entry.TokensUsed
		}
	}
	return snapshot
}

// todayUsageSnapshot returns today's aggregated token usage, served from a
// short-lived cache so budget checks and status builds don't each SCAN
// Valkey. The mutex also single-flights concurrent rebuilds.
func (ai *aiManager) todayUsageSnapshot() (*tokenUsageSnapshot, error) {
	ai.usageSnapshotMu.Lock()
	defer ai.usageSnapshotMu.Unlock()
	if ai.usageSnapshot != nil && time.Since(ai.usageSnapshotTime) < usageSnapshotTTL {
		return ai.usageSnapshot, nil
	}
	entries, err := ai.loadTokenUsageEntries()
	if err != nil {
		return nil, err
	}
	snapshot := aggregateTokenUsage(entries, startOfTodayUnix(time.Now()))
	ai.usageSnapshot = &snapshot
	ai.usageSnapshotTime = time.Now()
	return ai.usageSnapshot, nil
}

// invalidateUsageSnapshot drops the cached snapshot so the next check sees
// fresh numbers (called after bookings and resets).
func (ai *aiManager) invalidateUsageSnapshot() {
	ai.usageSnapshotMu.Lock()
	ai.usageSnapshot = nil
	ai.usageSnapshotMu.Unlock()
}

// loadTokenUsageEntries reads all usage entries from Valkey (SCAN + batched
// GETs). Filtering by day happens in aggregateTokenUsage.
func (ai *aiManager) loadTokenUsageEntries() ([]UsedToken, error) {
	ctx := context.Background()
	var entries []UsedToken

	keys, err := ai.valkeyClient.Keys(DB_AI_BUCKET_TOKENS + ":*")
	if err != nil {
		return nil, err
	}

	for chunk := range slices.Chunk(keys, valkeyBatchSize) {
		cmds := make([]valkey.Completed, len(chunk))
		for i, key := range chunk {
			cmds[i] = ai.valkeyClient.GetValkeyClient().B().Get().Key(key).Build()
		}
		results := ai.valkeyClient.GetValkeyClient().DoMulti(ctx, cmds...)
		for _, result := range results {
			item, err := result.ToString()
			if err != nil {
				// Key might have been deleted or expired, skip it
				continue
			}
			var tokenEntry UsedToken
			if err := json.Unmarshal([]byte(item), &tokenEntry); err != nil {
				continue
			}
			entries = append(entries, tokenEntry)
		}
	}
	return entries, nil
}

// isModelBudgetExceeded reports whether the resolved model's daily token
// budget is exhausted. 0 means unlimited; errors reading the usage fail
// closed. Side-effect free — status error/warning strings are derived
// holistically in GetStatus.
func (ai *aiManager) isModelBudgetExceeded(rc *ResolvedModelConfig) bool {
	if rc == nil || rc.DailyTokenLimit <= 0 {
		return false
	}
	snapshot, err := ai.todayUsageSnapshot()
	if err != nil {
		ai.logger.Error("Error getting today's token usage, failing closed", "error", err)
		return true
	}
	used := snapshot.PerModel[rc.ModelCrName]
	if used >= rc.DailyTokenLimit {
		ai.logger.Warn("Daily token limit of model reached", "model", rc.ModelCrName, "tokensUsed", used, "dailyTokenLimit", rc.DailyTokenLimit)
		return true
	}
	return false
}

// modelBudgetExceededMessage is the user-facing explanation attached to
// tasks/chat turns blocked by an exhausted model budget.
func modelBudgetExceededMessage(rc *ResolvedModelConfig, used int64) string {
	return fmt.Sprintf("Daily token limit of AI model %q reached (%d of %d tokens used today). Runs resume after midnight, or reset the usage via the model settings (annotation %s).", rc.ModelCrName, used, rc.DailyTokenLimit, v1alpha1.AiModelResetUsageAtAnnotation)
}

// modelBudgetError builds the budget message with the current usage number.
func (ai *aiManager) modelBudgetError(rc *ResolvedModelConfig) string {
	var used int64
	if snapshot, err := ai.todayUsageSnapshot(); err == nil {
		used = snapshot.PerModel[rc.ModelCrName]
	}
	return modelBudgetExceededMessage(rc, used)
}

func (ai *aiManager) getDbStats(namespace *string) (totalDbEntries int, unprocessedDbEntries int, ignoredDbEntries int, numberOfUnreadTasks int, err error) {
	key := ai.getValkeyKey("*", "*", "*")
	if namespace != nil {
		key = ai.getValkeyKey("*", *namespace, "*")
	}

	ctx := context.Background()

	keys, err := ai.valkeyClient.Keys(key)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	for chunk := range slices.Chunk(keys, valkeyBatchSize) {
		// Build all GET commands for this batch
		cmds := make([]valkey.Completed, len(chunk))
		for i, k := range chunk {
			cmds[i] = ai.valkeyClient.GetValkeyClient().B().Get().Key(k).Build()
		}

		// Execute all GETs in a single round trip
		results := ai.valkeyClient.GetValkeyClient().DoMulti(ctx, cmds...)

		// Process results
		for _, result := range results {
			item, err := result.ToString()
			if err != nil {
				// Key might have been deleted or expired, skip it
				continue
			}

			var task AiTask
			if err := json.Unmarshal([]byte(item), &task); err != nil {
				// Log error but continue processing
				continue
			}

			totalDbEntries++

			if task.State == AI_TASK_STATE_PENDING || task.State == AI_TASK_STATE_FAILED {
				unprocessedDbEntries++
			}
			if task.State == AI_TASK_STATE_IGNORED {
				ignoredDbEntries++
			}
			if len(task.ReadByUsers) == 0 {
				numberOfUnreadTasks++
			}
		}
	}

	return totalDbEntries, unprocessedDbEntries, ignoredDbEntries, numberOfUnreadTasks, nil
}

// getDbStatsForNamespaces counts the tasks visible to a workspace holding the
// given namespaces, using the same visibility rules as GetAiTasksForWorkspace
// so status badges (e.g. unread count) match the report list: event tasks by
// their key's namespace, whole-scope agent tasks by scope/finding namespace.
func (ai *aiManager) getDbStatsForNamespaces(namespaces map[string]bool) (totalDbEntries int, unprocessedDbEntries int, ignoredDbEntries int, numberOfUnreadTasks int, err error) {
	count := func(task *AiTask) {
		totalDbEntries++
		if task.State == AI_TASK_STATE_PENDING || task.State == AI_TASK_STATE_FAILED {
			unprocessedDbEntries++
		}
		if task.State == AI_TASK_STATE_IGNORED {
			ignoredDbEntries++
		}
		if len(task.ReadByUsers) == 0 {
			numberOfUnreadTasks++
		}
	}

	for namespace := range namespaces {
		tasks, nsErr := ai.getAiTasksForNamespace(namespace)
		if nsErr != nil {
			return 0, 0, 0, 0, nsErr
		}
		for i := range tasks {
			if !isAgentTaskID(tasks[i].ID) {
				count(&tasks[i])
			}
		}
	}

	agentTasks, agentErr := ai.getAgentTasksForNamespaces(namespaces)
	if agentErr != nil {
		return 0, 0, 0, 0, agentErr
	}
	for i := range agentTasks {
		count(&agentTasks[i])
	}

	return totalDbEntries, unprocessedDbEntries, ignoredDbEntries, numberOfUnreadTasks, nil
}

func (ai *aiManager) addTokenUsage(tokensUsed int, model string, timeUsedInMs int, entryKey string, modelRef string) error {
	now := time.Now()
	// The previous key included only Unix seconds, so two tasks finishing
	// in the same second silently overwrote one another, undercounting
	// tokens against the daily limit. UnixNano + a short NanoId suffix
	// makes the key unique even under simultaneous writes; readers filter
	// by usedToken.Timestamp (the authoritative date), not by parsing the
	// key, so the schema change is backward compatible.
	key := fmt.Sprintf("%s:%d:%s", DB_AI_BUCKET_TOKENS, now.UnixNano(), utils.NanoId())

	usedToken := UsedToken{
		Key:          entryKey,
		Timestamp:    now,
		TokensUsed:   int64(tokensUsed),
		IsIgnored:    false,
		Model:        model,
		TimeUsedInMs: timeUsedInMs,
		ModelRef:     modelRef,
	}

	err := ai.valkeyClient.SetObject(usedToken, ValkeyAiTTL, key)
	if err != nil {
		return fmt.Errorf("error saving AI token usage: %v", err)
	}

	metrics.AddAiTokensUsed(modelRef, tokensUsed)
	ai.invalidateUsageSnapshot()
	return nil
}

// resetTodayTokenUsage zeroes today's usage entries of one model (by AiModel
// CR name; "" resets every entry) and returns the number of tokens cleared.
func (ai *aiManager) resetTodayTokenUsage(modelRef string) (int64, error) {
	startOfDay := startOfTodayUnix(time.Now())

	keys, err := ai.valkeyClient.Keys(fmt.Sprintf("%s:*", DB_AI_BUCKET_TOKENS))
	if err != nil {
		return 0, err
	}

	var resettedTokens int64 = 0
	for _, key := range keys {
		item, err := ai.valkeyClient.Get(key)
		if err != nil {
			return resettedTokens, err
		}
		var tokenEntry UsedToken
		err = json.Unmarshal([]byte(item), &tokenEntry)
		if err != nil {
			return resettedTokens, err
		}
		if tokenEntry.Timestamp.Unix() < startOfDay || tokenEntry.TokensUsed == 0 {
			continue
		}
		if modelRef != "" && tokenEntry.ModelRef != modelRef {
			continue
		}
		resettedTokens += tokenEntry.TokensUsed
		tokenEntry.TokensUsed = 0
		if err := ai.valkeyClient.SetObject(tokenEntry, ValkeyAiTTL, key); err != nil {
			return resettedTokens, fmt.Errorf("error saving AI token usage: %v", err)
		}
	}
	ai.logger.Info("Reset today's AI token usage", "modelRef", modelRef, "resettedTokens", resettedTokens)

	ai.invalidateUsageSnapshot()
	ai.resetCache()

	return resettedTokens, nil
}

// ResetTokenUsageForModel zeroes today's recorded usage of one AiModel.
// Reconciler-facing wrapper around resetTodayTokenUsage.
func (ai *aiManager) ResetTokenUsageForModel(modelCrName string) (int64, error) {
	if modelCrName == "" {
		return 0, fmt.Errorf("model name must not be empty")
	}
	return ai.resetTodayTokenUsage(modelCrName)
}

func (ai *aiManager) getAllTaskKeys() ([]string, error) {
	return ai.valkeyClient.Keys(fmt.Sprintf("%s:*", DB_AI_BUCKET_TASKS))
}

type aiTaskWithKey struct {
	key  string
	task AiTask
}

// approvalResult carries the outcome of an approved or rejected tool call
// back to the agent run that is blocking on it.
type approvalResult struct {
	approver structs.User
	err      error
}

func (ai *aiManager) processAiTaskQueue(ctx context.Context) {
	keys, err := ai.getAllTaskKeys()
	if err != nil {
		ai.logger.Error("Error listing AI tasks", "error", err)
		return
	}

	// Load all pending/failed tasks and sort by CreatedAt ascending (oldest first)
	var pendingTasks []aiTaskWithKey
	for _, key := range keys {
		item, err := ai.valkeyClient.Get(key)
		if err != nil {
			ai.logger.Error("Error getting AI task", "key", key, "error", err)
			continue
		}
		var task AiTask
		err = json.Unmarshal([]byte(item), &task)
		if err != nil {
			ai.logger.Error("Error unmarshaling AI task", "key", key, "error", err)
			continue
		}
		if task.State == AI_TASK_STATE_PENDING || (task.State == AI_TASK_STATE_FAILED && task.Retries < maxAiTaskRetries) {
			pendingTasks = append(pendingTasks, aiTaskWithKey{key: key, task: task})
		}
	}
	sort.Slice(pendingTasks, func(i, j int) bool {
		return pendingTasks[i].task.CreatedAt < pendingTasks[j].task.CreatedAt
	})

	for _, entry := range pendingTasks {
		key := entry.key
		task := entry.task

		// Resolve the owning agent; tasks whose agent vanished, was disabled
		// or whose resource left the agent's scope are ignored.
		agent, toolCtx, err := ai.buildAgentTaskContext(&task)
		if err != nil {
			task.State = AI_TASK_STATE_IGNORED
			task.Error = err.Error()
			if updateErr := ai.createOrUpdateAiTask(&task, key); updateErr != nil {
				ai.logger.Error("Error updating AI task to ignored state", "taskID", task.ID, "error", updateErr)
			}
			ai.logger.Info("AI task ignored", "taskID", task.ID, "agent", task.AgentRef, "reason", err.Error())
			continue
		}

		// Resolve the model config once for the whole run — the budget check
		// below and the run itself must see the same model.
		rc, err := ai.resolveModelConfig(&agent.Spec)
		if err != nil {
			// Same semantics as a run failure: retried, then ignored (the
			// model may reappear or be fixed).
			task.Error = err.Error()
			task.Retries++
			task.State = AI_TASK_STATE_FAILED
			if task.Retries >= maxAiTaskRetries {
				task.State = AI_TASK_STATE_IGNORED
				task.Error = fmt.Sprintf("giving up after %d failed attempts: %s", task.Retries, err.Error())
				ai.setError(fmt.Sprintf("%s after %d attempts: %s", taskFailureErrorPrefix, task.Retries, err.Error()))
			}
			if updateErr := ai.createOrUpdateAiTask(&task, key); updateErr != nil {
				ai.logger.Error("Error updating AI task", "taskID", task.ID, "error", updateErr)
			}
			ai.logger.Error("Failed to resolve model config for AI task", "taskID", task.ID, "attempt", task.Retries, "error", err)
			continue
		}

		if ai.isModelBudgetExceeded(rc) {
			// Deliberately no Retries++: the task stays eligible and runs
			// again once the budget frees up (midnight or usage reset).
			task.State = AI_TASK_STATE_FAILED
			task.Error = ai.modelBudgetError(rc)
			if err := ai.createOrUpdateAiTask(&task, key); err != nil {
				ai.logger.Error("Error updating AI task", "taskID", task.ID, "error", err)
			}
			continue
		}

		// A queue pass runs for minutes; the task was selected as pending at
		// the start of the pass but may have been canceled (or otherwise
		// resolved) since — starting it now would resurrect it.
		if current, curErr := ai.getTaskByKey(key); curErr != nil || current == nil ||
			(current.State != AI_TASK_STATE_PENDING && current.State != AI_TASK_STATE_FAILED) {
			continue
		}

		task.State = AI_TASK_STATE_IN_PROGRESS
		task.Error = ""
		err = ai.createOrUpdateAiTask(&task, key)
		if err != nil {
			ai.logger.Error("Failed to set AI task state to in progress", "taskID", task.ID, "error", err)
			continue
		}

		// Send IN_PROGRESS notification immediately so the UI reflects the new state.
		ai.sendAiEvent(&AiTaskLatest{Task: &task, Status: ai.GetStatus(nil)})

		// Each run gets its own goroutine so approval waits (up to 7 days) don't
		// stall the queue. The semaphore keeps the goroutine count bounded; a
		// waiting goroutine is cheap and the task is already IN_PROGRESS so the
		// next queue pass won't re-pick it up.
		taskSnapshot := task
		agentSnapshot := *agent
		go func() {
			ai.runSem <- struct{}{}
			defer func() { <-ai.runSem }()
			ai.runOneTask(ctx, taskSnapshot, key, rc, agentSnapshot, toolCtx)
		}()
	}
}

// runOneTask drives a single agent run from IN_PROGRESS to its terminal state.
// It is invoked in a dedicated goroutine so approval waits don't stall the queue.
func (ai *aiManager) runOneTask(ctx context.Context, task AiTask, key string, rc *ResolvedModelConfig, agent v1alpha1.Agent, toolCtx *ToolContext) {
	latestTask := &AiTaskLatest{
		Task:   &task,
		Status: ai.GetStatus(nil),
	}

	// Per-task cancellable context: CancelTask aborts it directly when the
	// cancel request lands on this replica; a cancel marker in Valkey (set
	// by any replica) additionally aborts the LLM loop at the next turn
	// boundary. The same per-turn hook pushes live token counts to the UI,
	// throttled so a fast tool-call storm doesn't flood the event channel.
	taskCtx, cancelTask := context.WithCancel(ctx)
	ai.registerRunCancel(task.ID, cancelTask)
	var lastProgressPush time.Time
	onProgress := func(tokens int64, activity string) {
		if ai.taskCancelReason(task.ID) != "" {
			cancelTask()
			return
		}
		task.TokensUsed = tokens
		if activity != "" {
			// Keep the last activity even across throttled pushes so the
			// next event carries the current one, not a stale line.
			task.CurrentActivity = activity
		}
		if time.Since(lastProgressPush) < 2*time.Second {
			return
		}
		lastProgressPush = time.Now()
		if err := ai.createOrUpdateAiTask(&task, key); err != nil {
			ai.logger.Warn("Failed to persist AI task progress", "taskID", task.ID, "error", err)
		}
		ai.sendAiEvent(latestTask)
	}

	// Steps are keyed by the run id (== primary task ID) so the timeline
	// survives even when the run later spawns finding tasks.
	recordStep := ai.newStepRecorder(task.ID)

	tokensUsed, timeUsedInMs, modelUsed, err := ai.processPrompt(taskCtx, rc, task.Prompt, toolCtx, &agent.Spec, onProgress, recordStep)
	ai.unregisterRunCancel(task.ID)
	cancelTask()
	task.CurrentActivity = ""
	// Consume the cancel marker no matter how the run ended — a marker
	// surviving into a later retry of the same task would cancel that
	// retry on its first turn.
	cancelReason := ai.taskCancelReason(task.ID)
	if cancelReason != "" {
		ai.clearTaskCancelRequest(task.ID)
	}
	task.Model = modelUsed
	task.TimeUsedInMs = timeUsedInMs
	task.TokensUsed = tokensUsed
	discardTask := false
	if err != nil {
		if errors.Is(err, context.Canceled) && ctx.Err() == nil && cancelReason != "" {
			// Canceled by a user, not by shutdown: the run is void, not broken.
			task.State = AI_TASK_STATE_CANCELED
			task.Error = cancelReason
			ai.logger.Info("AI task canceled", "taskID", task.ID, "reason", cancelReason)
		} else {
			task.Error = err.Error()
			task.Retries++
			// Non-retryable API errors (billing, invalid request, auth) must not be retried.
			// Mark as ignored so the queue skips them on the next pass.
			var budgetErr *BudgetExhaustedError
			var apiErr *anthropic.Error
			if errors.As(err, &budgetErr) {
				// Per-run budget (token or tool-call limit): retrying won't help
				// until the user raises the limit in the model or agent settings.
				task.State = AI_TASK_STATE_CANCELED
			} else if errors.As(err, &apiErr) && (apiErr.StatusCode == 400 || apiErr.StatusCode == 401 || apiErr.StatusCode == 403) {
				task.State = AI_TASK_STATE_IGNORED
				// The trigger handler already answered 200 with the pending
				// task; without this the failure is only visible in the log.
				ai.setError(fmt.Sprintf("%s (HTTP %d, not retried): %s", taskFailureErrorPrefix, apiErr.StatusCode, err.Error()))
			} else if task.Retries >= maxAiTaskRetries {
				// Every retry re-runs the whole analysis loop; a task that
				// failed repeatedly is broken systematically, not transiently.
				task.State = AI_TASK_STATE_IGNORED
				task.Error = fmt.Sprintf("giving up after %d failed attempts: %s", task.Retries, err.Error())
				ai.setError(fmt.Sprintf("%s after %d attempts: %s", taskFailureErrorPrefix, task.Retries, err.Error()))
			} else {
				task.State = AI_TASK_STATE_FAILED
			}
			ai.logger.Error("Error processing AI task", "taskID", task.ID, "attempt", task.Retries, "state", task.State, "error", err)
		}
		// Close the timeline with the failure so the run's step history
		// explains itself without cross-checking the task error field.
		recordStep(AiRunStep{Kind: AI_RUN_STEP_ERROR, Label: task.Error})
	} else {
		// The run completed — proposals were created as separate PROPOSED
		// tasks via CreateApprovalRequest. The primary task is an all-clear.
		discardTask = true
		ai.clearTaskFailureError()
	}
	if err := ai.addTokenUsage(int(tokensUsed), modelUsed, timeUsedInMs, key, rc.ModelCrName); err != nil {
		ai.logger.Error("Error recording AI token usage", "taskID", task.ID, "error", err)
	}

	// update status for event
	ai.resetCache()
	latestTask.Status = ai.GetStatus(nil)

	if discardTask {
		// All-clear: the run inspected its scope and found nothing (new)
		// to fix. Keep it as a success report — a silently vanishing run
		// reads like a failure. The history is capped per agent (see
		// pruneAgentRunsToLimit), not collapsed to the newest all-clear.
		task.State = AI_TASK_STATE_COMPLETED
		task.Response = nil
		task.Error = ""
		if err := ai.createOrUpdateAiTask(&task, key); err != nil {
			ai.logger.Error("Error saving all-clear AI task", "taskID", task.ID, "error", err)
			return
		}
		ai.pruneAgentRunsToLimit(task.AgentRef)
		ai.sendAiEvent(latestTask)
		ai.logger.Info("AI run found nothing applicable — kept as all-clear report", "taskID", task.ID, "tokensUsed", tokensUsed)
		return
	}

	// send event notification
	ai.sendAiEvent(latestTask)

	// Save updated task
	if err := ai.createOrUpdateAiTask(&task, key); err != nil {
		ai.logger.Error("Error updating AI task", "taskID", task.ID, "error", err)
		return
	}
	// A retryable failure keeps the run open; every other outcome here
	// (ignored, canceled) is terminal and counts against the history cap.
	if task.State != AI_TASK_STATE_FAILED {
		ai.pruneAgentRunsToLimit(task.AgentRef)
	}
	ai.logger.Info("AI task processed", "taskID", task.ID, "tokensUsed", task.TokensUsed, "state", task.State, "name", task.ReferencingResource.ResourceName, "namespace", task.ReferencingResource.Namespace)
}

// HELPER FUNCTIONS
func (ai *aiManager) createOrUpdateAiTask(task *AiTask, key string) error {
	timestamp := time.Now().Unix()
	task.UpdatedAt = timestamp

	jsonString, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("error marshaling AI task: %v", err)
	}
	err = ai.valkeyClient.Set(string(jsonString), ValkeyAiTTL, key)
	if err != nil {
		return fmt.Errorf("error saving AI task: %v", err)
	}

	// New pending work starts immediately instead of waiting for the ticker.
	if task.State == AI_TASK_STATE_PENDING {
		ai.kickTaskQueue()
	}

	// last updated task
	err = ai.valkeyClient.Set(string(jsonString), ValkeyAiTTL, ai.getValkeyLatestTaskKey())
	if err != nil {
		ai.logger.Warn("Error saving AI task", "error", err)
	}
	for _, namespace := range latestTaskNamespaces(task, key) {
		err = ai.valkeyClient.Set(string(jsonString), ValkeyAiTTL, ai.getValkeyLatestNamespaceTaskKey(namespace))
		if err != nil {
			ai.logger.Warn("Error saving AI task for namespace", "namespace", namespace, "error", err)
		}
	}

	return nil
}

// latestTaskNamespaces returns the namespaces whose "latest task" pointer a
// save should update. Event tasks belong to their key's namespace segment.
// Whole-scope agent tasks used to inherit that segment too — the
// alphabetically first scope namespace — which pinned the run to one
// arbitrary namespace. Instead, a finding belongs to its target's namespace
// and every other run state (pending, running, all-clear) to the whole
// scope, so each affected workspace sees the run as its latest activity.
func latestTaskNamespaces(task *AiTask, key string) []string {
	parts := strings.SplitN(key, ":", 4)
	if len(parts) != 4 {
		return nil
	}
	if parts[1] != "Agent" {
		return []string{parts[2]}
	}
	if len(task.ScopeNamespaces) > 0 {
		return task.ScopeNamespaces
	}
	return []string{parts[2]}
}

// processPrompt runs the unattended tool-loop. The ToolContext scopes every
// tool call to the owning agent's namespaces; mutating built-in K8s tools and
// MCP tools with needsApprove policy are intercepted and turned into PROPOSED
// tasks instead of being executed directly. The primary run always completes
// without findings — proposals arrive as independent tasks via CreateApprovalRequest.
func (ai *aiManager) processPrompt(ctx context.Context, rc *ResolvedModelConfig, prompt string, toolCtx *ToolContext, agentSpec *v1alpha1.AgentSpec, onProgress func(tokensUsed int64, activity string), recordStep StepRecorder) (tokensUsed int64, timeUsedInMs int, modelUsed string, err error) {
	startTime := time.Now()
	systemPrompt := ai.getSystemPrompt()

	switch rc.Sdk {
	case AiSdkTypeOpenAI:
		return ai.processPromptOpenAi(ctx, rc, systemPrompt, prompt, toolCtx, onProgress, recordStep)
	case AiSdkTypeAnthropic:
		return ai.processPromptAnthropic(ctx, rc, systemPrompt, prompt, toolCtx, onProgress, recordStep)
	case AiSdkTypeOllama:
		return ai.processPromptOllama(ctx, rc, systemPrompt, prompt, toolCtx, onProgress, recordStep)
	default:
		return 0, int(time.Since(startTime).Milliseconds()), rc.Model, fmt.Errorf("unsupported AI SDK type: %s", rc.Sdk)
	}
}

// getTaskByKey loads and unmarshals a task from Valkey; returns nil when the
// key does not exist.
func (ai *aiManager) getTaskByKey(key string) (*AiTask, error) {
	item, err := ai.valkeyClient.Get(key)
	if err != nil {
		return nil, err
	}
	if item == "" {
		return nil, nil
	}
	var task AiTask
	if err := json.Unmarshal([]byte(item), &task); err != nil {
		return nil, fmt.Errorf("error unmarshaling AI task %q: %w", key, err)
	}
	return &task, nil
}

// newOpenAIClientFor builds an OpenAI client for one resolved model config.
// An empty BaseUrl selects the SDK's default public endpoint.
func (ai *aiManager) newOpenAIClientFor(rc *ResolvedModelConfig) *openai.Client {
	opts := []option.RequestOption{option.WithAPIKey(rc.ApiKey)}
	if rc.BaseUrl != "" {
		opts = append(opts, option.WithBaseURL(rc.BaseUrl))
	}
	client := openai.NewClient(opts...)
	return &client
}

// newAnthropicClientFor builds an Anthropic client for one resolved model
// config. An empty BaseUrl selects the SDK's default public endpoint.
func (ai *aiManager) newAnthropicClientFor(rc *ResolvedModelConfig) *anthropic.Client {
	opts := []anthropic_option.RequestOption{anthropic_option.WithAPIKey(rc.ApiKey)}
	if rc.BaseUrl != "" {
		opts = append(opts, anthropic_option.WithBaseURL(rc.BaseUrl))
	}
	client := anthropic.NewClient(opts...)
	return &client
}

// newOllamaClientFor builds an Ollama client for one resolved model config.
// Ollama has no public default endpoint, so BaseUrl must be set (enforced by
// ValidateAiModelSpec / the legacy config).
func (ai *aiManager) newOllamaClientFor(rc *ResolvedModelConfig) (*ollama.Client, error) {
	url, err := url.Parse(rc.BaseUrl)
	if err != nil {
		return nil, err
	}
	return ollama.NewClient(url, http.DefaultClient), nil
}

// modelsRequestConfig turns an explicit ModelsRequest (UI-supplied SDK and
// credentials, e.g. from the "add model" dialog) into a resolved config. A nil
// request resolves the configured model (default AiModel or legacy secret)
// instead; a request without an API key borrows the configured one.
func (ai *aiManager) modelsRequestConfig(request *ModelsRequest) (*ResolvedModelConfig, error) {
	if request == nil {
		return ai.resolveModelConfig(nil)
	}
	rc := &ResolvedModelConfig{
		Source:  "request",
		Sdk:     AiSdkType(request.Sdk),
		BaseUrl: request.ApiUrl,
	}
	switch {
	case request.ApiKey != nil:
		rc.ApiKey = *request.ApiKey
	case request.ApiKeySecretName != "":
		ownNamespace, err := ai.config.TryGet("MO_OWN_NAMESPACE")
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve own namespace: %w", err)
		}
		apiKey, err := ai.resolveApiKeyFromRef(ownNamespace, &v1alpha1.SecretKeyRef{
			Name: request.ApiKeySecretName,
			Key:  request.ApiKeySecretKey,
		})
		if err != nil {
			return nil, err
		}
		rc.ApiKey = apiKey
	default:
		if configured, err := ai.resolveModelConfig(nil); err == nil {
			rc.ApiKey = configured.ApiKey
		}
	}
	return rc, nil
}

func (ai *aiManager) getOpenAIClient(request *ModelsRequest) (*openai.Client, error) {
	rc, err := ai.modelsRequestConfig(request)
	if err != nil {
		return nil, err
	}
	return ai.newOpenAIClientFor(rc), nil
}

func (ai *aiManager) getAnthropicClient(request *ModelsRequest) (*anthropic.Client, error) {
	rc, err := ai.modelsRequestConfig(request)
	if err != nil {
		return nil, err
	}
	return ai.newAnthropicClientFor(rc), nil
}

func (ai *aiManager) getOllamaClient(request *ModelsRequest) (*ollama.Client, error) {
	rc, err := ai.modelsRequestConfig(request)
	if err != nil {
		return nil, err
	}
	return ai.newOllamaClientFor(rc)
}

func (ai *aiManager) GetAvailableModels(request *ModelsRequest) ([]string, error) {
	var sdk AiSdkType
	if request != nil {
		sdk = AiSdkType(request.Sdk)
	} else {
		rc, err := ai.resolveModelConfig(nil)
		if err != nil {
			return []string{}, err
		}
		sdk = rc.Sdk
	}
	switch sdk {
	case AiSdkTypeOpenAI:
		client, err := ai.getOpenAIClient(request)
		if err != nil {
			ai.logger.Error("Error getting OpenAI client for available models", "error", err)
			return []string{}, err
		}

		ctx := context.Background()
		models, err := client.Models.List(ctx)
		if err != nil {
			ai.logger.Error("Error listing available AI models", "error", err)
			return []string{}, err
		}

		var modelNames []string
		for _, model := range models.Data {
			modelNames = append(modelNames, model.ID)
		}
		return modelNames, nil
	case AiSdkTypeAnthropic:
		client, err := ai.getAnthropicClient(request)
		if err != nil {
			ai.logger.Error("Error getting Anthropic client for available models", "error", err)
			return []string{}, err
		}

		ctx := context.Background()
		models, err := client.Models.List(ctx, anthropic.ModelListParams{})
		if err != nil {
			ai.logger.Error("Error listing available AI models", "error", err)
			return []string{}, err
		}

		var modelNames []string
		for _, model := range models.Data {
			modelNames = append(modelNames, model.ID)
		}
		return modelNames, nil
	case AiSdkTypeOllama:
		client, err := ai.getOllamaClient(request)
		if err != nil {
			ai.logger.Error("Error getting Ollama client for available models", "error", err)
			return []string{}, err
		}

		ctx := context.Background()
		listResponse, err := client.List(ctx)
		if err != nil {
			ai.logger.Error("Error listing available AI models", "error", err)
			return []string{}, err
		}

		var modelNames []string
		for _, model := range listResponse.Models {
			modelNames = append(modelNames, model.Name)
		}
		return modelNames, nil
	default:
		return []string{}, fmt.Errorf("unsupported AI SDK type: %s", sdk)
	}
}

func (ai *aiManager) getValkeyKey(kind, namespace, name string) string {
	// controller lookup for pods
	if kind == "Pod" {
		controller := ai.ownerCacheService.ControllerForPod(namespace, name)
		if controller != nil {
			kind = controller.Kind
			name = controller.ResourceName
		}
	}
	return fmt.Sprintf("%s:%s:%s:%s", DB_AI_BUCKET_TASKS, kind, namespace, name)
}

func (ai *aiManager) getValkeyLatestTaskKey() string {
	return fmt.Sprintf("%s:%s", DB_AI_BUCKET_TASKS_LATEST, DB_AI_LATEST_TASK_KEY)
}

func (ai *aiManager) getValkeyLatestNamespaceTaskKey(namespace string) string {
	return fmt.Sprintf("%s:%s:%s", DB_AI_BUCKET_TASKS_LATEST, DB_AI_LATEST_NAMESPACE_TASK_KEY, namespace)
}

func (ai *aiManager) sendAiEvent(task *AiTaskLatest) {
	datagram := structs.Datagram{
		Id:      utils.NanoId(),
		Pattern: "AiProcessEvent",
		Payload: map[string]any{
			"task":   task.Task,
			"status": task.Status,
		},
		CreatedAt: time.Now(),
	}
	structs.ReportEventToServer(ai.eventClient, datagram)
}

func (ai *aiManager) sendAiDeleteEvent(taskId string) {
	datagram := structs.Datagram{
		Id:      utils.NanoId(),
		Pattern: "AiDeleteEvent",
		Payload: map[string]any{
			"taskId": taskId,
		},
		CreatedAt: time.Now(),
	}
	structs.ReportEventToServer(ai.eventClient, datagram)
}

func (ai *aiManager) resetCache() {
	aiStatusMu.Lock()
	defer aiStatusMu.Unlock()
	cachedStatusTime = time.Time{}
	for k := range cachedWorkspaceStatusTime {
		delete(cachedWorkspaceStatusTime, k)
	}
}
