package ai

import (
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
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

// ValidateAgentSpec checks an agent spec for the invariants the pipeline
// relies on: a non-empty scope (an agent without scope restrictions must not
// exist — empty allow-maps would disable namespace checks entirely), a
// parseable cron expression and event filters with a kind.
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
	for i, filter := range spec.Triggers.Events {
		if filter.Kind == "" {
			return fmt.Errorf("event filter %d is missing a kind", i)
		}
		if len(filter.Contains) == 0 {
			return fmt.Errorf("event filter %d (%s) needs at least one contains condition", i, filter.Name)
		}
	}
	return nil
}

// agentFilterToAiFilter converts a CRD event filter into the internal AiFilter
// shape so the existing matching and debounce machinery keeps working on it.
func agentFilterToAiFilter(agent *v1alpha1.Agent, filter v1alpha1.AgentEventFilter) AiFilter {
	converted := AiFilter{
		Id:       filter.Id,
		Name:     filter.Name,
		Kind:     filter.Kind,
		Contains: filter.Contains,
		Excludes: filter.Excludes,
		Prompt:   filter.Prompt,
		IsActive: true,
	}
	if converted.Id == "" {
		converted.Id = agent.Name + "/" + filter.Kind
	}
	if converted.Name == "" {
		converted.Name = converted.Id
	}
	if filter.For != nil {
		duration := filter.For.Duration
		converted.For = &duration
	}
	return converted
}

// getEnabledAgents returns all enabled agents from the operator namespace.
func (ai *aiManager) getEnabledAgents() []v1alpha1.Agent {
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
func buildAgentRunPrompt(agent *v1alpha1.Agent, namespaces []string) string {
	var sb strings.Builder
	sb.WriteString("You are running a scheduled analysis of the Kubernetes namespaces in your scope: ")
	sb.WriteString(strings.Join(namespaces, ", "))
	sb.WriteString(".\n\nInspect the workloads in these namespaces with your read-only tools and report every distinct issue you find as its own finding — there is no upper limit. Submit findings incrementally via " + submitAnalysisToolName + " as soon as you have confirmed them, then keep investigating. Be efficient — you have a limited tool-call and token budget: list resources cluster-wide (omit the namespace parameter) instead of namespace by namespace, inspect suspicious candidates with get detail=summary, and fetch the full manifest only when you need it to build an UpdateResource proposal.")
	if agent.Spec.Instruction != "" {
		sb.WriteString("\n\nYour instruction:\n")
		sb.WriteString(agent.Spec.Instruction)
	}
	sb.WriteString("\n\nOnly report findings you can back with a concrete, safe, directly applicable remediation: a proposed operation plus the complete target resource YAML, based on the live manifest you retrieved. Advice-only findings without an applicable change are discarded — do not report them. If nothing needs fixing, submit an empty findings list; that is a perfectly good result.")
	return sb.String()
}

// agentRunKeyPrefix returns the Valkey key prefix for whole-scope runs of an
// agent. The second segment is a scope namespace so workspace-filtered task
// queries (pattern ai_tasks:*:<ns>:*) include these runs.
func agentRunKeyPrefix(namespace, agentName string) string {
	return fmt.Sprintf("%s:Agent:%s:%s-run-", DB_AI_BUCKET_TASKS, namespace, agentName)
}

// hasOpenAgentRun reports whether the agent already has a pending or
// in-progress whole-scope run, bounding cron/manual fan-out to one open run.
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
		if task.State == AI_TASK_STATE_PENDING || task.State == AI_TASK_STATE_IN_PROGRESS {
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
	}
	if err := ai.createOrUpdateAiTask(task, key); err != nil {
		return nil, fmt.Errorf("failed to create agent run task: %w", err)
	}
	ai.logger.Info("Agent run task created", "agent", agent.Name, "trigger", trigger, "taskID", task.ID)
	return task, nil
}

// TriggerAgent creates a manual whole-scope run for the agent, requested by a
// user from the UI.
func (ai *aiManager) TriggerAgent(agentName string, user structs.User) (*AiTask, error) {
	agent, err := ai.getAgent(agentName)
	if err != nil {
		return nil, err
	}
	if !agent.Spec.Enabled {
		return nil, fmt.Errorf("agent %q is disabled", agentName)
	}
	if !agent.Spec.Triggers.Manual {
		return nil, fmt.Errorf("agent %q does not allow manual triggering", agentName)
	}
	return ai.createAgentRunTask(agent, AI_TASK_TRIGGER_MANUAL, &user)
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
		inScope := false
		for _, ns := range namespaces {
			if ns == taskNamespace {
				inScope = true
				break
			}
		}
		if !inScope {
			return nil, nil, fmt.Errorf("resource namespace %q is no longer in the scope of agent %q", taskNamespace, agent.Name)
		}
	}
	return agent, newToolContextFromAgent(agent, namespaces), nil
}

// matchingAgentTask returns the task to create for a watched object, or nil if
// no enabled agent's event filter matches. The first matching agent wins —
// tasks are deduplicated per resource, matching the previous filter behavior.
func (ai *aiManager) matchingAgentTask(obj *unstructured.Unstructured, resource utils.ResourceDescriptor) *AiTask {
	for _, agent := range ai.getEnabledAgents() {
		if len(agent.Spec.Triggers.Events) == 0 {
			continue
		}
		namespaces := ai.resolveAgentScope(&agent)
		if len(namespaces) == 0 {
			continue
		}
		inScope := false
		for _, ns := range namespaces {
			if ns == obj.GetNamespace() {
				inScope = true
				break
			}
		}
		if !inScope {
			continue
		}

		for _, eventFilter := range agent.Spec.Triggers.Events {
			if obj.GetKind() != eventFilter.Kind {
				continue
			}
			filter := agentFilterToAiFilter(&agent, eventFilter)
			matches, err := filterMatchesForObject(filter, obj)
			if err != nil {
				ai.logger.Error("Error checking agent event filter match", "agent", agent.Name, "filter", filter.Name, "objectKind", obj.GetKind(), "objectName", obj.GetName(), "objectNamespace", obj.GetNamespace(), "error", err)
				continue
			}
			if !matches {
				continue
			}

			timestamp := time.Now().Unix()
			return &AiTask{
				Prompt:    filter.Prompt,
				State:     AI_TASK_STATE_PENDING,
				CreatedAt: timestamp,
				UpdatedAt: timestamp,
				ReferencingResource: utils.WorkloadSingleRequest{
					ResourceDescriptor: resource,
					Namespace:          obj.GetNamespace(),
					ResourceName:       obj.GetName(),
				},
				TriggeredBy: filter,
				AgentRef:    agent.Name,
				Trigger:     AI_TASK_TRIGGER_EVENT,
			}
		}
	}
	return nil
}
