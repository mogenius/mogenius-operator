package ai

import (
	"fmt"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"strings"
	"time"

	"sigs.k8s.io/yaml"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ApproveTask executes a proposed task's operation with the approving user's
// permissions. The whole flow is deterministic — no LLM is involved; the
// stored TargetResourceYaml/TargetResource of the proposal is applied through
// the same tool handlers (and thus the same RBAC checks and audit trail) that
// the chat path uses.
func (ai *aiManager) ApproveTask(taskID string, user structs.User, workspace string) (*AiTask, error) {
	task, err := ai.getTaskByKey(taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf("no ai task with the specified id has been found: %s", taskID)
	}
	if task.State != AI_TASK_STATE_PROPOSED {
		return nil, fmt.Errorf("task %s is in state %q; only proposed tasks can be approved", taskID, task.State)
	}
	if user.Email == "" {
		return nil, fmt.Errorf("approving a task requires an attributable user")
	}

	toolCtx, err := ai.buildApproverToolContext(task, user, workspace)
	if err != nil {
		return nil, err
	}

	if err := ai.checkProposalFreshness(task); err != nil {
		// Keep the task approvable: the proposal itself is intact, only the
		// cluster moved on. The error is surfaced on the task and returned.
		task.Error = err.Error()
		if updateErr := ai.createOrUpdateAiTask(task, taskID); updateErr != nil {
			ai.logger.Error("Error persisting staleness info on AI task", "taskID", taskID, "error", updateErr)
		}
		ai.notifyTaskChanged(task)
		return task, err
	}

	task.State = AI_TASK_STATE_EXECUTING
	task.Error = ""
	task.Approval = &ApprovalRecord{User: user, At: time.Now()}
	if err := ai.createOrUpdateAiTask(task, taskID); err != nil {
		return nil, fmt.Errorf("failed to set task to executing: %w", err)
	}
	ai.notifyTaskChanged(task)

	result, execErr := ai.executeProposal(task, toolCtx)
	task.ExecutionResult = result
	if execErr != nil {
		task.State = AI_TASK_STATE_EXECUTION_FAILED
		task.Error = execErr.Error()
		ai.logger.Error("AI task execution failed", "taskID", taskID, "approvedBy", user.Email, "error", execErr)
	} else {
		task.State = AI_TASK_STATE_EXECUTED
		ai.logger.Info("AI task executed", "taskID", taskID, "approvedBy", user.Email, "result", result)
	}

	if err := ai.createOrUpdateAiTask(task, taskID); err != nil {
		ai.logger.Error("Error persisting AI task after execution", "taskID", taskID, "error", err)
	}
	ai.notifyTaskChanged(task)
	return task, execErr
}

// RejectTask marks a proposed task as rejected by the given user.
func (ai *aiManager) RejectTask(taskID string, user structs.User, reason string) (*AiTask, error) {
	task, err := ai.getTaskByKey(taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf("no ai task with the specified id has been found: %s", taskID)
	}
	if task.State != AI_TASK_STATE_PROPOSED {
		return nil, fmt.Errorf("task %s is in state %q; only proposed tasks can be rejected", taskID, task.State)
	}
	if user.Email == "" {
		return nil, fmt.Errorf("rejecting a task requires an attributable user")
	}

	task.State = AI_TASK_STATE_REJECTED
	task.Approval = &ApprovalRecord{User: user, At: time.Now(), Rejected: true, Reason: reason}
	if err := ai.createOrUpdateAiTask(task, taskID); err != nil {
		return nil, fmt.Errorf("failed to persist rejection: %w", err)
	}
	ai.notifyTaskChanged(task)
	return task, nil
}

// buildApproverToolContext derives the executing ToolContext from the
// approving user's workspace grant. An empty workspace mirrors the chat
// path's platform-admin context (unrestricted); with a workspace the user
// needs an editor/admin grant covering the target namespace.
func (ai *aiManager) buildApproverToolContext(task *AiTask, user structs.User, workspace string) (*ToolContext, error) {
	var toolCtx *ToolContext
	if workspace == "" {
		toolCtx = newToolContextFromUserGrant(&user, "", true, nil, nil)
	} else {
		workspaceSpec, grantSpec := ai.ResolveWorkspaceContext(user.Email, workspace)
		if workspaceSpec == nil && grantSpec == nil {
			return nil, fmt.Errorf("no grant found for user %q in workspace %q", user.Email, workspace)
		}
		toolCtx = newToolContextFromUserGrant(&user, workspace, false, workspaceSpec, grantSpec)
	}

	toolCtx.AuditSource = "ai-agent-approval"

	if !toolCtx.IsEditor() && !toolCtx.IsAdmin() {
		return nil, fmt.Errorf("user %q has no editor or admin role for workspace %q", user.Email, workspace)
	}

	// Every namespace touched by the proposal must be permitted — for a bulk
	// delete that is each target's namespace, not just the primary's.
	namespaces := map[string]bool{task.Response.Analysis.TargetResource.Namespace: true}
	if task.Response.Analysis.ProposedOperation == ProposedOperationDelete {
		for _, t := range ai.deleteTargets(task.Response.Analysis) {
			namespaces[t.Namespace] = true
		}
	}
	for ns := range namespaces {
		if ns != "" && !toolCtx.IsNamespaceAllowed(ns) {
			return nil, fmt.Errorf("user %q is not allowed to modify namespace %q", user.Email, ns)
		}
	}
	return toolCtx, nil
}

// checkProposalFreshness rejects execution when the target resource changed
// since the proposal was created (or, for creates, already exists).
func (ai *aiManager) checkProposalFreshness(task *AiTask) error {
	if task.Response == nil {
		return fmt.Errorf("task has no analysis response")
	}
	analysis := task.Response.Analysis
	target := analysis.TargetResource

	switch analysis.ProposedOperation {
	case ProposedOperationUpdate:
		current, err := store.GetResource(ai.valkeyClient, target.ApiVersion, target.Kind, target.Namespace, target.ResourceName, ai.logger)
		if err != nil || current == nil {
			return fmt.Errorf("proposal stale: target resource %s/%s in namespace %q no longer exists", target.Kind, target.ResourceName, target.Namespace)
		}
		if task.BaseResourceVersion != "" && current.GetResourceVersion() != task.BaseResourceVersion {
			return fmt.Errorf("proposal stale: resource changed since the proposal was created (resourceVersion %s → %s)", task.BaseResourceVersion, current.GetResourceVersion())
		}
	case ProposedOperationDelete:
		// A delete is idempotent: individually vanished targets are fine and
		// handled per-target at execution. Only block when there is nothing
		// left to delete at all.
		for _, t := range ai.deleteTargets(analysis) {
			if current, err := store.GetResource(ai.valkeyClient, t.ApiVersion, t.Kind, t.Namespace, t.ResourceName, ai.logger); err == nil && current != nil {
				return nil
			}
		}
		return fmt.Errorf("proposal stale: none of the target resources exist anymore")
	case ProposedOperationCreate:
		parsed, err := parseTargetYaml(analysis.TargetResourceYaml)
		if err != nil {
			return err
		}
		current, _ := store.GetResource(ai.valkeyClient, target.ApiVersion, parsed.GetKind(), parsed.GetNamespace(), parsed.GetName(), ai.logger)
		if current != nil {
			return fmt.Errorf("proposal stale: resource %s/%s in namespace %q already exists", parsed.GetKind(), parsed.GetName(), parsed.GetNamespace())
		}
	default:
		return fmt.Errorf("task has no executable proposed operation (got %q)", analysis.ProposedOperation)
	}
	return nil
}

// executeProposal applies the proposed operation through the existing AI tool
// handlers so RBAC checks and resource-indexed audit entries fire on the same
// code path as chat-driven mutations. Tool handlers return error STRINGS by
// convention ("Error: ..."), which is translated back into an error here.
func (ai *aiManager) executeProposal(task *AiTask, toolCtx *ToolContext) (string, error) {
	analysis := task.Response.Analysis
	target := analysis.TargetResource

	if target.ApiVersion == "" || target.Plural == "" {
		return "", fmt.Errorf("proposal is missing the target resource descriptor (apiVersion=%q, plural=%q); cannot execute safely", target.ApiVersion, target.Plural)
	}

	var toolName string
	var args map[string]any

	switch analysis.ProposedOperation {
	case ProposedOperationUpdate, ProposedOperationCreate:
		parsed, err := parseTargetYaml(analysis.TargetResourceYaml)
		if err != nil {
			return "", err
		}
		if target.ResourceName != "" && parsed.GetName() != target.ResourceName {
			return "", fmt.Errorf("proposal inconsistent: YAML resource name %q does not match target %q", parsed.GetName(), target.ResourceName)
		}
		if target.Namespace != "" && parsed.GetNamespace() != target.Namespace {
			return "", fmt.Errorf("proposal inconsistent: YAML namespace %q does not match target %q", parsed.GetNamespace(), target.Namespace)
		}
		toolName = "update_kubernetes_resource"
		if analysis.ProposedOperation == ProposedOperationCreate {
			toolName = "create_kubernetes_resource"
		}
		args = map[string]any{
			"yamlData":   analysis.TargetResourceYaml,
			"apiVersion": target.ApiVersion,
			"plural":     target.Plural,
			"namespaced": target.Namespaced,
		}
	case ProposedOperationDelete:
		return ai.executeBulkDelete(analysis, toolCtx)
	default:
		return "", fmt.Errorf("task has no executable proposed operation (got %q)", analysis.ProposedOperation)
	}

	tool, ok := toolDefinitions[toolName]
	if !ok {
		return "", fmt.Errorf("tool %q is not registered", toolName)
	}

	result := tool(args, toolCtx, ai.valkeyClient, ai.logger)
	if strings.HasPrefix(result, "Error") {
		return result, fmt.Errorf("%s", result)
	}
	return result, nil
}

// deleteTargets returns the full list of resources a DeleteResource proposal
// removes: the primary target plus any additional bulk targets. Missing
// apiVersion/plural/namespaced on an additional target default to the
// primary's, since bulk deletions are almost always the same kind.
func (ai *aiManager) deleteTargets(analysis Analysis) []utils.WorkloadSingleRequest {
	primary := analysis.TargetResource
	targets := make([]utils.WorkloadSingleRequest, 0, 1+len(analysis.AdditionalTargets))
	seen := map[string]bool{}
	add := func(t utils.WorkloadSingleRequest) {
		if t.ApiVersion == "" {
			t.ApiVersion = primary.ApiVersion
		}
		if t.Plural == "" {
			t.Plural = primary.Plural
		}
		if t.Kind == "" {
			t.Kind = primary.Kind
		}
		if !t.Namespaced {
			t.Namespaced = primary.Namespaced
		}
		if t.ResourceName == "" {
			return
		}
		key := aiResourceKey(t.ApiVersion, t.Kind, t.Namespace, t.ResourceName)
		if seen[key] {
			return
		}
		seen[key] = true
		targets = append(targets, t)
	}
	add(primary)
	for _, t := range analysis.AdditionalTargets {
		add(t)
	}
	return targets
}

// executeBulkDelete deletes every target of a (possibly bulk) DeleteResource
// proposal through the delete tool, so each deletion runs the same RBAC and
// audit path. Individual failures are collected and reported without aborting
// the rest, so one blocked resource does not strand the others.
func (ai *aiManager) executeBulkDelete(analysis Analysis, toolCtx *ToolContext) (string, error) {
	tool, ok := toolDefinitions["delete_kubernetes_resource"]
	if !ok {
		return "", fmt.Errorf("tool %q is not registered", "delete_kubernetes_resource")
	}
	targets := ai.deleteTargets(analysis)
	if len(targets) == 0 {
		return "", fmt.Errorf("delete proposal has no target resources")
	}

	var lines []string
	deleted := 0
	for _, t := range targets {
		if t.ApiVersion == "" || t.Plural == "" {
			lines = append(lines, fmt.Sprintf("✗ %s/%s: missing apiVersion/plural", t.Kind, t.ResourceName))
			continue
		}
		result := tool(map[string]any{
			"apiVersion": t.ApiVersion,
			"plural":     t.Plural,
			"namespace":  t.Namespace,
			"name":       t.ResourceName,
		}, toolCtx, ai.valkeyClient, ai.logger)
		if strings.HasPrefix(result, "Error") {
			lines = append(lines, fmt.Sprintf("✗ %s/%s: %s", t.Kind, t.ResourceName, result))
		} else {
			deleted++
			lines = append(lines, fmt.Sprintf("✓ deleted %s/%s", t.Kind, t.ResourceName))
		}
	}

	failures := len(targets) - deleted
	summary := fmt.Sprintf("Deleted %d of %d resources", deleted, len(targets))
	if len(targets) == 1 {
		// Single delete: keep the tool's own message as the result.
		if failures > 0 {
			return lines[0], fmt.Errorf("%s", lines[0])
		}
		return lines[0], nil
	}
	detail := summary + "\n" + strings.Join(lines, "\n")
	if failures > 0 {
		return detail, fmt.Errorf("%d of %d deletions failed", failures, len(targets))
	}
	return detail, nil
}

func parseTargetYaml(yamlData string) (*unstructured.Unstructured, error) {
	if yamlData == "" {
		return nil, fmt.Errorf("proposal has no target resource YAML")
	}
	var obj *unstructured.Unstructured
	if err := yaml.Unmarshal([]byte(yamlData), &obj); err != nil {
		return nil, fmt.Errorf("failed to parse proposal target YAML: %w", err)
	}
	if obj == nil || obj.GetName() == "" {
		return nil, fmt.Errorf("proposal target YAML has no metadata.name")
	}
	return obj, nil
}

// notifyTaskChanged pushes the task's new state to the UI and refreshes the
// cached status counters.
func (ai *aiManager) notifyTaskChanged(task *AiTask) {
	ai.resetCache()
	ai.sendAiEvent(&AiTaskLatest{
		Task:   task,
		Status: ai.GetStatus(nil),
	})
}
