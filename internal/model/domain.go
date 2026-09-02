package model

// Domain 对应 Java 的 Domain
type Domain struct {
	Name           *string `json:"name,omitempty"`
	Version        *string `json:"version,omitempty"`
	EnableHttps    *string `json:"enableHttps,omitempty"`
	CertIdentifier *string `json:"certIdentifier,omitempty"`
}

func (d *Domain) GetVersion() *string  { return d.Version }
func (d *Domain) SetVersion(v *string) { d.Version = v }

// Domain EnableHttps 常量
const (
	DomainHttpsOff   = "off"
	DomainHttpsOn    = "on"
	DomainHttpsForce = "force"
)

// Service 对应 Java 的 Service
type Service struct {
	Name      *string  `json:"name,omitempty"`
	Namespace *string  `json:"namespace,omitempty"`
	Port      *int     `json:"port,omitempty"`
	Version   *int     `json:"version,omitempty"`
	Endpoints []string `json:"endpoints,omitempty"`
	Protocol  *string  `json:"protocol,omitempty"`
}

// TlsCertificate 对应 Java 的 TlsCertificate
type TlsCertificate struct {
	Name          *string  `json:"name,omitempty"`
	Version       *string  `json:"version,omitempty"`
	Cert          *string  `json:"cert,omitempty"`
	Key           *string  `json:"key,omitempty"`
	Domains       []string `json:"domains,omitempty"`
	ValidityStart *string  `json:"validityStart,omitempty"`
	ValidityEnd   *string  `json:"validityEnd,omitempty"`
}

func (t *TlsCertificate) GetVersion() *string  { return t.Version }
func (t *TlsCertificate) SetVersion(v *string) { t.Version = v }

// ProxyServer 对应 Java 的 ProxyServer
type ProxyServer struct {
	Type           *string `json:"type,omitempty"`
	Name           *string `json:"name,omitempty"`
	Version        *string `json:"version,omitempty"`
	ServerAddress  *string `json:"serverAddress,omitempty"`
	ServerPort     *int    `json:"serverPort,omitempty"`
	ConnectTimeout *int    `json:"connectTimeout,omitempty"`
}

// VPort 对应 Java 的 VPort
type VPort struct {
	Default  *int           `json:"default,omitempty"`
	Services []ServiceVport `json:"services,omitempty"`
}

// ServiceVport 对应 Java 的 VPort.ServiceVport
type ServiceVport struct {
	Name  *string `json:"name,omitempty"`
	Value *int    `json:"value,omitempty"`
}

// ServiceSourceAuthN 对应 Java 的 ServiceSourceAuthN
type ServiceSourceAuthN struct {
	Enabled    *bool             `json:"enabled,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
}

// ServiceSource 对应 Java 的 ServiceSource
type ServiceSource struct {
	Name       *string             `json:"name,omitempty"`
	Vport      *VPort              `json:"vport,omitempty"`
	Version    *string             `json:"version,omitempty"`
	Type       *string             `json:"type,omitempty"`
	Domain     *string             `json:"domain,omitempty"`
	Port       *int                `json:"port,omitempty"`
	Protocol   *string             `json:"protocol,omitempty"`
	Sni        *string             `json:"sni,omitempty"`
	ProxyName  *string             `json:"proxyName,omitempty"`
	Properties map[string]any      `json:"properties"`
	AuthN      *ServiceSourceAuthN `json:"authN,omitempty"`
}

func (s *ServiceSource) GetVersion() *string  { return s.Version }
func (s *ServiceSource) SetVersion(v *string) { s.Version = v }
