package kubernetes

import (
	"testing"

	v1 "k8s.io/api/core/v1"
)

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
