package ai

import (
	"context"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// task trigger kinds
const (
	AI_TASK_TRIGGER_EVENT  = "event"
	AI_TASK_TRIGGER_CRON   = "cron"
	AI_TASK_TRIGGER_MANUAL = "manual"
)

// validChangeEventTypes are the change signals a change trigger may react to.
var validChangeEventTypes = map[string]bool{"created": true, "updated": true, "deleted": true}

// defaultChangeCooldown bounds change-triggered runs when the agent does not
// set MinInterval — the rate limit that keeps a burst of changes from starting
// many runs.
const defaultChangeCooldown = 6 * time.Hour

// agentCacheTTL bounds how long getEnabledAgents may serve its cached agent
// list without re-reading the store. Agent CR changes invalidate the cache
// immediately via ProcessObject; the TTL only covers events the watcher
// missed, so it can stay short without re-creating per-event store reads.
const agentCacheTTL = 30 * time.Second

// ValidateAgentSpec checks an agent spec for the invariants the pipeline
// relies on: a non-empty scope (an agent without scope restrictions must not
// exist — empty allow-maps would disable namespace checks entirely), a
// parseable cron expression and a well-formed change trigger.
func ValidateAgentSpec(spec v1alpha1.AgentSpec) error {
	if spec.Scope.WorkspaceRef == "" && len(spec.Scope.Namespaces) == 0 {
		return fmt.Errorf("agent scope must reference a workspace or list at least one namespace")
	}
	for _, ns := range spec.Scope.Namespaces {
		if strings.TrimSpace(ns) == "" {
			return fmt.Errorf("agent scope contains an empty namespace entry")
		}
	}
	if spec.Triggers.Cron != "" {
		if _, err := cron.ParseStandard(spec.Triggers.Cron); err != nil {
			return fmt.Errorf("invalid cron expression %q: %w", spec.Triggers.Cron, err)
		}
	}
	if oc := spec.Triggers.OnChange; oc != nil {
		for _, evt := range oc.On {
			if !validChangeEventTypes[evt] {
				return fmt.Errorf("onChange.on contains invalid change type %q (allowed: created, updated, deleted)", evt)
			}
		}
	}
	return nil
}

// changeCooldown returns the effective cooldown for an agent's change trigger.
func changeCooldown(oc *v1alpha1.AgentChangeTrigger) time.Duration {
	if oc != nil && oc.MinInterval.Duration > 0 {
		return oc.MinInterval.Duration
	}
	return defaultChangeCooldown
}

// getEnabledAgents returns all enabled agents from the operator namespace.
//
// The result is cached: this runs on EVERY watch event in the cluster (via
// ProcessObject → triggerChangeAgents), and reading the store each time meant
// a full-keyspace SCAN per event — enough sustained load to saturate Valkey
// on busy clusters (MOG-4518). Agent CR events invalidate the cache directly,
// agentCacheTTL is the safety net. Callers must not mutate the returned slice.
//
// While one goroutine refreshes, concurrent callers get the previous list
// (possibly nil right after startup) instead of blocking: this is called
// synchronously from informer handlers, and a slow store read here would
// stall watch-event processing — during a Valkey degradation every event
// would otherwise wait out the full client timeout.
func (ai *aiManager) getEnabledAgents() []v1alpha1.Agent {
	ai.agentCacheMu.Lock()
	if ai.agentCacheFetching || (ai.agentCacheValid && time.Since(ai.agentCacheTime) < agentCacheTTL) {
		agents := ai.cachedEnabledAgents
		ai.agentCacheMu.Unlock()
		return agents
	}
	generation := ai.agentCacheGen
	ai.agentCacheFetching = true
	ai.agentCacheMu.Unlock()

	enabled := ai.fetchEnabledAgents()

	ai.agentCacheMu.Lock()
	ai.agentCacheFetching = false
	// An invalidation while we were reading means `enabled` may predate that
	// change — serve it once but leave the cache invalid so the next call
	// re-reads.
	if ai.agentCacheGen == generation {
		ai.cachedEnabledAgents = enabled
		ai.agentCacheTime = time.Now()
		ai.agentCacheValid = true
	}
	ai.agentCacheMu.Unlock()
	return enabled
}

// invalidateAgentCache marks the enabled-agents cache stale. Called for every
// Agent CR watch event, so cache staleness is bounded by informer latency
// rather than agentCacheTTL.
func (ai *aiManager) invalidateAgentCache() {
	ai.agentCacheMu.Lock()
	ai.agentCacheGen++
	ai.agentCacheValid = false
	ai.agentCacheMu.Unlock()
}

// fetchEnabledAgents reads the enabled agents from the store. Failures are
// cached like results: during a store outage, retrying on every watch event
// would amplify the outage (each retry is a full-keyspace SCAN against an
// already unresponsive Valkey).
func (ai *aiManager) fetchEnabledAgents() []v1alpha1.Agent {
	ownNamespace, err := ai.config.TryGet("MO_OWN_NAMESPACE")
	if err != nil {
		ai.logger.Warn("getEnabledAgents: failed to get own namespace", "error", err)
		return nil
	}
	agents, err := store.GetAllAgents(ownNamespace)
	if err != nil {
		ai.logger.Warn("getEnabledAgents: failed to list agents", "error", err)
		return nil
	}
	enabled := make([]v1alpha1.Agent, 0, len(agents))
	for _, agent := range agents {
		if agent.Spec.Enabled {
			enabled = append(enabled, agent)
		}
	}
	return enabled
}

func (ai *aiManager) getAgent(name string) (*v1alpha1.Agent, error) {
	ownNamespace, err := ai.config.TryGet("MO_OWN_NAMESPACE")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve own namespace: %w", err)
	}
	agent, err := store.GetAgent(ownNamespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent %q: %w", name, err)
	}
	if agent == nil {
		return nil, fmt.Errorf("agent %q not found", name)
	}
	return agent, nil
}

// resolveAgentScope resolves the agent's scope to a sorted, deduplicated list
// of namespaces. WorkspaceRef contributes the workspace's namespace resources,
// Namespaces contributes verbatim entries; both are unioned. The wildcard
// entry "*" expands to every namespace currently known to the store — the
// ToolContext still gets an explicit allow-map, never an unrestricted one.
func (ai *aiManager) resolveAgentScope(agent *v1alpha1.Agent) []string {
	namespaces := map[string]bool{}

	for _, ns := range agent.Spec.Scope.Namespaces {
		if ns == "*" {
			for _, nsObj := range store.GetResourceByKindAndNamespace(ai.valkeyClient, utils.NamespaceResource.ApiVersion, utils.NamespaceResource.Kind, "", ai.logger) {
				if name := nsObj.GetName(); name != "" {
					namespaces[name] = true
				}
			}
			continue
		}
		if ns != "" {
			namespaces[ns] = true
		}
	}

	if agent.Spec.Scope.WorkspaceRef != "" {
		ownNamespace, err := ai.config.TryGet("MO_OWN_NAMESPACE")
		if err != nil {
			ai.logger.Warn("resolveAgentScope: failed to get own namespace", "agent", agent.Name, "error", err)
		} else {
			workspace, err := store.GetWorkspace(ownNamespace, agent.Spec.Scope.WorkspaceRef)
			if err != nil || workspace == nil {
				ai.logger.Warn("resolveAgentScope: workspace not found", "agent", agent.Name, "workspace", agent.Spec.Scope.WorkspaceRef, "error", err)
			} else {
				for _, res := range workspace.Spec.Resources {
					if res.Type == "namespace" && res.Id != "" {
						namespaces[res.Id] = true
					}
				}
			}
		}
	}

	result := make([]string, 0, len(namespaces))
	for ns := range namespaces {
		result = append(result, ns)
	}
	sort.Strings(result)
	return result
}

// buildAgentRunPrompt is the user prompt for scheduled/manual whole-scope runs.
// The agent's instruction (if any) is the primary task; namespace context is
// prepended so the model knows its operational scope.
func buildAgentRunPrompt(agent *v1alpha1.Agent, namespaces []string) string {
	nsStr := strings.Join(namespaces, ", ")

	var sb strings.Builder
	sb.WriteString("Your scope for this run includes the following Kubernetes namespaces: ")
	sb.WriteString(nsStr)
	sb.WriteString(". You operate in read-only mode by default; any mutation requires explicit approval.")

	if agent.Spec.Instruction != "" {
		sb.WriteString("\n\n")
		sb.WriteString(agent.Spec.Instruction)
		return sb.String()
	}

	// No custom instruction: apply the built-in K8s analysis framing.
	sb.WriteString("\n\nInspect the workloads in these namespaces with your tools and address every distinct issue you find. When a fix requires a resource change, call the appropriate mutation tool (update_kubernetes_resource, create_kubernetes_resource, or delete_kubernetes_resource) — each call is intercepted and surfaced as an approval request for the operator. Be efficient — you have a limited tool-call and token budget: list resources cluster-wide (omit the namespace parameter) instead of namespace by namespace, inspect suspicious candidates with get detail=summary, and fetch the full manifest only when you need it to build an update proposal.")
	sb.WriteString("\n\nOnly report findings you can back with a concrete, safe, directly applicable remediation: a proposed operation plus the complete target resource YAML, based on the live manifest you retrieved. Advice-only findings without an applicable change are discarded — do not report them. If nothing needs fixing, submit an empty findings list; that is a perfectly good result.")
	sb.WriteString("\n\nWhen many similar resources should be deleted (e.g. dozens of completed Jobs or obsolete zero-replica ReplicaSets), do NOT summarize them in prose and do NOT emit one finding per resource. Emit a SINGLE DeleteResource finding that lists ALL of them: put the first in targetResource and every other one in additionalTargets. Enumerate them completely — list every matching resource you found, not just examples.")
	return sb.String()
}

// agentRunKeyPrefix returns the Valkey key prefix for whole-scope runs of an
// agent. The namespace segment is only a storage detail (kept so the key
// shape stays uniform); workspace visibility is decided by
// agentTaskVisibleInNamespaces, not by this segment.
func agentRunKeyPrefix(namespace, agentName string) string {
	return fmt.Sprintf("%s:Agent:%s:%s-run-", DB_AI_BUCKET_TASKS, namespace, agentName)
}

// agentTaskIDPrefix is the ID/key prefix shared by all whole-scope agent
// tasks (runs and their findings).
var agentTaskIDPrefix = fmt.Sprintf("%s:Agent:", DB_AI_BUCKET_TASKS)

// isAgentTaskID reports whether a task ID (== storage key) belongs to a
// whole-scope agent run or one of its findings, as opposed to an
// event-triggered single-resource task (ai_tasks:<Kind>:<ns>:<name>).
func isAgentTaskID(id string) bool {
	return strings.HasPrefix(id, agentTaskIDPrefix)
}

// agentTaskKeyForNamespace re-homes an agent task key to the given namespace
// segment (ai_tasks:Agent:<ns>:<rest>). Findings are stored under their
// target resource's namespace so the key states where the finding belongs;
// keys of other shapes and empty namespaces are returned unchanged.
func agentTaskKeyForNamespace(key, namespace string) string {
	parts := strings.SplitN(key, ":", 4)
	if namespace == "" || len(parts) != 4 || parts[1] != "Agent" {
		return key
	}
	parts[2] = namespace
	return strings.Join(parts, ":")
}

// agentTaskVisibleInNamespaces decides whether a whole-scope agent task is
// visible to a workspace holding the given namespaces. A finding belongs to
// its target resource's namespace — a workspace only sees findings about its
// own namespaces, never manifests of foreign ones. Tasks without a finding
// (all-clear reports, queued/running/failed runs) and findings on
// cluster-scoped targets are visible wherever the agent's scope overlaps the
// workspace; a wildcard scope is visible everywhere. Legacy tasks that
// predate ScopeNamespaces fall back to the storage key's namespace segment
// (the old behavior).
func agentTaskVisibleInNamespaces(task *AiTask, namespaces map[string]bool) bool {
	if task.ScopeAllNamespaces {
		return true
	}
	if len(task.ScopeNamespaces) > 0 {
		for _, ns := range task.ScopeNamespaces {
			if namespaces[ns] {
				return true
			}
		}
		return false
	}
	parts := strings.SplitN(task.ID, ":", 4)
	return len(parts) == 4 && namespaces[parts[2]]
}

// hasOpenAgentRun reports whether the agent already has a pending, in-progress
// or cancelling whole-scope run, bounding cron/manual fan-out to one open run.
func (ai *aiManager) hasOpenAgentRun(agentName string) (bool, error) {
	keys, err := ai.valkeyClient.Keys(fmt.Sprintf("%s:Agent:*:%s-run-*", DB_AI_BUCKET_TASKS, agentName))
	if err != nil {
		return false, err
	}
	for _, key := range keys {
		task, err := ai.getTaskByKey(key)
		if err != nil || task == nil {
			continue
		}
		if task.State == AI_TASK_STATE_PENDING || task.State == AI_TASK_STATE_IN_PROGRESS || task.State == AI_TASK_STATE_CANCELLING {
			return true, nil
		}
	}
	return false, nil
}

// createAgentRunTask enqueues a whole-scope run for the agent. It is picked up
// by the regular task queue on the next ticker.
func (ai *aiManager) createAgentRunTask(agent *v1alpha1.Agent, trigger string, triggeredBy *structs.User) (*AiTask, error) {
	namespaces := ai.resolveAgentScope(agent)
	if len(namespaces) == 0 {
		return nil, fmt.Errorf("agent %q has no resolvable scope namespaces", agent.Name)
	}

	open, err := ai.hasOpenAgentRun(agent.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check open runs for agent %q: %w", agent.Name, err)
	}
	if open {
		return nil, fmt.Errorf("agent %q already has a pending or in-progress run", agent.Name)
	}

	timestamp := time.Now().Unix()
	key := fmt.Sprintf("%s%d", agentRunKeyPrefix(namespaces[0], agent.Name), timestamp)
	task := &AiTask{
		ID:              key,
		Prompt:          buildAgentRunPrompt(agent, namespaces),
		State:           AI_TASK_STATE_PENDING,
		CreatedAt:       timestamp,
		UpdatedAt:       timestamp,
		AgentRef:        agent.Name,
		Trigger:         trigger,
		TriggeredByUser: triggeredBy,
		// Visibility must not depend on the key's namespace segment (an
		// arbitrary pick — the alphabetically first scope namespace), so the
		// scope travels with the task.
		ScopeNamespaces:    namespaces,
		ScopeAllNamespaces: slices.Contains(agent.Spec.Scope.Namespaces, "*"),
	}
	if err := ai.createOrUpdateAiTask(task, key); err != nil {
		return nil, fmt.Errorf("failed to create agent run task: %w", err)
	}
	ai.cronStateLock.Lock()
	ai.lastAgentRun[agent.Name] = time.Now()
	ai.cronStateLock.Unlock()
	ai.logger.Info("Agent run task created", "agent", agent.Name, "trigger", trigger, "taskID", task.ID)
	return task, nil
}

// TriggerAgent creates a manual whole-scope run for the agent. A manual run is
// always available for an enabled agent — the caller (annotation reconcile or,
// historically, the UI) has already established intent.
func (ai *aiManager) TriggerAgent(agentName string, user *structs.User) (*AiTask, error) {
	agent, err := ai.getAgent(agentName)
	if err != nil {
		return nil, err
	}
	if !agent.Spec.Enabled {
		return nil, fmt.Errorf("agent %q is disabled", agentName)
	}
	return ai.createAgentRunTask(agent, AI_TASK_TRIGGER_MANUAL, user)
}

// processAgentCronTriggers evaluates all enabled agents' cron schedules and
// enqueues a run for every agent whose schedule fired since its last run.
// Called from the minute ticker on the leading replica only.
func (ai *aiManager) processAgentCronTriggers() {
	now := time.Now()
	for _, agent := range ai.getEnabledAgents() {
		if agent.Spec.Triggers.Cron == "" {
			continue
		}
		schedule, err := cron.ParseStandard(agent.Spec.Triggers.Cron)
		if err != nil {
			ai.logger.Warn("Skipping agent with invalid cron expression", "agent", agent.Name, "cron", agent.Spec.Triggers.Cron, "error", err)
			continue
		}

		ai.cronStateLock.Lock()
		lastRun, seen := ai.lastCronRun[agent.Name]
		if !seen {
			// First sighting after startup: anchor to now so we don't
			// immediately fire for schedules that elapsed while down.
			ai.lastCronRun[agent.Name] = now
			ai.cronStateLock.Unlock()
			continue
		}
		due := schedule.Next(lastRun)
		fire := !due.After(now)
		if fire {
			ai.lastCronRun[agent.Name] = now
		}
		ai.cronStateLock.Unlock()

		if !fire {
			continue
		}
		if _, err := ai.createAgentRunTask(&agent, AI_TASK_TRIGGER_CRON, nil); err != nil {
			ai.logger.Warn("Failed to enqueue cron run for agent", "agent", agent.Name, "error", err)
		}
	}
}

// buildAgentTaskContext resolves the agent and its ToolContext for a queued
// task. Returns an error when the task must not run (agent deleted, disabled,
// or scope empty).
func (ai *aiManager) buildAgentTaskContext(task *AiTask) (*v1alpha1.Agent, *ToolContext, error) {
	if task.AgentRef == "" {
		return nil, nil, fmt.Errorf("task has no agent reference (created by a previous operator version)")
	}
	agent, err := ai.getAgent(task.AgentRef)
	if err != nil {
		return nil, nil, err
	}
	if !agent.Spec.Enabled {
		return nil, nil, fmt.Errorf("agent %q is disabled", agent.Name)
	}
	namespaces := ai.resolveAgentScope(agent)
	if len(namespaces) == 0 {
		return nil, nil, fmt.Errorf("agent %q has no resolvable scope namespaces", agent.Name)
	}
	// Event tasks must still be inside the (possibly changed) scope.
	if taskNamespace := task.ReferencingResource.Namespace; taskNamespace != "" {
		inScope := slices.Contains(namespaces, taskNamespace)
		if !inScope {
			return nil, nil, fmt.Errorf("resource namespace %q is no longer in the scope of agent %q", taskNamespace, agent.Name)
		}
	}
	toolCtx := newToolContextFromAgent(agent, namespaces)
	toolCtx.McpSessions = agent.Spec.Tools.McpServerRefs
	if b := agent.Spec.Tools.Builtin; b != nil {
		toolCtx.DisableKubernetes = !b.Kubernetes
		toolCtx.DisableHelm = !b.Helm
	}

	// Wire the approval-request callback. When a mutating tool call is
	// intercepted, the run task itself is flipped to PROPOSED so the UI can
	// surface the pending tool call to the user. Once approved the run task
	// is reset to IN_PROGRESS and execution continues.
	runTaskID := task.ID
	mcpSessions := toolCtx.McpSessions
	toolCtx.CreateApprovalRequest = func(ctx context.Context, toolName string, args map[string]any) (*ToolContext, error) {
		// Load the run task and set it to PROPOSED with the pending tool call
		// info so the UI can show what needs to be approved.
		runTask, err := ai.getTaskByKey(runTaskID)
		if err != nil || runTask == nil {
			return nil, fmt.Errorf("failed to load run task for approval: %w", err)
		}
		runTask.State = AI_TASK_STATE_PROPOSED
		runTask.Response = &AiResponse{
			ToolRequests: []ToolRequest{{
				Name:     toolName,
				Args:     args,
				Sessions: mcpSessions,
			}},
		}
		if err := ai.createOrUpdateAiTask(runTask, runTaskID); err != nil {
			return nil, fmt.Errorf("set run task to proposed: %w", err)
		}
		ai.notifyTaskChanged(runTask)

		// Register a channel so ApproveTask/RejectTask can wake us up.
		ch := make(chan approvalResult, 1)
		ai.pendingApprovalsMu.Lock()
		ai.pendingApprovals[runTaskID] = ch
		ai.pendingApprovalsMu.Unlock()
		defer func() {
			ai.pendingApprovalsMu.Lock()
			delete(ai.pendingApprovals, runTaskID)
			ai.pendingApprovalsMu.Unlock()
		}()

		// buildApproverCtx copies the agent's namespace scope but replaces
		// the user and role with those of the approver so the tool executes
		// with their identity and permissions.
		buildApproverCtx := func(approver structs.User) *ToolContext {
			c := *toolCtx
			c.CreateApprovalRequest = nil
			c.User = &approver
			if toolCtx.Workspace != "" {
				_, grant := ai.ResolveWorkspaceContext(approver.Email, toolCtx.Workspace)
				if grant != nil {
					c.Role = grant.Role
				}
			}
			return &c
		}

		// resetToInProgress reads the latest run task from Valkey (which
		// carries the Approval record written by ApproveTask) and resets the
		// state to IN_PROGRESS so the agent loop can continue.
		resetToInProgress := func() {
			current, err := ai.getTaskByKey(runTaskID)
			if err != nil || current == nil {
				return
			}
			current.State = AI_TASK_STATE_IN_PROGRESS
			current.Response = nil
			if saveErr := ai.createOrUpdateAiTask(current, runTaskID); saveErr != nil {
				ai.logger.Warn("Failed to reset run task to in-progress after approval", "taskID", runTaskID, "error", saveErr)
			}
			ai.notifyTaskChanged(current)
		}

		ai.logger.Info("Agent run waiting for approval", "taskID", runTaskID, "tool", toolName)
		pollTicker := time.NewTicker(3 * time.Second)
		defer pollTicker.Stop()
		timer := time.NewTimer(approvalTimeout)
		defer timer.Stop()
		for {
			select {
			case res := <-ch:
				// Fast path: signaled directly by ApproveTask/RejectTask on
				// this replica.
				if res.err != nil {
					return nil, res.err
				}
				resetToInProgress()
				return buildApproverCtx(res.approver), nil

			case <-pollTicker.C:
				// Fallback: read Valkey to catch approvals that arrived via
				// any other path (different replica, UI update endpoint, etc.).
				current, err := ai.getTaskByKey(runTaskID)
				if err != nil || current == nil {
					return nil, fmt.Errorf("run task no longer exists")
				}
				if current.State == AI_TASK_STATE_PROPOSED {
					continue
				}
				if current.State == AI_TASK_STATE_EXECUTED || current.State == AI_TASK_STATE_SOLVED {
					var approver structs.User
					if current.Approval != nil {
						approver = current.Approval.User
					}
					resetToInProgress()
					return buildApproverCtx(approver), nil
				}
				// Any other non-proposed state is treated as a rejection.
				reason := string(current.State)
				if current.Approval != nil && current.Approval.Reason != "" {
					reason = current.Approval.Reason
				}
				return nil, fmt.Errorf("rejected: %s", reason)

			case <-timer.C:
				return nil, fmt.Errorf("approval timeout: no decision within %s", approvalTimeout)
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	return agent, toolCtx, nil
}

// pruneOlderAllClearReports deletes every all-clear report (completed
// whole-scope run without findings) of the agent except keepKey, so exactly
// one — the newest — stays visible. Tasks with findings are never touched.
func (ai *aiManager) pruneOlderAllClearReports(agentName string, keepKey string) {
	if agentName == "" {
		return
	}
	keys, err := ai.valkeyClient.Keys(fmt.Sprintf("%s:Agent:*:%s-run-*", DB_AI_BUCKET_TASKS, agentName))
	if err != nil {
		ai.logger.Warn("Failed to list agent tasks for all-clear pruning", "agent", agentName, "error", err)
		return
	}
	for _, key := range keys {
		if key == keepKey {
			continue
		}
		task, err := ai.getTaskByKey(key)
		if err != nil || task == nil {
			continue
		}
		// Only completed runs WITHOUT findings are all-clear reports; the
		// key pattern also matches finding tasks (-f2…), which carry a
		// response and are skipped here.
		if task.State != AI_TASK_STATE_COMPLETED || task.Response != nil {
			continue
		}
		if delErr := ai.valkeyClient.DeleteSingle(key); delErr != nil {
			ai.logger.Error("Error deleting superseded all-clear report", "taskID", task.ID, "error", delErr)
			continue
		}
		ai.sendAiDeleteEvent(key)
		ai.logger.Info("Pruned superseded all-clear report", "agent", agentName, "taskID", task.ID)
	}
}

// triggerChangeAgents enqueues a whole-scope run for every enabled agent whose
// change trigger matches this object change (kind + change type + scope) and
// whose cooldown has elapsed. The cooldown coalesces a burst of changes into a
// single run: the first change fires, the rest are absorbed until the interval
// passes. Runs are also gated by hasOpenAgentRun (one open run per agent).
func (ai *aiManager) triggerChangeAgents(obj *unstructured.Unstructured, changeType string) {
	for _, agent := range ai.getEnabledAgents() {
		oc := agent.Spec.Triggers.OnChange
		if oc == nil {
			continue
		}
		if !changeTypeSelected(oc.On, changeType) || !kindSelected(oc.Kinds, obj.GetKind()) {
			continue
		}
		namespaces := ai.resolveAgentScope(&agent)
		if !namespaceSelected(namespaces, obj.GetNamespace()) {
			continue
		}
		if !ai.changeCooldownElapsed(agent.Name, oc) {
			continue
		}
		agentCopy := agent
		if _, err := ai.createAgentRunTask(&agentCopy, AI_TASK_TRIGGER_EVENT, nil); err != nil {
			// An already-open run or empty scope is expected/benign here.
			ai.logger.Info("Change trigger did not enqueue a run", "agent", agent.Name, "reason", err.Error())
			continue
		}
		ai.logger.Info("Change trigger enqueued a run", "agent", agent.Name, "kind", obj.GetKind(), "namespace", obj.GetNamespace(), "changeType", changeType)
	}
}

// changeCooldownElapsed reports whether enough time passed since the agent's
// last run for a change trigger to fire again.
func (ai *aiManager) changeCooldownElapsed(agentName string, oc *v1alpha1.AgentChangeTrigger) bool {
	ai.cronStateLock.Lock()
	last, seen := ai.lastAgentRun[agentName]
	ai.cronStateLock.Unlock()
	if !seen {
		return true
	}
	return time.Since(last) >= changeCooldown(oc)
}

// changeTypeSelected reports whether changeType is selected; an empty list
// means all change types.
func changeTypeSelected(on []string, changeType string) bool {
	if len(on) == 0 {
		return true
	}
	return slices.Contains(on, changeType)
}

// kindSelected reports whether kind is selected; an empty list means all kinds.
func kindSelected(kinds []string, kind string) bool {
	if len(kinds) == 0 {
		return true
	}
	return slices.Contains(kinds, kind)
}

// namespaceSelected reports whether the object's namespace is within the
// resolved agent scope. "*" in the scope matches any namespace.
func namespaceSelected(scope []string, namespace string) bool {
	for _, ns := range scope {
		if ns == "*" || ns == namespace {
			return true
		}
	}
	return false
}
