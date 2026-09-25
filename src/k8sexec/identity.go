package k8sexec

import (
	"fmt"
	"log/slog"

	"mogenius-operator/src/k8sclient"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Clients are the Kubernetes clients one request — or one tunnel — uses for
// every API call it triggers. They are impersonated for regular users so the
// kube-apiserver enforces the user's own RBAC grants, and the operator's own
// for admins where the bypass is allowed.
type Clients struct {
	RestConfig *rest.Config
	Clientset  kubernetes.Interface
}

// Identity is the platform user behind a request.
//
// Both fields are set by the platform API and reach the operator over its
// authenticated control connection, in the same JSON frame as the request
// payload. They therefore carry the platform's trust and no separate one:
// treat them as "the API decided", not as an independent check. What
// actually constrains a request inside the cluster is the impersonated
// identity ResolveClients binds it to.
type Identity struct {
	Email string
	// IsAdmin is the platform's judgement that the requester is an
	// organization admin or holds a CLUSTER_ADMIN scope for this cluster.
	IsAdmin bool
}

// ResolveClients maps a platform user to Kubernetes clients. Admin requests
// may use the operator identity, unless allowAdminBypass is off; everyone
// else must have a User CRD in ownNamespace with an RBAC subject — otherwise
// the request is rejected (fail closed). The SSH gateway and the exec-request
// service share this so a shell and a one-off command run under the same
// rules.
func ResolveClients(logger *slog.Logger, provider k8sclient.K8sClientProvider, ownNamespace string, allowAdminBypass bool, user Identity) (*Clients, error) {
	if provider == nil {
		return nil, fmt.Errorf("no kubernetes client provider configured")
	}
	if user.IsAdmin {
		if !allowAdminBypass {
			// The admin marking comes from the platform over the control
			// connection, so it is only as trustworthy as that connection.
			// With the bypass off, such a request still has to name a user
			// the cluster itself knows.
			if logger != nil {
				logger.Info("admin bypass is disabled; falling back to impersonation", "email", user.Email)
			}
		} else {
			return &Clients{RestConfig: provider.ClientConfig(), Clientset: provider.K8sClientSet()}, nil
		}
	}
	if user.Email == "" {
		return nil, fmt.Errorf("request carries no user identity")
	}

	users, err := provider.MogeniusClientSet().MogeniusV1alpha1.ListUsers(ownNamespace)
	if err != nil {
		return nil, fmt.Errorf("list user CRDs: %w", err)
	}
	for i := range users {
		if users[i].Spec.Email != user.Email {
			continue
		}
		if users[i].Spec.Subject == nil {
			return nil, fmt.Errorf("user %q has no RBAC subject on its User CRD", user.Email)
		}
		if err := ValidateImpersonationSubject(*users[i].Spec.Subject); err != nil {
			return nil, fmt.Errorf("user %q has an unusable RBAC subject: %w", user.Email, err)
		}
		impersonated, err := provider.WithImpersonate(*users[i].Spec.Subject)
		if err != nil {
			return nil, fmt.Errorf("impersonate %q: %w", user.Email, err)
		}
		return &Clients{RestConfig: impersonated.ClientConfig(), Clientset: impersonated.K8sClientSet()}, nil
	}
	return nil, fmt.Errorf("no User CRD found for %q", user.Email)
}

// ValidateImpersonationSubject rejects a Subject that WithImpersonate cannot
// handle. Its unknown-kind branch calls assert.Assert(false, ...), which
// exits the process — and the subject comes from a User CRD whose schema
// declares kind as a plain string, so a malformed or hand-edited CRD would
// otherwise take the whole operator down on the first request.
func ValidateImpersonationSubject(subject rbacv1.Subject) error {
	switch subject.Kind {
	case "User", "Group":
		if subject.Name == "" {
			return fmt.Errorf("%s subject has no name", subject.Kind)
		}
		if subject.APIGroup != rbacv1.GroupName {
			return fmt.Errorf("%s subject must use apiGroup %q, got %q", subject.Kind, rbacv1.GroupName, subject.APIGroup)
		}
		if subject.Namespace != "" {
			return fmt.Errorf("%s subject must not set a namespace", subject.Kind)
		}
	case "ServiceAccount":
		if subject.Name == "" || subject.Namespace == "" {
			return fmt.Errorf("ServiceAccount subject needs both name and namespace")
		}
		if subject.APIGroup != "" {
			return fmt.Errorf("ServiceAccount subject must not set an apiGroup, got %q", subject.APIGroup)
		}
	default:
		return fmt.Errorf("unsupported subject kind %q (expected User, Group or ServiceAccount)", subject.Kind)
	}
	return nil
}
