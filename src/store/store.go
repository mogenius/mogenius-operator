package store

import (
	"errors"
	"fmt"
	"log/slog"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/logging"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"sigs.k8s.io/yaml"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "k8s.io/api/apps/v1"
	v1batch "k8s.io/api/batch/v1"
	coreV1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

const (
	VALKEY_RESOURCE_PREFIX = "resources"
)

var AuditLogLimit = int64(100) // Default limit for audit log entries IMPORTANT: this is set per resource not globally

var ErrNotFound = errors.New("not found")

var valkeyClient valkeyclient.ValkeyClient

func Setup(
	logManagerModule logging.SlogManager,
	valkey valkeyclient.ValkeyClient,
	auditLogLimitStr string,
) error {
	valkeyClient = valkey
	auditLogLimit, _ := strconv.ParseInt(auditLogLimitStr, 10, 64)
	if auditLogLimit > 0 {
		AuditLogLimit = auditLogLimit
	}

	return nil
}

func SearchResourceByKeyParts(valkeyClient valkeyclient.ValkeyClient, parts ...string) ([]unstructured.Unstructured, error) {
	key := CreateResourceKey(parts...)

	items, err := valkeyclient.GetObjectsByPrefix[unstructured.Unstructured](valkeyClient, valkeyclient.ORDER_NONE, key)

	if len(items) == 0 {
		return nil, ErrNotFound

	}

	return items, err
}

func SearchByNamespaceAndName(valkeyClient valkeyclient.ValkeyClient, namespace string, name string) ([]unstructured.Unstructured, error) {
	pattern := CreateKeyPattern(nil, nil, &namespace, &name)

	items, err := valkeyclient.GetObjectsByPattern[unstructured.Unstructured](valkeyClient, pattern, []string{})

	return items, err
}

func SearchByGroupKindNameNamespace(valkeyClient valkeyclient.ValkeyClient, apiVersion string, kind string, name string, namespace *string) ([]unstructured.Unstructured, error) {
	pattern := CreateKeyPattern(&apiVersion, &kind, namespace, &name)

	items, err := valkeyclient.GetObjectsByPattern[unstructured.Unstructured](valkeyClient, pattern, []string{})

	return items, err
}

func SearchResourceByNamespace(valkeyClient valkeyclient.ValkeyClient, namespace string, whitelist []*utils.ResourceDescriptor) ([]unstructured.Unstructured, error) {
	pattern := CreateKeyPattern(nil, nil, &namespace, nil)

	var searchKeys []string
	if len(whitelist) > 0 {
		for _, item := range whitelist {
			searchKey := CreateResourceKey(item.ApiVersion, item.Kind, namespace)
			searchKeys = append(searchKeys, searchKey)
		}
	}

	items, err := valkeyclient.GetObjectsByPattern[unstructured.Unstructured](valkeyClient, pattern, searchKeys)

	return items, err
}

func DropAllResourcesFromValkey(valkeyClient valkeyclient.ValkeyClient, logger *slog.Logger) error {
	keys, err := valkeyClient.Keys(VALKEY_RESOURCE_PREFIX + ":*")
	if err != nil {
		return fmt.Errorf("failed to get keys: %v", err)
	}
	err = valkeyClient.DeleteMultiple(keys...)
	if err != nil {
		logger.Error("failed to DropAllResourcesFromValkey", "error", err)
	}
	return err
}

func DropKey(valkeyClient valkeyclient.ValkeyClient, logger *slog.Logger, key string) error {
	err := valkeyClient.DeleteMultiple(key)
	if err != nil {
		logger.Error("failed to DropKey", "error", err)
	}
	return err
}

func CreateResourceKey(parts ...string) string {
	parts = append([]string{VALKEY_RESOURCE_PREFIX}, parts...)
	return strings.Join(parts, ":")
}

func CreateKeyPattern(apiVersion, kind, namespace, name *string) string {
	parts := make([]string, 5)

	parts[0] = VALKEY_RESOURCE_PREFIX

	if apiVersion != nil && *apiVersion != "" {
		parts[1] = *apiVersion
	} else {
		parts[1] = "*"
	}

	if kind != nil && *kind != "" {
		parts[2] = *kind
	} else {
		parts[2] = "*"
	}

	if namespace != nil && *namespace != "" {
		parts[3] = *namespace
	} else {
		parts[3] = "*"
	}

	if name != nil && *name != "" {
		parts[4] = *name
	} else {
		parts[4] = "*"
	}

	pattern := strings.Join(parts, ":")
	return pattern
}

func GetResource(valkeyClient valkeyclient.ValkeyClient, apiVersion string, kind string, namespace string, name string, logger *slog.Logger) (*unstructured.Unstructured, error) {
	return valkeyclient.GetObjectForKey[unstructured.Unstructured](valkeyClient, VALKEY_RESOURCE_PREFIX, apiVersion, kind, namespace, name)
}

func GetResourceByKindAndNamespace(valkeyClient valkeyclient.ValkeyClient, apiVersion string, kind string, namespace string, logger *slog.Logger) []unstructured.Unstructured {
	var results []unstructured.Unstructured

	pattern := CreateKeyPattern(&apiVersion, &kind, &namespace, nil)
	storeResults, err := valkeyclient.GetObjectsByPrefix[unstructured.Unstructured](valkeyClient, valkeyclient.ORDER_NONE, pattern)
	if err != nil {
		logger.Error("failed to get resources by kind and namespace", "apiVersion", apiVersion, "kind", kind, "namespace", namespace, "error", err)
		return results
	}

	for _, ref := range storeResults {
		if (namespace != "" && ref.GetNamespace() != namespace) || (kind != "" && ref.GetKind() != kind) {
			continue
		}

		results = append(results, ref)
	}
	return results
}

func GetIngressClasses() []networkingv1.IngressClass {
	ingressClasses, err := valkeyclient.GetObjectsByPrefix[networkingv1.IngressClass](valkeyClient, valkeyclient.ORDER_NONE, VALKEY_RESOURCE_PREFIX, utils.IngressClassResource.ApiVersion, utils.IngressClassResource.Kind, "*")
	if err != nil {
		return ingressClasses
	}
	return ingressClasses
}

func GetPod(namespace string, name string) *coreV1.Pod {
	pod, err := valkeyclient.GetObjectForKey[coreV1.Pod](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.PodResource.ApiVersion, utils.PodResource.Kind, namespace, name)
	if err != nil || pod == nil {
		return nil
	}
	return pod
}

func GetPods(namespace string) []coreV1.Pod {
	pods, err := valkeyclient.GetObjectsByPrefix[coreV1.Pod](valkeyClient, valkeyclient.ORDER_ASC, VALKEY_RESOURCE_PREFIX, utils.PodResource.ApiVersion, utils.PodResource.Kind, namespace)
	if err != nil || pods == nil {
		return nil
	}
	return pods
}

func GetReplicaset(namespace string, name string) *v1.ReplicaSet {
	replicaSet, err := valkeyclient.GetObjectForKey[v1.ReplicaSet](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.ReplicaSetResource.ApiVersion, utils.ReplicaSetResource.Kind, namespace, name)
	if err != nil || replicaSet == nil {
		return nil
	}
	return replicaSet
}

func GetDeployment(namespace string, name string) *v1.Deployment {
	deployment, err := valkeyclient.GetObjectForKey[v1.Deployment](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.DeploymentResource.ApiVersion, utils.DeploymentResource.Kind, namespace, name)
	if err != nil || deployment == nil {
		return nil
	}
	return deployment
}

func GetDeployments(namespace string, name string) []v1.Deployment {
	deployments, err := valkeyclient.GetObjectsByPrefix[v1.Deployment](valkeyClient, valkeyclient.ORDER_ASC, VALKEY_RESOURCE_PREFIX, utils.DeploymentResource.ApiVersion, utils.DeploymentResource.Kind, namespace, name)
	if err != nil || deployments == nil {
		return nil
	}
	return deployments
}

func GetSecret(namespace string, name string) *coreV1.Secret {
	secret, err := valkeyclient.GetObjectForKey[coreV1.Secret](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.SecretResource.ApiVersion, utils.SecretResource.Kind, namespace, name)
	if err != nil || secret == nil {
		return nil
	}

	return secret
}

func GetSecrets(namespace string, name string) []coreV1.Secret {
	secrets, err := valkeyclient.GetObjectsByPrefix[coreV1.Secret](valkeyClient, valkeyclient.ORDER_ASC, VALKEY_RESOURCE_PREFIX, utils.SecretResource.ApiVersion, utils.SecretResource.Kind, namespace, name)
	if err != nil || secrets == nil {
		return nil
	}
	return secrets
}

func GetService(namespace string, name string) *coreV1.Service {
	service, err := valkeyclient.GetObjectForKey[coreV1.Service](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.ServiceResource.ApiVersion, utils.ServiceResource.Kind, namespace, name)
	if err != nil || service == nil {
		return nil
	}

	return service
}

func GetServices(namespace string, name string) []coreV1.Service {
	services, err := valkeyclient.GetObjectsByPrefix[coreV1.Service](valkeyClient, valkeyclient.ORDER_ASC, VALKEY_RESOURCE_PREFIX, utils.ServiceResource.ApiVersion, utils.ServiceResource.Kind, namespace, name)
	if err != nil || services == nil {
		return nil
	}
	return services
}

func GetStatefulSet(namespace string, name string) *v1.StatefulSet {
	statefulSet, err := valkeyclient.GetObjectForKey[v1.StatefulSet](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.StatefulSetResource.ApiVersion, utils.StatefulSetResource.Kind, namespace, name)
	if err != nil || statefulSet == nil {
		return nil
	}
	return statefulSet
}

func GetDaemonSet(namespace string, name string) *v1.DaemonSet {
	daemonSet, err := valkeyclient.GetObjectForKey[v1.DaemonSet](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.DaemonSetResource.ApiVersion, utils.DaemonSetResource.Kind, namespace, name)
	if err != nil || daemonSet == nil {
		return nil
	}
	return daemonSet
}

func GetJob(namespace string, name string) *v1batch.Job {
	job, err := valkeyclient.GetObjectForKey[v1batch.Job](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.JobResource.ApiVersion, utils.JobResource.Kind, namespace, name)
	if err != nil || job == nil {
		return nil
	}
	return job
}

func GetConfigMap(namespace string, name string) *coreV1.ConfigMap {
	configMap, err := valkeyclient.GetObjectForKey[coreV1.ConfigMap](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.ConfigMapResource.ApiVersion, utils.ConfigMapResource.Kind, namespace, name)
	if err != nil || configMap == nil {
		return nil
	}

	return configMap
}

func GetCronJob(namespace string, name string) *v1batch.CronJob {
	cronJob, err := valkeyclient.GetObjectForKey[v1batch.CronJob](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.CronJobResource.ApiVersion, utils.CronJobResource.Kind, namespace, name)
	if err != nil || cronJob == nil {
		return nil
	}
	return cronJob
}

func GetNode(name string) *coreV1.Node {
	node, err := valkeyclient.GetObjectForKey[coreV1.Node](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.NodeResource.ApiVersion, utils.NodeResource.Kind, "", name)
	if err != nil || node == nil {
		return nil
	}
	return node
}

func GetNodes() []coreV1.Node {
	nodes, err := valkeyclient.GetObjectsByPrefix[coreV1.Node](valkeyClient, valkeyclient.ORDER_ASC, VALKEY_RESOURCE_PREFIX, utils.NodeResource.ApiVersion, utils.NodeResource.Kind, "", "*")
	if err != nil {
		return nil
	}

	return nodes
}

func GetAllWorkspaces(namespace string) ([]v1alpha1.Workspace, error) {
	pattern := CreateKeyPattern(&utils.WorkspaceResource.ApiVersion, &utils.WorkspaceResource.Kind, &namespace, nil)
	workspaces, err := valkeyclient.GetObjectsByPrefix[v1alpha1.Workspace](valkeyClient, valkeyclient.ORDER_ASC, pattern)
	if err != nil || workspaces == nil {
		return nil, err
	}
	return workspaces, nil
}

func GetWorkspace(namespace string, name string) (*v1alpha1.Workspace, error) {
	workspace, err := valkeyclient.GetObjectForKey[v1alpha1.Workspace](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.WorkspaceResource.ApiVersion, utils.WorkspaceResource.Kind, namespace, name)
	if err != nil || workspace == nil {
		return nil, err
	}
	return workspace, nil
}

// Audit Log
type AuditLogEntry struct {
	Pattern   string       `json:"pattern" validate:"required"`
	Payload   any          `json:"payload,omitempty"`
	Diff      string       `json:"diff,omitempty"`
	Result    any          `json:"result,omitempty"`
	Error     string       `json:"error,omitempty"`
	CreatedAt time.Time    `json:"createdAt"`
	User      structs.User `json:"user,omitempty"`
	Workspace string       `json:"workspace,omitempty"`
}

type AuditLogResponse struct {
	Data       []AuditLogEntry `json:"data"`
	TotalCount int             `json:"totalCount"`
}

func AddToAuditLog[T any](datagram structs.Datagram, logger *slog.Logger, result T, err error, oldObj *unstructured.Unstructured, updatedObj *unstructured.Unstructured) (T, error) {
	resourceNamespace := ""
	resourceName := ""

	auditLogEntry := auditLogFromDatagram(datagram, result, err)
	if oldObj != nil || updatedObj != nil {
		patch, diffErr := Diff(oldObj, updatedObj)
		if diffErr != nil {
			logger.Error("failed to create kubectl style diff", "error", diffErr)
			return result, err
		}
		auditLogEntry.Diff = patch
	}
	if oldObj != nil {
		resourceNamespace = oldObj.GetNamespace()
		resourceName = oldObj.GetName()
	} else if updatedObj != nil {
		resourceNamespace = updatedObj.GetNamespace()
		resourceName = updatedObj.GetName()
	} else if payload, ok := datagram.Payload.(map[string]any); ok {
		if payload["namespace"] != nil {
			resourceNamespace = payload["namespace"].(string)
		}
		if payload["name"] != nil {
			resourceName = payload["name"].(string)
		}
		if payload["pod"] != nil {
			resourceName = payload["pod"].(string)
		}
	} else if yamlData, ok := payload["yamlData"].(string); ok {
		var unstruct unstructured.Unstructured
		err := yaml.Unmarshal([]byte(yamlData), &unstruct)
		if err == nil {
			resourceNamespace = unstruct.GetNamespace()
			resourceName = unstruct.GetName()
		} else {
			return result, fmt.Errorf("failed to guess Namespace and ResourceName from datagram payload: %w", err)
		}
	}

	auditLogAddErr := valkeyClient.SetObjectWithAutoincrementLimit(auditLogEntry, AuditLogLimit, "audit-log", resourceNamespace, resourceName)
	if auditLogAddErr != nil {
		logger.Error("failed to add to audit log", "error", auditLogAddErr)
	}
	return result, err
}

func ListAuditLog(limit int, offset int, namespaces []string, clusterWide bool) ([]AuditLogEntry, int, error) {
	if limit <= 0 {
		limit = 100
	}

	entries, size, err := valkeyclient.GetObjectsByPrefixWithSizeAndNs[AuditLogEntry](valkeyClient, limit, offset, namespaces, clusterWide, "audit-log")
	if err != nil {
		return []AuditLogEntry{}, size, err
	}

	return entries, size, nil
}

func auditLogFromDatagram(datagram structs.Datagram, result any, err error) AuditLogEntry {
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	return AuditLogEntry{
		Pattern:   datagram.Pattern,
		Payload:   datagram.Payload,
		CreatedAt: datagram.CreatedAt,
		User:      datagram.User,
		Workspace: datagram.Workspace,
		Error:     errStr,
		Result:    result,
	}
}

func Diff(oldObj, newObj *unstructured.Unstructured) (string, error) {
	modified := []byte{}
	original := []byte{}
	err := error(nil)
	ns := ""
	resourceName := ""

	if oldObj != nil {
		ns = oldObj.GetNamespace()
		resourceName = oldObj.GetName()
	} else if newObj != nil {
		ns = newObj.GetNamespace()
		resourceName = newObj.GetName()
	} else {
		return "", fmt.Errorf("both oldObj and newObj are nil, cannot create diff")
	}

	tempDir := os.TempDir()
	originalPath := tempDir + "/original.yaml"
	modifiedPath := tempDir + "/modified.yaml"

	defer func() {
		_ = os.Remove(originalPath)
		_ = os.Remove(modifiedPath)
	}()

	if oldObj != nil {
		oldObj = removeUnusedFields(oldObj)
		original, err = yaml.Marshal(oldObj.Object)
		if err != nil {
			return "", fmt.Errorf("failed to marshal original data: %w", err)
		}
	}
	err = os.WriteFile(originalPath, original, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write original data to file: %w", err)
	}

	if newObj != nil {
		newObj = removeUnusedFields(newObj)
		modified, err = yaml.Marshal(newObj.Object)
		if err != nil {
			return "", fmt.Errorf("failed to marshal modified data: %w", err)
		}
	}
	err = os.WriteFile(modifiedPath, modified, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write modified data to file: %w", err)
	}

	diff, err := unifiedDiff(originalPath, modifiedPath, ns, resourceName)
	if err != nil {
		return "", fmt.Errorf("failed to create unified diff: %w", err)
	}
	return diff, nil
}

func unifiedDiff(filePath1 string, filePath2 string, ns, resourceName string) (string, error) {
	cmd := exec.Command("diff", "-u", "-N", "--label", ns+"/"+resourceName, "--label", ns+"/"+resourceName, filePath1, filePath2)
	cmd.Dir = os.TempDir()
	out, err := cmd.CombinedOutput()

	if err != nil {
		// diff returns exit code 1 if files differ
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 {
				return string(out), nil
			} else {
				return "", fmt.Errorf("Error running diff: %s\n%s\n", err.Error(), string(out))
			}
		} else {
			return "", err
		}
	}
	return "", nil
}

func removeUnusedFields(obj *unstructured.Unstructured) *unstructured.Unstructured {
	if obj == nil {
		return obj
	}

	obj.SetManagedFields(nil)
	unstructured.RemoveNestedField(obj.Object, "status")
	unstructured.RemoveNestedField(obj.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(obj.Object, "metadata", "generation")
	unstructured.RemoveNestedField(obj.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(obj.Object, "metadata", "uid")
	unstructured.RemoveNestedField(obj.Object, "metadata", "creationTimestamp")

	return obj
}
