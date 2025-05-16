package dtos

import v1 "k8s.io/api/core/v1"

type K8sProbeHTTPHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type K8sProbeHTTPGet struct {
	Path        string                `json:"path,omitempty"`
	Port        string                `json:"port"`
	Host        *string               `json:"host,omitempty"`
	Scheme      *v1.URIScheme         `json:"scheme,omitempty"`
	HTTPHeaders *[]K8sProbeHTTPHeader `json:"httpHeaders,omitempty"`
}

type K8sProbeTCPSocket struct {
	Port string `json:"port"`
}

type K8sProbeExec struct {
	Command []string `json:"command"`
}

type K8sProbeGRPC struct {
	Port    int     `json:"port"`
	Service *string `json:"service,omitempty"`
}

type K8sProbe struct {
	IsActive            bool               `json:"isActive"`
	InitialDelaySeconds int                `json:"initialDelaySeconds,omitempty"`
	PeriodSeconds       int                `json:"periodSeconds,omitempty"`
	TimeoutSeconds      int                `json:"timeoutSeconds,omitempty"`
	SuccessThreshold    int                `json:"successThreshold,omitempty"`
	FailureThreshold    int                `json:"failureThreshold,omitempty"`
	HTTPGet             *K8sProbeHTTPGet   `json:"httpGet,omitempty"`
	TCPSocket           *K8sProbeTCPSocket `json:"tcpSocket,omitempty"`
	Exec                *K8sProbeExec      `json:"exec,omitempty"`
	GRPC                *K8sProbeGRPC      `json:"grpc,omitempty"`
}

type K8sProbes struct {
	IsActive       bool      `json:"isActive"`
	LivenessProbe  *K8sProbe `json:"livenessProbe,omitempty"`
	ReadinessProbe *K8sProbe `json:"readinessProbe,omitempty"`
	StartupProbe   *K8sProbe `json:"startupProbe,omitempty"`
}
