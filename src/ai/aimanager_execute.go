package ai

import (
	"fmt"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
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

	targetNamespace := task.Response.Analysis.TargetResource.Namespace
	if targetNamespace != "" && !toolCtx.IsNamespaceAllowed(targetNamespace) {
		return nil, fmt.Errorf("user %q is not allowed to modify namespace %q", user.Email, targetNamespace)
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
	case ProposedOperationUpdate, ProposedOperationDelete:
		current, err := store.GetResource(ai.valkeyClient, target.ApiVersion, target.Kind, target.Namespace, target.ResourceName, ai.logger)
		if err != nil || current == nil {
			return fmt.Errorf("proposal stale: target resource %s/%s in namespace %q no longer exists", target.Kind, target.ResourceName, target.Namespace)
		}
		if task.BaseResourceVersion != "" && current.GetResourceVersion() != task.BaseResourceVersion {
			return fmt.Errorf("proposal stale: resource changed since the proposal was created (resourceVersion %s → %s)", task.BaseResourceVersion, current.GetResourceVersion())
		}
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
		toolName = "delete_kubernetes_resource"
		args = map[string]any{
			"apiVersion": target.ApiVersion,
			"plural":     target.Plural,
			"namespace":  target.Namespace,
			"name":       target.ResourceName,
		}
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
