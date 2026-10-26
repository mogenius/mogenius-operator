package services

import (
	"context"
	"fmt"
	"mogenius-operator/src/k8sexec"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UserRbacConfigKey switches on the RBAC check for the access paths that
// act with the operator's own identity: files v2, the web terminal without
// a session, the pod log stream and port-forward tunnels to workloads.
// Exec requests, terminal sessions and SSH already run impersonated; these
// paths need the operator's exec, so instead of running as the user they ask
// the cluster whether the user may do the equivalent (pods/exec, pods/log,
// pods/portforward) first. Off by default: users whose Grants do not cover
// it today would lose access.
const UserRbacConfigKey = "MO_ENFORCE_USER_RBAC_FOR_FILES_LOGS_TUNNELS"

const userAccessReviewTimeout = 10 * time.Second

// AuthorizeUserPodAccess checks, as the requesting user, whether they may
// perform verb on pods/subresource in namespace (pod "" = any pod there).
// Admin requests pass under the same rule as exec (admin bypass); everyone
// else is resolved to their User CRD subject like ResolveClients does and
// asked via a SelfSubjectAccessReview. It is a no-op unless
// UserRbacConfigKey is enabled.
func AuthorizeUserPodAccess(identity k8sexec.Identity, namespace, pod, subresource, verb string) error {
	if config == nil {
		return nil
	}
	if enforce, _ := config.TryGetBool(UserRbacConfigKey); !enforce {
		return nil
	}
	allowAdminBypass, _ := config.TryGetBool(execAdminBypassConfigKey)
	if identity.IsAdmin && allowAdminBypass {
		return nil
	}
	clients, err := k8sexec.ResolveClients(serviceLogger, clientProvider, config.Get("MO_OWN_NAMESPACE"), allowAdminBypass, identity)
	if err != nil {
		return fmt.Errorf("access denied: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), userAccessReviewTimeout)
	defer cancel()
	review, err := clients.Clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace:   namespace,
				Verb:        verb,
				Resource:    "pods",
				Subresource: subresource,
				Name:        pod,
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("check access of %q: %w", identity.Email, err)
	}
	if !review.Status.Allowed {
		target := "pods in namespace " + namespace
		if pod != "" {
			target = "pod " + namespace + "/" + pod
		}
		return fmt.Errorf("access denied: %q may not %s pods/%s on %s", identity.Email, verb, subresource, target)
	}
	return nil
}
