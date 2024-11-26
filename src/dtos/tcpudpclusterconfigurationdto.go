package dtos

type TcpUdpClusterConfigurationDto struct {
	IngressServices interface{} `json:"ingressServices"`
	TcpServices     interface{} `json:"tcpServices"`
	UdpServices     interface{} `json:"udpServices"`
}
