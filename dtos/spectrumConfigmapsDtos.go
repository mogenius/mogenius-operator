package dtos

type SpectrumConfigmapDto struct {
	IngressServices interface{} `json:"ingressServices"`
	TcpServices     interface{} `json:"tcpServices"`
	UdpServices     interface{} `json:"udpServices"`
}
