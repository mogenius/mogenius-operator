package k8sexec

import (
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
)

// WithImpersonate exits the process on an unknown subject kind, and the
// subject comes from a User CRD whose schema does not constrain it.
func TestValidateImpersonationSubject(t *testing.T) {
	valid := []rbacv1.Subject{
		{Kind: "User", Name: "jane", APIGroup: rbacv1.GroupName},
		{Kind: "Group", Name: "ops", APIGroup: rbacv1.GroupName},
		{Kind: "ServiceAccount", Name: "runner", Namespace: "mogenius"},
	}
	for _, subject := range valid {
		if err := ValidateImpersonationSubject(subject); err != nil {
			t.Errorf("valid %s subject rejected: %v", subject.Kind, err)
		}
	}

	invalid := map[string]rbacv1.Subject{
		"empty kind":               {Name: "jane", APIGroup: rbacv1.GroupName},
		"lowercase kind":           {Kind: "user", Name: "jane", APIGroup: rbacv1.GroupName},
		"unknown kind":             {Kind: "Robot", Name: "jane"},
		"user without name":        {Kind: "User", APIGroup: rbacv1.GroupName},
		"user with wrong apigroup": {Kind: "User", Name: "jane"},
		"user with namespace":      {Kind: "User", Name: "jane", APIGroup: rbacv1.GroupName, Namespace: "default"},
		"sa without namespace":     {Kind: "ServiceAccount", Name: "runner"},
		"sa with apigroup":         {Kind: "ServiceAccount", Name: "runner", Namespace: "mogenius", APIGroup: rbacv1.GroupName},
	}
	for name, subject := range invalid {
		if err := ValidateImpersonationSubject(subject); err == nil {
			t.Errorf("%s was accepted; WithImpersonate would exit the process", name)
		}
	}
}

// A request without an identity must be refused before any cluster call:
// the stub provider has no CRD client, so reaching it would panic.
func TestResolveClientsRefusesAnonymousRequest(t *testing.T) {
	if _, err := ResolveClients(nil, nil, "mogenius", true, Identity{}); err == nil {
		t.Fatal("nil provider was accepted")
	}
}

// The operator's own namespace is admin-only on every pod path; other
// namespaces and an unknown own namespace are left to the regular checks.
func TestCheckOwnNamespace(t *testing.T) {
	tests := []struct {
		name      string
		user      Identity
		own       string
		namespace string
		wantErr   bool
	}{
		{"non-admin in own namespace", Identity{Email: "dev@example.com"}, "mogenius", "mogenius", true},
		{"admin in own namespace", Identity{Email: "admin@example.com", IsAdmin: true}, "mogenius", "mogenius", false},
		{"non-admin elsewhere", Identity{Email: "dev@example.com"}, "mogenius", "team-a", false},
		{"own namespace unknown", Identity{Email: "dev@example.com"}, "", "mogenius", false},
		{"empty target namespace", Identity{Email: "dev@example.com"}, "mogenius", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckOwnNamespace(tt.user, tt.own, tt.namespace)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CheckOwnNamespace() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
