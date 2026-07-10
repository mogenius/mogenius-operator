package kubernetes

import (
	"testing"
	"time"

	"mogenius-operator/src/utils"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeService(namespace, name string, ports ...v1.ServicePort) v1.Service {
	return v1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec:       v1.ServiceSpec{Ports: ports},
	}
}

func TestMatchServiceByPriority(t *testing.T) {
	tests := []struct {
		name         string
		services     []v1.Service
		candidates   []serviceCandidate
		wantService  string
		wantPort     int32
		wantBasePath string
		wantErr      bool
	}{
		{
			name: "kube-prometheus-stack exact name wins over VictoriaMetrics",
			services: []v1.Service{
				makeService("vm", "vmsingle-vm-stack", v1.ServicePort{Name: "http", Port: 8429}),
				makeService("monitoring", "kube-prometheus-stack-prometheus", v1.ServicePort{Name: "http-web", Port: 9090}),
			},
			candidates:  prometheusCandidates,
			wantService: "kube-prometheus-stack-prometheus",
			wantPort:    9090,
		},
		{
			name: "victoria-metrics single-node matched by prefix",
			services: []v1.Service{
				makeService("vm", "vmagent-vm-stack", v1.ServicePort{Name: "http", Port: 8429}),
				makeService("vm", "vmsingle-vm-stack", v1.ServicePort{Name: "http", Port: 8429}),
			},
			candidates:  prometheusCandidates,
			wantService: "vmsingle-vm-stack",
			wantPort:    8429,
		},
		{
			name: "victoria-metrics cluster mode gets the vmselect tenant base path",
			services: []v1.Service{
				makeService("vm", "vminsert-vm-stack", v1.ServicePort{Name: "http", Port: 8480}),
				makeService("vm", "vmselect-vm-stack", v1.ServicePort{Name: "http", Port: 8481}),
			},
			candidates:   prometheusCandidates,
			wantService:  "vmselect-vm-stack",
			wantPort:     8481,
			wantBasePath: "/select/0/prometheus",
		},
		{
			name: "vmsingle preferred over vmselect when both exist",
			services: []v1.Service{
				makeService("vm", "vmselect-vm-stack", v1.ServicePort{Name: "http", Port: 8481}),
				makeService("vm", "vmsingle-vm-stack", v1.ServicePort{Name: "http", Port: 8429}),
			},
			candidates:  prometheusCandidates,
			wantService: "vmsingle-vm-stack",
			wantPort:    8429,
		},
		{
			name: "victoria-metrics alertmanager matched by prefix",
			services: []v1.Service{
				makeService("vm", "vmalertmanager-vm-stack", v1.ServicePort{Name: "http", Port: 9093}),
			},
			candidates:  alertmanagerCandidates,
			wantService: "vmalertmanager-vm-stack",
			wantPort:    9093,
		},
		{
			name: "multiple prefix matches pick the lexicographically smallest name",
			services: []v1.Service{
				makeService("vm-b", "vmsingle-stack-b", v1.ServicePort{Name: "http", Port: 8429}),
				makeService("vm-a", "vmsingle-stack-a", v1.ServicePort{Name: "http", Port: 8429}),
			},
			candidates:  prometheusCandidates,
			wantService: "vmsingle-stack-a",
			wantPort:    8429,
		},
		{
			name: "services without ports are ignored",
			services: []v1.Service{
				makeService("vm", "vmsingle-vm-stack"),
			},
			candidates: prometheusCandidates,
			wantErr:    true,
		},
		{
			name:       "no match returns an error",
			services:   []v1.Service{makeService("default", "kubernetes", v1.ServicePort{Name: "https", Port: 443})},
			candidates: prometheusCandidates,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := clusterServicesCached
			clusterServicesCached = utils.NewTTLCache(time.Minute, func() []v1.Service { return tt.services })
			t.Cleanup(func() { clusterServicesCached = orig })

			_, service, port, basePath, err := matchServiceByPriority(tt.candidates, "prometheus")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("matchServiceByPriority() expected error, got service %q", service)
				}
				return
			}
			if err != nil {
				t.Fatalf("matchServiceByPriority() unexpected error: %v", err)
			}
			if service != tt.wantService || port != tt.wantPort || basePath != tt.wantBasePath {
				t.Errorf("matchServiceByPriority() = (%q, %d, %q), want (%q, %d, %q)",
					service, port, basePath, tt.wantService, tt.wantPort, tt.wantBasePath)
			}
		})
	}
}

func TestSelectHTTPPort(t *testing.T) {
	tests := []struct {
		name  string
		ports []v1.ServicePort
		want  int32
	}{
		{
			name:  "single port",
			ports: []v1.ServicePort{{Name: "http-web", Port: 9090}},
			want:  9090,
		},
		{
			name: "prefers 9090 over a sidecar port listed first",
			ports: []v1.ServicePort{
				{Name: "grpc", Port: 10901},
				{Name: "http-web", Port: 9090},
			},
			want: 9090,
		},
		{
			name: "prefers alertmanager 9093",
			ports: []v1.ServicePort{
				{Name: "cluster", Port: 9094},
				{Name: "web", Port: 9093},
			},
			want: 9093,
		},
		{
			name: "falls back to web-named port when no standard port present",
			ports: []v1.ServicePort{
				{Name: "grpc", Port: 10901},
				{Name: "reloader-web", Port: 8080},
			},
			want: 8080,
		},
		{
			name: "falls back to first port when nothing matches",
			ports: []v1.ServicePort{
				{Name: "grpc", Port: 10901},
				{Name: "metrics", Port: 8081},
			},
			want: 10901,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := selectHTTPPort(tt.ports); got != tt.want {
				t.Errorf("selectHTTPPort() = %d, want %d", got, tt.want)
			}
		})
	}
}
