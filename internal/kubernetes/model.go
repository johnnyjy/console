package kubernetes

// RegistryzService 对应 model.RegistryzService
type RegistryzService struct {
	Attributes *RegistryzServiceAttributes `json:"Attributes"`
	Hostname   string                      `json:"hostname"`
	Ports      []Port                      `json:"ports"`
}

// RegistryzServiceAttributes 对应 model.RegistryzServiceAttributes
type RegistryzServiceAttributes struct {
	ServiceRegistry string `json:"ServiceRegistry"`
	Name            string `json:"Name"`
	Namespace       string `json:"Namespace"`
}

// Port 对应 model.Port
type Port struct {
	Name     string `json:"name"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}

// IstioEndpointShard 对应 model.IstioEndpointShard
type IstioEndpointShard struct {
	Shards map[string][]IstioEndpoint `json:"Shards"`
}

// IstioEndpoint 对应 model.IstioEndpoint
type IstioEndpoint struct {
	Labels    map[string]string `json:"Labels"`
	Addresses []string          `json:"Addresses"`
}

// Address 兼容旧调用，返回第一个地址
func (e IstioEndpoint) Address() string {
	if len(e.Addresses) == 0 {
		return ""
	}
	return e.Addresses[0]
}
