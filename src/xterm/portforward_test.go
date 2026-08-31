package xterm

import "testing"

// RemotePort used to carry a blanket `validate:"required"`, which rejected a
// kind=ssh request that correctly omitted it — before the handler ran, and
// with nothing the user could act on. Namespace had the opposite problem: no
// tag at all, yet dereferenced on the SSH path.
func TestPortForwardConnectionRequestValidate(t *testing.T) {
	tests := []struct {
		name    string
		request PortForwardConnectionRequest
		wantErr bool
	}{
		{
			// The hostname grammar makes the kind optional, so an ssh request
			// legitimately carries neither kind nor port. A `required` struct
			// tag on either rejected exactly this before the handler ran.
			name:    "ssh with neither kind nor port is valid",
			request: PortForwardConnectionRequest{Mode: "ssh", Namespace: "test2", WorkloadName: "claude-sandbox"},
		},
		{
			name:    "ssh with an explicit kind is valid",
			request: PortForwardConnectionRequest{Mode: "ssh", Kind: "deployment", Namespace: "test2", WorkloadName: "sandbox"},
		},
		{
			name:    "legacy kind=ssh spelling is still accepted",
			request: PortForwardConnectionRequest{Kind: "ssh", Namespace: "prod", WorkloadName: "api"},
		},
		{
			name:    "ssh without a namespace is rejected",
			request: PortForwardConnectionRequest{Mode: "ssh", WorkloadName: "api"},
			wantErr: true,
		},
		{
			name:    "a non-ssh request still needs a kind",
			request: PortForwardConnectionRequest{Namespace: "prod", WorkloadName: "api", RemotePort: 80},
			wantErr: true,
		},
		{
			name:    "pod needs a port",
			request: PortForwardConnectionRequest{Kind: "pod", Namespace: "prod", WorkloadName: "api"},
			wantErr: true,
		},
		{
			name:    "pod with a port is valid",
			request: PortForwardConnectionRequest{Kind: "pod", Namespace: "prod", WorkloadName: "api", RemotePort: 8080},
		},
		{
			name:    "host needs no namespace but does need a port",
			request: PortForwardConnectionRequest{Kind: "host", WorkloadName: "10.0.0.5", RemotePort: 5432},
		},
		{
			name:    "host without a port is rejected",
			request: PortForwardConnectionRequest{Kind: "host", WorkloadName: "10.0.0.5"},
			wantErr: true,
		},
		{
			name:    "kind casing does not change the rules",
			request: PortForwardConnectionRequest{Kind: "SSH", Namespace: "prod", WorkloadName: "api"},
		},
		{
			name:    "whitespace is not a namespace",
			request: PortForwardConnectionRequest{Kind: "pod", Namespace: "   ", WorkloadName: "api", RemotePort: 80},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.request.validate()
			if tc.wantErr && err == nil {
				t.Fatal("expected the request to be rejected")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected the request to be accepted, got: %v", err)
			}
		})
	}
}
