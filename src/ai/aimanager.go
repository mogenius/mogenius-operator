package ai

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"mogenius-operator/src/websocket"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"encoding/json"

	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/jsonpath"
	"sigs.k8s.io/yaml"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/valkey-io/valkey-go"

	"github.com/anthropics/anthropic-sdk-go"
	anthropic_option "github.com/anthropics/anthropic-sdk-go/option"

	"github.com/ollama/ollama/api"
	ollama "github.com/ollama/ollama/api"
)

const (
	DB_AI_BUCKET_TASKS              = "ai_tasks"
	DB_AI_BUCKET_TASKS_LATEST       = "ai_tasks_latest"
	DB_AI_BUCKET_TOKENS             = "ai_tokens"
	DB_AI_LATEST_TASK_KEY           = "latest-task"
	DB_AI_LATEST_NAMESPACE_TASK_KEY = "latest-namespace-task"
)

var ValkeyAiTTL = time.Hour * 24 * 7 // 7 days

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
	TriggeredBy         AiFilter                    `json:"triggeredBy"`         // the event filter that matched (empty for cron/manual runs)
	ReadByUsers         []ReadBy                    `json:"readByUsers"`
	Error               string                      `json:"error"`

	AgentRef        string        `json:"agentRef,omitempty"`        // name of the Agent CR this task belongs to
	Trigger         string        `json:"trigger,omitempty"`         // "event", "cron" or "manual"
	TriggeredByUser *structs.User `json:"triggeredByUser,omitempty"` // set for manual triggers

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

type AiPromptConfig struct {
	Id           string     `json:"id"`
	Name         string     `json:"name"`
	SystemPrompt string     `json:"systemPrompt"`
	Filters      []AiFilter `json:"filters"`
	UserFilters  []AiFilter `json:"userFilters"` // filters added by users via the UI
}

type AiPrompts struct {
	ChatSystemPrompt                string `json:"chatSystemPrompt"`
	GithubSystemPrompt              string `json:"githubSystemPrompt"`
	GitMemoryRepositorySystemPrompt string `json:"gitMemoryRepositorySystemPrompt"`
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
}

type AiResponse struct {
	ErrorMessage string   `json:"errorMessage"`
	Analysis     Analysis `json:"analysis"`
}

type Analysis struct {
	ProblemDescription  string                      `json:"problemDescription"`
	PossibleCauses      []string                    `json:"possibleCauses"`
	ProposedSolutions   []Solution                  `json:"proposedSolutions"`
	AdditionalInfo      string                      `json:"additionalInformation"`
	NeedsFollowUp       bool                        `json:"needsFollowUp"`
	FollowUpResources   []FollowUpResource          `json:"followUpResources"`
	CurrentResourceYaml string                      `json:"currentResourceYaml"`
	TargetResourceYaml  string                      `json:"targetResourceYaml"`
	TargetResource      utils.WorkloadSingleRequest `json:"targetResource"`
	ProposedOperation   string                      `json:"proposedOperation,omitempty"` // UpdateResource', 'DeleteResource', 'CreateResource', 'Other'

	// AdditionalTargets turns a DeleteResource proposal into a bulk deletion:
	// TargetResource plus every entry here are deleted together in one
	// reviewable proposal. Only valid for DeleteResource.
	AdditionalTargets []utils.WorkloadSingleRequest `json:"additionalTargets,omitempty"`
}

type Solution struct {
	SolutionDescription string   `json:"solutionDescription"`
	Steps               []string `json:"steps"`
}

// values of Analysis.ProposedOperation
const (
	ProposedOperationUpdate = "UpdateResource"
	ProposedOperationDelete = "DeleteResource"
	ProposedOperationCreate = "CreateResource"
	ProposedOperationOther  = "Other"
)

type UsedToken struct {
	Timestamp    time.Time `json:"timestamp"`
	TokensUsed   int64     `json:"tokensUsed"`
	IsIgnored    bool      `json:"isIgnored"`
	Key          string    `json:"key"`
	Model        string    `json:"model"`
	TimeUsedInMs int       `json:"timeUsedInMs"`
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
	InjectAiPromptConfig(prompt AiPromptConfig, aiPrompts *AiPrompts)
	GetStatus(workspace *string) AiManagerStatus
	ResetDailyTokenLimit() error
	DeleteAllAiData() error
	GetAvailableModels(request *ModelsRequest) ([]string, error)
	TestAiModel(name string) (*AiModelTestResult, error)
	GetPromptConfig() (*AiPromptConfig, error)
	Chat(ctx context.Context, ch IOChatChannel) error

	ApproveTask(taskID string, user structs.User, workspace string) (*AiTask, error)
	RejectTask(taskID string, user structs.User, reason string) (*AiTask, error)
	CancelTask(taskID string, user structs.User) (*AiTask, error)
	DeleteTask(taskID string, user structs.User) (*AiTask, error)
	TriggerAgent(agentName string, user *structs.User) (*AiTask, error)

	ResolveWorkspaceContext(userEmail string, workspaceName string) (*v1alpha1.WorkspaceSpec, *v1alpha1.GrantSpec)
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
	pendingTasks      map[string]AiTask
	pendingTasksLock  *sync.RWMutex
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

	// prompts
	chatPromptMu sync.RWMutex
	aiPrompts    AiPrompts
}

// auditInsightToolCall writes a durable audit entry for every tool the
// unattended agent pipeline executes ("no unattributed actions"): unlike
// the chat path, this pipeline has no user whose audit trail would capture
// the call, so calls are attributed to the agent's synthetic user. The result
// is truncated — the entry documents WHAT was queried, not the full payload.
func (ai *aiManager) auditInsightToolCall(toolCtx *ToolContext, toolName string, args map[string]any, result string) {
	user := structs.User{FirstName: "AI", LastName: "Insights", Email: "ai-insights@system", Source: "ai-insights"}
	workspace := ""
	if toolCtx != nil {
		if toolCtx.User != nil {
			user = *toolCtx.User
		}
		workspace = toolCtx.Workspace
	}
	store.AddAiChatAuditLog(
		ai.logger,
		"ai/insight-tool",
		map[string]any{"tool": toolName, "args": args},
		truncateResult(result, 500),
		"",
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
	self.pendingTasks = make(map[string]AiTask)
	self.pendingTasksLock = &sync.RWMutex{}
	self.mcpManager = newMCPClientManager(logger)
	self.lastCronRun = make(map[string]time.Time)
	self.lastAgentRun = make(map[string]time.Time)
	self.isLeading = isLeading
	self.taskQueueKick = make(chan struct{}, 1)

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

func (ai *aiManager) insertNewAiTask(task *AiTask, obj *unstructured.Unstructured, eventType string, key string) {
	controller := ai.ownerCacheService.OwnerFromReference(obj.GetNamespace(), obj.GetOwnerReferences())
	task.Controller = controller
	if controller != nil {
		ctrlOb, err := store.GetResource(ai.valkeyClient, controller.ResourceDescriptor.ApiVersion, controller.ResourceDescriptor.Kind, controller.Namespace, controller.ResourceName, ai.logger)
		if err != nil {
			ai.logger.Error("Error fetching controller object for AI task", "controllerKind", controller.ResourceDescriptor.Kind, "controllerName", controller.ResourceName, "controllerNamespace", controller.Namespace, "error", err)
		} else {
			if ctrlOb != nil {
				controllerYaml, err := store.GetYamlFromUnstructuredResource(ctrlOb)
				if err != nil {
					ai.logger.Error("Error generating controller YAML for AI task prompt", "controllerKind", controller.ResourceDescriptor.Kind, "controllerName", controller.ResourceName, "controllerNamespace", controller.Namespace, "error", err)
				}
				task.Prompt += "\n\nThe controller resource YAML is as follows:\n" + controllerYaml
			}
		}
	}
	err := ai.createOrUpdateAiTask(task, key)
	if err != nil {
		ai.logger.Error("Error creating AI task", "error", err)
	} else {
		ai.logger.Info("AI task created", "taskID", task.ID, "event", eventType, "objectKind", obj.GetKind(), "objectName", obj.GetName(), "objectNamespace", obj.GetNamespace(), "filter", task.TriggeredBy.Name)
	}
}

func filterMatchesForObject(filter AiFilter, obj *unstructured.Unstructured) (bool, error) {
	matched := false
	for path, expectedValue := range filter.Contains {
		value, found, err := getNestedStringWithJSONPath(obj, path, expectedValue)
		if err != nil {
			return false, fmt.Errorf("Error checking AI filter contains: expectedValue=%s, error=%v", expectedValue, err)
		}

		if !found {
			continue
		}

		// For array results (comma-separated), check if expectedValue is in any of the values
		values := strings.SplitSeq(value, ", ")
		for v := range values {
			if strings.TrimSpace(v) == expectedValue {
				matched = true
				break
			}
		}

		if matched {
			break
		}
	}

	// no need to check excludes if not matched
	if !matched {
		return false, nil
	}

	// check excludes conditions
	for path, expectedValue := range filter.Excludes {
		value, found, err := getNestedStringWithJSONPath(obj, path, expectedValue)
		if err != nil {
			return false, fmt.Errorf("Error checking AI filter excludes: expectedValue=%s, error=%v", expectedValue, err)
		}

		if !found {
			continue
		}

		// For array results (comma-separated), check if expectedValue is in any of the values
		values := strings.SplitSeq(value, ", ")
		for v := range values {
			if strings.TrimSpace(v) == expectedValue {
				return false, nil
			}
		}
	}

	return true, nil
}

// BACKGROUND PROCESSING
func (ai *aiManager) Run() {
	// On startup, reset any potentially orphaned in-progress tasks back to pending
	if err := ai.resetInProgressTasksOnStartup(); err != nil {
		ai.logger.Error("Failed resetting in-progress AI tasks on startup", "error", err)
	}

	// Connect to configured MCP servers
	ai.connectMCPServers()

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
	if ai.isTokenLimitExceeded() {
		return
	}

	// Cron runs only on the leading replica — unlike event tasks,
	// their per-run keys are not deduplicated across replicas.
	if includeCron && ai.isLeading != nil && ai.isLeading() {
		ai.processAgentCronTriggers()
	}

	ai.processPendingTasks()

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

func (ai *aiManager) processPendingTasks() {
	ai.pendingTasksLock.Lock()
	defer ai.pendingTasksLock.Unlock()

	stillPending := make(map[string]AiTask)
	for key, task := range ai.pendingTasks {
		shouldCreate, err := ai.shouldCreateNewTask(key)
		if err != nil {
			ai.logger.Error("Error checking if should create new AI task", "error", err)
			continue
		}
		if !shouldCreate {
			continue
		}

		now := time.Now()
		if task.TriggeredBy.For == nil {
			// should never happen due to earlier checks, but just in case
			ai.logger.Error("Pending AI task filter has nil 'For' duration", "key", key, "filter", task.TriggeredBy.Name)
			continue
		}

		createdTime := time.Unix(task.CreatedAt, 0)
		if createdTime.Add(*task.TriggeredBy.For).Before(now) {
			// The duration has elapsed since CreatedAt
			reloadedObject, err := store.GetResource(ai.valkeyClient,
				task.ReferencingResource.ApiVersion,
				task.ReferencingResource.Kind,
				task.ReferencingResource.Namespace,
				task.ReferencingResource.ResourceName,
				ai.logger)
			if err != nil {
				ai.logger.Error("Error reloading object for pending AI task", "key", key, "filter", task.TriggeredBy.Name, "error", err)
				continue
			}
			if reloadedObject == nil {
				ai.logger.Info("Referenced object for pending AI task no longer exists, skipping task creation", "key", key, "filter", task.TriggeredBy.Name)
				continue
			}
			matched, err := filterMatchesForObject(task.TriggeredBy, reloadedObject)
			if err != nil {
				ai.logger.Error("Error checking AI filter match for reloaded object", "filter", task.TriggeredBy.Name, "objectKind", reloadedObject.GetKind(), "objectName", reloadedObject.GetName(), "objectNamespace", reloadedObject.GetNamespace(), "error", err)
				continue
			}
			if matched {
				// create AI task
				ai.insertNewAiTask(&task, reloadedObject, "delayed", key)
			}
		} else {
			stillPending[key] = task
		}
	}
	ai.pendingTasks = stillPending
}

func (ai *aiManager) isTokenLimitExceeded() bool {
	tokenLimit, err := ai.getDailyTokenLimit()
	if err != nil {
		ai.logger.Error("Error getting daily token limit", "error", err)
		ai.setError(err.Error())
		return true
	}

	tokensUsed, _, err := ai.getTodayTokenUsage()
	if err != nil {
		ai.logger.Error("Error getting today's token usage", "error", err)
		ai.setError(err.Error())
		return true
	}

	if tokensUsed >= tokenLimit {
		ai.logger.Warn("Daily AI token limit reached, skipping AI task processing", "tokensUsed", tokensUsed, "dailyLimit", tokenLimit)
		ai.setError(fmt.Errorf("Daily AI token limit reached (%d tokens used of %d). Increase limit or wait 24 hours.", tokensUsed, tokenLimit).Error())
		return true
	} else if tokensUsed >= int64(float64(tokenLimit)*0.8) {
		// warn at 80%
		ai.logger.Warn("Approaching daily AI token limit", "tokensUsed", tokensUsed, "dailyLimit", tokenLimit)
		ai.setWarning(fmt.Sprintf("Approaching daily AI token limit (%d tokens used of %d).", tokensUsed, tokenLimit))
	} else {
		ai.setWarning("")
	}
	return false
}

func (ai *aiManager) tokenLimitErrorMessage() string {
	now := time.Now()
	nextReset := now.Add(24 * time.Hour)
	nextReset = time.Date(nextReset.Year(), nextReset.Month(), nextReset.Day(), 0, 0, 0, 0, nextReset.Location())
	return fmt.Sprintf("The daily token limit for your organization has been exceeded. It will reset on %s at 12:00 AM, or an admin can reset it manually.", nextReset.Format("Jan 2"))
}

func (ai *aiManager) getTodayTokenUsage() (todaysTokens int64, todaysProcessedTasks int, err error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()

	cursor := uint64(0)
	pattern := DB_AI_BUCKET_TOKENS + ":*"
	ctx := context.Background()

	for {
		// Build and execute SCAN command
		scanCmd := ai.valkeyClient.GetValkeyClient().B().Scan().Cursor(cursor).Match(pattern).Count(100).Build()
		scanResult, err := ai.valkeyClient.GetValkeyClient().Do(ctx, scanCmd).AsScanEntry()
		if err != nil {
			return 0, 0, err
		}

		if len(scanResult.Elements) > 0 {
			// Build all GET commands for this batch
			cmds := make([]valkey.Completed, len(scanResult.Elements))
			for i, key := range scanResult.Elements {
				cmds[i] = ai.valkeyClient.GetValkeyClient().B().Get().Key(key).Build()
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

				var tokenEntry UsedToken
				if err := json.Unmarshal([]byte(item), &tokenEntry); err != nil {
					// Log error but continue processing
					continue
				}

				if tokenEntry.Timestamp.Unix() >= startOfDay && !tokenEntry.IsIgnored {
					todaysTokens += tokenEntry.TokensUsed
					todaysProcessedTasks++
				}
			}
		}

		cursor = scanResult.Cursor
		if cursor == 0 {
			break // SCAN complete
		}
	}

	return todaysTokens, todaysProcessedTasks, nil
}

func (ai *aiManager) getDbStats(namespace *string) (totalDbEntries int, unprocessedDbEntries int, ignoredDbEntries int, numberOfUnreadTasks int, err error) {
	key := ai.getValkeyKey("*", "*", "*")
	if namespace != nil {
		key = ai.getValkeyKey("*", *namespace, "*")
	}

	cursor := uint64(0)
	ctx := context.Background()

	for {
		// Build and execute SCAN command
		scanCmd := ai.valkeyClient.GetValkeyClient().B().Scan().Cursor(cursor).Match(key).Count(100).Build()
		scanResult, err := ai.valkeyClient.GetValkeyClient().Do(ctx, scanCmd).AsScanEntry()
		if err != nil {
			return 0, 0, 0, 0, err
		}

		if len(scanResult.Elements) > 0 {
			// Build all GET commands for this batch
			cmds := make([]valkey.Completed, len(scanResult.Elements))
			for i, k := range scanResult.Elements {
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

		cursor = scanResult.Cursor
		if cursor == 0 {
			break // SCAN complete
		}
	}

	return totalDbEntries, unprocessedDbEntries, ignoredDbEntries, numberOfUnreadTasks, nil
}

func (ai *aiManager) addTokenUsage(tokensUsed int, model string, timeUsedInMs int, entryKey string) error {
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
	}

	err := ai.valkeyClient.SetObject(usedToken, ValkeyAiTTL, key)
	if err != nil {
		return fmt.Errorf("error saving AI token usage: %v", err)
	}

	return nil
}

func (ai *aiManager) resetTodayTokenUsage() error {
	// Calculate the start of today in Unix timestamp
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()

	keys, err := ai.valkeyClient.Keys(fmt.Sprintf("%s:*", DB_AI_BUCKET_TOKENS))
	if err != nil {
		return err
	}

	var resettedTokens int64 = 0
	for _, key := range keys {
		item, err := ai.valkeyClient.Get(key)
		if err != nil {
			return err
		}
		var tokenEntry UsedToken
		err = json.Unmarshal([]byte(item), &tokenEntry)
		if err != nil {
			return err
		}
		if tokenEntry.Timestamp.Unix() >= startOfDay {
			resettedTokens += tokenEntry.TokensUsed
			tokenEntry.TokensUsed = 0
			err := ai.valkeyClient.SetObject(tokenEntry, ValkeyAiTTL, key)
			if err != nil {
				return fmt.Errorf("error saving AI token usage: %v", err)
			}

		}
	}
	ai.logger.Info("Reset today's AI token usage", "resettedTokens", resettedTokens)

	ai.resetCache()

	return nil
}

func (ai *aiManager) getAllTaskKeys() ([]string, error) {
	pattern := fmt.Sprintf("%s:*", DB_AI_BUCKET_TASKS)
	cursor := uint64(0)
	ctx := context.Background()
	var allKeys []string

	for {
		scanCmd := ai.valkeyClient.GetValkeyClient().B().Scan().Cursor(cursor).Match(pattern).Count(100).Build()
		scanResult, err := ai.valkeyClient.GetValkeyClient().Do(ctx, scanCmd).AsScanEntry()
		if err != nil {
			return nil, err
		}

		allKeys = append(allKeys, scanResult.Elements...)

		cursor = scanResult.Cursor
		if cursor == 0 {
			break
		}
	}

	return allKeys, nil
}

type aiTaskWithKey struct {
	key  string
	task AiTask
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
		if err == nil && toolCtx != nil && (task.Trigger == "manual" || task.Trigger == "cron") {
			// Whole-scope runs discard advice-only findings at the end — let
			// the model repair them while the conversation is still running.
			toolCtx.RequireActionableFindings = true
		}
		if err != nil {
			task.State = AI_TASK_STATE_IGNORED
			task.Error = err.Error()
			if updateErr := ai.createOrUpdateAiTask(&task, key); updateErr != nil {
				ai.logger.Error("Error updating AI task to ignored state", "taskID", task.ID, "error", updateErr)
			}
			ai.logger.Info("AI task ignored", "taskID", task.ID, "agent", task.AgentRef, "reason", err.Error())
			continue
		}

		if ai.isTokenLimitExceeded() {
			task.State = AI_TASK_STATE_FAILED
			task.Error = "Daily AI token limit exceeded, cannot process further tasks. Increase limit or wait 24 hours."
			err := ai.createOrUpdateAiTask(&task, key)
			if err != nil {
				ai.logger.Error("Error updating AI task", "taskID", task.ID, "error", err)
			}
			continue
		}

		task.State = AI_TASK_STATE_IN_PROGRESS
		task.Error = ""
		err = ai.createOrUpdateAiTask(&task, key)
		if err != nil {
			ai.logger.Error("Failed to set AI task state to in progress", "taskID", task.ID, "error", err)
			continue
		}

		latestTask := &AiTaskLatest{
			Task:   &task,
			Status: ai.GetStatus(nil),
		}

		// send event notification
		ai.sendAiEvent(latestTask)

		// Per-task cancellable context: a cancel marker in Valkey (set by any
		// replica) aborts the LLM loop at the next turn boundary. The same
		// per-turn hook pushes live token counts to the UI, throttled so a
		// fast tool-call storm doesn't flood the event channel.
		taskCtx, cancelTask := context.WithCancel(ctx)
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

		responses, tokensUsed, timeUsedInMs, modelUsed, err := ai.processPrompt(taskCtx, task.Prompt, toolCtx, &agent.Spec, onProgress)
		cancelTask()
		task.CurrentActivity = ""
		discardTask := false
		if err != nil {
			if reason := ai.taskCancelReason(task.ID); errors.Is(err, context.Canceled) && ctx.Err() == nil && reason != "" {
				// Canceled by a user, not by shutdown: the run is void, not broken.
				task.State = AI_TASK_STATE_IGNORED
				task.Error = reason
				ai.clearTaskCancelRequest(task.ID)
				ai.logger.Info("AI task canceled", "taskID", task.ID, "reason", reason)
			} else {
				task.Error = err.Error()
				task.Retries++
				// Non-retryable API errors (billing, invalid request, auth) must not be retried.
				// Mark as ignored so processPendingTasks skips them on the next run.
				var apiErr *anthropic.Error
				if errors.As(err, &apiErr) && (apiErr.StatusCode == 400 || apiErr.StatusCode == 401 || apiErr.StatusCode == 403) {
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
		} else {
			// Whole-scope runs exist to produce applicable changes, not
			// advice: drop findings whose proposal does not survive
			// validation. A task with nothing applicable left disappears
			// entirely — the UI shows its all-clear empty state instead.
			if task.Trigger == "manual" || task.Trigger == "cron" {
				responses = ai.actionableFindings(task.ID, responses)
			}
			if len(responses) == 0 {
				discardTask = true
			} else {
				task.Response = responses[0]
				ai.finalizeTaskOutcome(&task)
				// Every further finding of the run becomes its own review task.
				ai.spawnFindingTasks(&task, responses[1:], modelUsed)
			}
			ai.clearTaskFailureError()
		}
		task.Model = modelUsed
		task.TimeUsedInMs = timeUsedInMs
		task.TokensUsed = tokensUsed
		err = ai.addTokenUsage(int(tokensUsed), modelUsed, timeUsedInMs, key)
		if err != nil {
			ai.logger.Error("Error recording AI token usage", "taskID", task.ID, "error", err)
		}

		// update status for event
		ai.resetCache()
		latestTask.Status = ai.GetStatus(nil)

		if discardTask {
			// Remove the (already visible) in-progress task; the delete event
			// clears it from every client.
			if delErr := ai.valkeyClient.DeleteSingle(key); delErr != nil {
				ai.logger.Error("Error deleting all-clear AI task", "taskID", task.ID, "error", delErr)
			}
			ai.sendAiDeleteEvent(key)
			ai.logger.Info("AI run found nothing applicable — no report created", "taskID", task.ID, "tokensUsed", tokensUsed)
			continue
		}

		// send event notification
		ai.sendAiEvent(latestTask)

		// Save updated task
		err = ai.createOrUpdateAiTask(&task, key)
		if err != nil {
			ai.logger.Error("Error updating AI task", "taskID", task.ID, "error", err)
			continue
		}
		ai.logger.Info("AI task processed", "taskID", task.ID, "tokensUsed", task.TokensUsed, "state", task.State, "name", task.ReferencingResource.ResourceName, "namespace", task.ReferencingResource.Namespace)
	}
}

// HELPER FUNCTIONS
func getNestedStringWithJSONPath(obj *unstructured.Unstructured, path string, keyword string) (value string, found bool, err error) {
	j := jsonpath.New("parser")
	j.AllowMissingKeys(true)

	// JSONPath expects the path to start with {}, e.g., {.status.conditions[?(@.type=="Ready")].status}
	if !strings.HasPrefix(path, "{") {
		path = "{" + path + "}"
	}

	// JSONPath expects double quotes instead of single quotes
	path = strings.ReplaceAll(path, "'", "\"")

	if err := j.Parse(path); err != nil {
		return "", false, fmt.Errorf("failed to parse JSONPath: %w", err)
	}

	results, err := j.FindResults(obj.Object)
	if err != nil {
		// Handle array out of bounds as "not found" instead of error
		if strings.Contains(err.Error(), "array index out of bounds") {
			return "", false, nil
		}
		return "", false, fmt.Errorf("failed to find results: %w", err)
	}

	if len(results) == 0 || len(results[0]) == 0 {
		return "", false, nil
	}

	// Handle multiple results (when using [*] or filters that return multiple items)
	var allValues []string
	for _, resultArray := range results {
		for _, result := range resultArray {
			val := result.Interface()

			// Handle string result
			if str, ok := val.(string); ok {
				allValues = append(allValues, str)
				continue
			}

			// Handle map (for keyword search)
			if labelMap, ok := val.(map[string]any); ok {
				var matches []string
				for key, value := range labelMap {
					valueStr := fmt.Sprintf("%v", value)
					if keyword == "" {
						// If no keyword, return all key=value pairs
						matches = append(matches, fmt.Sprintf("%s=%s", key, valueStr))
					} else if strings.Contains(strings.ToLower(key), strings.ToLower(keyword)) ||
						strings.Contains(strings.ToLower(valueStr), strings.ToLower(keyword)) {
						matches = append(matches, fmt.Sprintf("%s=%s", key, valueStr))
					}
				}

				if len(matches) > 0 {
					sort.Strings(matches)
					allValues = append(allValues, strings.Join(matches, ", "))
				}
				continue
			}

			// Handle other types (numbers, booleans, etc.)
			allValues = append(allValues, fmt.Sprintf("%v", val))
		}
	}

	if len(allValues) == 0 {
		return "", false, nil
	}

	// Join all values with a delimiter
	return strings.Join(allValues, ", "), true, nil
}

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
	parts := strings.Split(key, ":")
	if len(parts) > 2 {
		namespace := parts[2]
		err = ai.valkeyClient.Set(string(jsonString), ValkeyAiTTL, ai.getValkeyLatestNamespaceTaskKey(namespace))
		if err != nil {
			ai.logger.Warn("Error saving AI task for namespace", "namespace", namespace, "error", err)
		}
	}

	return nil
}

func (ai *aiManager) shouldCreateNewTask(key string) (bool, error) {
	exists, err := ai.valkeyClient.Exists(key)
	if err != nil {
		return false, fmt.Errorf("error checking if AI task exists: %v", err)
	}
	return !exists, nil
}

// processPrompt runs one unattended analysis. The ToolContext scopes every
// tool call to the owning agent's namespaces with the viewer role; the agent
// spec contributes its instruction and optional model override.
// finalAnswerNudge is sent when an unattended run exhausts its tool-call
// budget: one last turn with tool use disabled so the model must produce the
// required JSON verdict instead of the run failing outright.
const finalAnswerNudge = "Your budget for this run is exhausted — do not request any more inspection tools. Call " + submitAnalysisToolName + " now with all remaining findings. Every finding must carry an applicable proposal: proposedOperation (UpdateResource, DeleteResource or CreateResource) plus the exact live targetResource — findings without one are discarded. Submit an empty findings array if nothing applicable remains."

// processPrompt runs the unattended analysis loop. onProgress (nil-tolerant)
// is invoked after every LLM turn with the tokens used so far and on every
// tool call with a human-readable activity line — it powers the live token
// counter and "currently working on" display in the UI plus the cancel check.
func (ai *aiManager) processPrompt(ctx context.Context, prompt string, toolCtx *ToolContext, agentSpec *v1alpha1.AgentSpec, onProgress func(tokensUsed int64, activity string)) (responses []*AiResponse, tokensUsed int64, timeUsedInMs int, modelUser string, err error) {
	startTime := time.Now()
	// Resolve the full model config (provider, endpoint, credentials, limits)
	// once for the whole run: the agent's modelRef, the default AiModel or the
	// legacy secret — see resolveModelConfig.
	rc, err := ai.resolveModelConfig(agentSpec)
	if err != nil {
		return nil, 0, int(time.Since(startTime).Milliseconds()), "", err
	}
	systemPrompt := ai.getSystemPrompt()
	if agentSpec != nil {
		if agentSpec.Instruction != "" {
			systemPrompt += "\n\nAgent instruction:\n" + agentSpec.Instruction
		}
		// The proposal review UI is decision-style: a short explanation above
		// a YAML diff. Without the structured proposal fields the task stays a
		// text-only report, so spell out exactly what a proposal requires.
		systemPrompt += "\n\nOutput style: write problemDescription as 2-4 crisp sentences a DevOps decision maker can act on — what is wrong, what your proposed change does, and any risk." +
			"\n\nWhen you recommend a concrete change, do NOT only describe it in prose — emit it as a structured proposal in the analysis:" +
			"\n- proposedOperation: one of UpdateResource, DeleteResource, CreateResource" +
			"\n- targetResource: apiVersion, kind, namespace, resourceName, plural and namespaced of the affected resource (always required)" +
			"\n- targetResourceYaml: the complete resource manifest, based on the live manifest you retrieved from the cluster with ONLY the fields the fix requires changed — never invent values, never include server-managed fields (metadata.resourceVersion, uid, creationTimestamp, generation, managedFields, status); required for UpdateResource and CreateResource, omit for DeleteResource" +
			"\n- additionalTargets: for a DeleteResource that removes many similar resources at once, list every extra resource here (first one stays in targetResource). Prefer one bulk delete over many findings, and enumerate them all." +
			"\nOnly a structured proposal can be reviewed and applied with one click; prose-only recommendations end up as plain reports."
	}

	switch rc.Sdk {
	case AiSdkTypeOpenAI:
		return ai.processPromptOpenAi(ctx, rc, systemPrompt, prompt, toolCtx, onProgress)
	case AiSdkTypeAnthropic:
		return ai.processPromptAnthropic(ctx, rc, systemPrompt, prompt, toolCtx, onProgress)
	case AiSdkTypeOllama:
		return ai.processPromptOllama(ctx, rc, systemPrompt, prompt, toolCtx, onProgress)
	default:
		return nil, 0, int(time.Since(startTime).Milliseconds()), rc.Model, fmt.Errorf("unsupported AI SDK type: %s", rc.Sdk)
	}
}

// finalizeTaskOutcome decides whether a successfully analyzed task is a mere
// analysis ("completed") or an actionable proposal ("proposed") awaiting user
// approval, and captures the target's resourceVersion for the staleness guard.
func (ai *aiManager) finalizeTaskOutcome(task *AiTask) {
	task.State = AI_TASK_STATE_COMPLETED
	if task.Response == nil {
		return
	}

	analysis := task.Response.Analysis
	target := analysis.TargetResource
	switch analysis.ProposedOperation {
	case ProposedOperationUpdate:
		if analysis.TargetResourceYaml == "" || target.ResourceName == "" {
			return
		}
		current, err := store.GetResource(ai.valkeyClient, target.ApiVersion, target.Kind, target.Namespace, target.ResourceName, ai.logger)
		if err != nil || current == nil {
			// Target vanished — nothing left to apply, keep it as analysis.
			return
		}
		task.BaseResourceVersion = current.GetResourceVersion()
		ai.captureCurrentResourceYaml(task, current)
		ai.sanitizeTargetResourceYaml(task)
		task.State = AI_TASK_STATE_PROPOSED
	case ProposedOperationDelete:
		if target.ResourceName == "" {
			return
		}
		current, err := store.GetResource(ai.valkeyClient, target.ApiVersion, target.Kind, target.Namespace, target.ResourceName, ai.logger)
		if err != nil || current == nil {
			return
		}
		task.BaseResourceVersion = current.GetResourceVersion()
		ai.captureCurrentResourceYaml(task, current)
		task.State = AI_TASK_STATE_PROPOSED
	case ProposedOperationCreate:
		if analysis.TargetResourceYaml == "" {
			return
		}
		// Nothing exists yet — the diff renders against an empty document.
		task.Response.Analysis.CurrentResourceYaml = ""
		ai.sanitizeTargetResourceYaml(task)
		task.State = AI_TASK_STATE_PROPOSED
	}
}

// findingRejectionReason explains why a finding would not survive proposal
// validation, or returns "" when it is actionable. Mirrors the rules of
// finalizeTaskOutcome so submit-time feedback matches the final filter.
func (ai *aiManager) findingRejectionReason(response *AiResponse) string {
	if response == nil {
		return "empty finding"
	}
	analysis := response.Analysis
	target := analysis.TargetResource
	switch analysis.ProposedOperation {
	case ProposedOperationUpdate, ProposedOperationCreate:
		if analysis.TargetResourceYaml == "" {
			return analysis.ProposedOperation + " requires targetResourceYaml (the complete proposed manifest)"
		}
		if analysis.ProposedOperation == ProposedOperationUpdate {
			if target.ResourceName == "" {
				return "UpdateResource requires targetResource.resourceName"
			}
			if current, err := store.GetResource(ai.valkeyClient, target.ApiVersion, target.Kind, target.Namespace, target.ResourceName, ai.logger); err != nil || current == nil {
				return fmt.Sprintf("target %s %q not found in namespace %q — use the exact apiVersion, kind, namespace and name of a live resource", target.Kind, target.ResourceName, target.Namespace)
			}
		}
		return ""
	case ProposedOperationDelete:
		if target.ResourceName == "" {
			return "DeleteResource requires targetResource.resourceName"
		}
		if current, err := store.GetResource(ai.valkeyClient, target.ApiVersion, target.Kind, target.Namespace, target.ResourceName, ai.logger); err != nil || current == nil {
			return fmt.Sprintf("target %s %q not found in namespace %q — use the exact apiVersion, kind, namespace and name of a live resource", target.Kind, target.ResourceName, target.Namespace)
		}
		return ""
	default:
		return "no applicable proposedOperation — set it to UpdateResource, DeleteResource or CreateResource with the matching targetResource"
	}
}

// actionableFindings keeps only findings whose proposal survives validation
// (concrete operation, target exists, manifest complete). Whole-scope runs
// exist to produce applicable changes — advice-only findings are dropped and
// logged with their headline so they don't vanish silently.
func (ai *aiManager) actionableFindings(taskID string, responses []*AiResponse) []*AiResponse {
	kept := make([]*AiResponse, 0, len(responses))
	for _, response := range responses {
		probe := AiTask{ID: taskID, Response: response, State: AI_TASK_STATE_COMPLETED}
		ai.finalizeTaskOutcome(&probe)
		if probe.State == AI_TASK_STATE_PROPOSED {
			kept = append(kept, response)
		} else {
			ai.logger.Info("Dropping advice-only finding from run", "taskID", taskID, "headline", response.ErrorMessage, "reason", ai.findingRejectionReason(response))
		}
	}
	return kept
}

// spawnFindingTasks persists every finding beyond the first as its own task,
// so each one can be reviewed, approved and audited independently. All
// findings share the run's single exploration — the token cost is booked on
// the primary task only.
func (ai *aiManager) spawnFindingTasks(primary *AiTask, extra []*AiResponse, model string) {
	if len(extra) > 0 {
		// Group the run's tasks so the UI can render them as one report.
		primary.RunID = primary.ID
	}
	for i, response := range extra {
		now := time.Now().Unix()
		finding := AiTask{
			ID:                  fmt.Sprintf("%s-f%d", primary.ID, i+2),
			RunID:               primary.ID,
			Prompt:              primary.Prompt,
			Response:            response,
			State:               AI_TASK_STATE_COMPLETED,
			Model:               model,
			CreatedAt:           now,
			UpdatedAt:           now,
			ReferencingResource: primary.ReferencingResource,
			TriggeredBy:         primary.TriggeredBy,
			AgentRef:            primary.AgentRef,
			Trigger:             primary.Trigger,
			TriggeredByUser:     primary.TriggeredByUser,
		}
		ai.finalizeTaskOutcome(&finding)
		if err := ai.createOrUpdateAiTask(&finding, finding.ID); err != nil {
			ai.logger.Error("Error persisting finding task", "taskID", finding.ID, "error", err)
			continue
		}
		ai.notifyTaskChanged(&finding)
	}
	if len(extra) > 0 {
		ai.logger.Info("Run produced additional findings", "primaryTaskID", primary.ID, "additionalTasks", len(extra))
	}
}

// captureCurrentResourceYaml replaces the model-provided (untrusted, possibly
// truncated or hallucinated) CurrentResourceYaml with the authoritative state
// of the target at proposal time, so the review diff in the UI is exact.
func (ai *aiManager) captureCurrentResourceYaml(task *AiTask, current *unstructured.Unstructured) {
	sanitized := current.DeepCopy()
	stripServerManagedFields(sanitized)
	yaml, err := store.GetYamlFromUnstructuredResource(sanitized)
	if err != nil {
		ai.logger.Warn("Failed to serialize current resource for proposal diff", "taskID", task.ID, "error", err)
		return // keep the model-provided value as a fallback
	}
	task.Response.Analysis.CurrentResourceYaml = yaml
}

// sanitizeTargetResourceYaml strips server-managed fields the model tends to
// hallucinate (fabricated resourceVersion/uid/creationTimestamp/...) from the
// proposed manifest. They are pure noise in the review diff and would break
// the apply: a made-up resourceVersion causes an update conflict, a wrong uid
// a rejection. Malformed YAML is left untouched — the execution path reports
// the parse error to the user.
func (ai *aiManager) sanitizeTargetResourceYaml(task *AiTask) {
	obj, err := parseTargetYaml(task.Response.Analysis.TargetResourceYaml)
	if err != nil {
		return
	}
	stripServerManagedFields(obj)
	yaml, err := store.GetYamlFromUnstructuredResource(obj)
	if err != nil {
		return
	}
	task.Response.Analysis.TargetResourceYaml = yaml
}

// stripServerManagedFields removes fields owned by the API server or
// controllers from a manifest; applied to both sides of the proposal diff so
// it only shows changes a user could actually make.
func stripServerManagedFields(obj *unstructured.Unstructured) {
	unstructured.RemoveNestedField(obj.Object, "status")
	unstructured.RemoveNestedField(obj.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(obj.Object, "metadata", "uid")
	unstructured.RemoveNestedField(obj.Object, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(obj.Object, "metadata", "generation")
	unstructured.RemoveNestedField(obj.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(obj.Object, "metadata", "selfLink")
	if annotations := obj.GetAnnotations(); annotations != nil {
		delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
		delete(annotations, "deployment.kubernetes.io/revision")
		if len(annotations) == 0 {
			unstructured.RemoveNestedField(obj.Object, "metadata", "annotations")
		} else {
			obj.SetAnnotations(annotations)
		}
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

// for nasty AIs which return markdown code blocks or extra text around JSON
func cleanJSONResponse(response string) string {
	// Trim whitespace
	response = strings.TrimSpace(response)

	// Remove markdown code blocks (```json or ``` at start/end)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")

	// Trim again after removing code blocks
	return strings.TrimSpace(response)
}

func extractJSONRobust(text string) (jsonData []byte, removedText string, err error) {
	start := strings.Index(text, "{")
	if start == -1 {
		return nil, "", fmt.Errorf("no JSON object found")
	}

	// Capture the text that was removed (the "bullshit")
	removedText = text[:start]

	braceCount := 0
	inString := false
	escapeNext := false

	for i := start; i < len(text); i++ {
		char := text[i]

		if escapeNext {
			escapeNext = false
			continue
		}

		if char == '\\' {
			escapeNext = true
			continue
		}

		if char == '"' {
			inString = !inString
			continue
		}

		if !inString {
			switch char {
			case '{':
				braceCount++
			case '}':
				braceCount--
				if braceCount == 0 {
					return []byte(text[start : i+1]), removedText, nil
				}
			}
		}
	}

	return nil, removedText, fmt.Errorf("unbalanced braces in JSON")
}

func buildUserPrompt(prompt string, obj *unstructured.Unstructured) string {
	objBytes, err := yaml.Marshal(obj.Object)
	if err != nil {
		return fmt.Sprintf("%s\n\nError serializing Kubernetes object: %v", prompt, err)
	}
	return fmt.Sprintf("%s\n\nHere are the related Kubernetes resources in yaml format:\n%s", prompt, string(objBytes))
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
	return api.NewClient(url, http.DefaultClient), nil
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
