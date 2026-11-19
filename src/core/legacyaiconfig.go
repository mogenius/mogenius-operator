package core

import (
	"context"
	"fmt"
	"log/slog"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/kubernetes"
	"mogenius-operator/src/shutdown"
	"mogenius-operator/src/utils"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	k8s "k8s.io/client-go/kubernetes"
)

// LEGACY_AI_CONFIG_SECRET_NAME is the Secret the AI manager read its provider
// settings from until the AiModel CRs replaced it (2026-07). It holds the
// provider API key and possibly a GitHub PAT in plain text, and nothing reads
// it any more.
const LEGACY_AI_CONFIG_SECRET_NAME = "mogenius-ai-config"

const legacyAiConfigCleanupTimeout = 30 * time.Second

// RemoveLegacyAiConfigSecret deletes the leftover mogenius-ai-config Secret in
// the operator namespace. A credential nobody uses is only something to steal:
// until v2.33.0 every Secret of the operator namespace was readable through the
// unauthenticated /socketapi, and the platform used to send this one to every
// browser that opened the AI settings.
//
// The Secret stays when an AiModel, McpServer or PlatformConfig still names it
// (the "use an existing Secret" option lists every Secret of the namespace, so a
// migration may well have pointed a model at it), and whenever the references
// cannot be read: deleting a key a model still uses would break it silently.
// Runs once per leadership; a failure is logged and retried on the next one.
func RemoveLegacyAiConfigSecret(logger *slog.Logger, configModule cfg.ConfigModule, clientProvider k8sclient.K8sClientProvider) {
	ctx, cancel := context.WithTimeout(context.Background(), legacyAiConfigCleanupTimeout)
	defer cancel()
	shutdown.Add(cancel)

	err := removeLegacyAiConfigSecret(ctx, logger, configModule.Get("MO_OWN_NAMESPACE"), clientProvider.K8sClientSet(), clientProvider.DynamicClient())
	if err != nil {
		logger.Error("remove legacy AI config secret: giving up until the next leadership change", "secret", LEGACY_AI_CONFIG_SECRET_NAME, "error", err)
	}
}

func removeLegacyAiConfigSecret(ctx context.Context, logger *slog.Logger, namespace string, clientset k8s.Interface, dynamicClient dynamic.Interface) error {
	secrets := clientset.CoreV1().Secrets(namespace)
	secret, err := secrets.Get(ctx, LEGACY_AI_CONFIG_SECRET_NAME, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read secret: %w", err)
	}

	referencedBy, err := legacyAiConfigSecretReferences(ctx, namespace, dynamicClient)
	if err != nil {
		return err
	}
	if len(referencedBy) > 0 {
		logger.Warn("legacy AI config secret is still referenced and is kept; move the key into the referencing object's own secret to have it removed",
			"secret", LEGACY_AI_CONFIG_SECRET_NAME, "namespace", namespace, "referencedBy", referencedBy)
		return nil
	}

	// the UID precondition keeps a Secret that was deleted and re-created in
	// between (by a user, after the reference check) from being removed
	uid := secret.GetUID()
	err = secrets.Delete(ctx, LEGACY_AI_CONFIG_SECRET_NAME, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete secret: %w", err)
	}
	logger.Info("removed legacy AI config secret (unused since AiModel CRs replaced it)", "secret", LEGACY_AI_CONFIG_SECRET_NAME, "namespace", namespace)
	return nil
}

// legacyAiConfigSecretReferences lists the objects (kind/name) whose spec names
// the legacy secret. A kind whose CRD is not served yet has no objects and so
// no references; every other read failure is an error, never "unreferenced".
func legacyAiConfigSecretReferences(ctx context.Context, namespace string, dynamicClient dynamic.Interface) ([]string, error) {
	sources := []utils.ResourceDescriptor{
		utils.AiModelResource,
		utils.McpServerResource,
		utils.PlatformConfigResource,
	}

	var referencedBy []string
	for _, source := range sources {
		gvr := kubernetes.CreateGroupVersionResource(source.ApiVersion, source.Plural)
		var list *unstructured.UnstructuredList
		var err error
		if source.Namespaced {
			list, err = dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
		} else {
			list, err = dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
		}
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", source.Plural, err)
		}
		for _, item := range list.Items {
			spec, found, _ := unstructured.NestedMap(item.Object, "spec")
			if found && namesSecret(spec, LEGACY_AI_CONFIG_SECRET_NAME) {
				referencedBy = append(referencedBy, source.Kind+"/"+item.GetName())
			}
		}
	}
	return referencedBy, nil
}

// secretReferenceFields are the spec fields (lower case) that point at a Secret
// by name: AiModel apiKeySecretRef, McpServer secretRef/valueFrom and
// PlatformConfig secretRef/secret.
var secretReferenceFields = map[string]struct{}{
	"apikeysecretref": {},
	"secretref":       {},
	"valuefrom":       {},
	"secret":          {},
}

// namesSecret walks a spec and reports whether any secret reference field
// carries the given name. Walking the whole spec instead of fixed paths keeps
// references in lists (McpServer headers/env) and future fields covered.
func namesSecret(value any, name string) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if _, isRef := secretReferenceFields[strings.ToLower(key)]; isRef {
				if ref, ok := child.(map[string]any); ok && ref["name"] == name {
					return true
				}
			}
			if namesSecret(child, name) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if namesSecret(child, name) {
				return true
			}
		}
	}
	return false
}
