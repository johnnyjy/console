package kubernetes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// McpBridge 常量（对应 V1McpBridge）
const (
	McpBridgeAPIGroup    = "networking.higress.io"
	McpBridgeVersion     = "v1"
	McpBridgeKind        = "McpBridge"
	McpBridgePlural      = "mcpbridges"
	McpBridgeDefaultName = "default"

	McpBridgeRegistryTypeNacos                 = "nacos"
	McpBridgeRegistryTypeNacos2                = "nacos2"
	McpBridgeRegistryTypeNacos3                = "nacos3"
	McpBridgeRegistryTypeNacosGroups           = "nacosGroups"
	McpBridgeRegistryTypeNacosNamespaceId      = "nacosNamespaceId"
	McpBridgeRegistryTypeNacosUsername         = "nacosUsername"
	McpBridgeRegistryTypeNacosPassword         = "nacosPassword"
	McpBridgeRegistryTypeZk                    = "zookeeper"
	McpBridgeRegistryTypeZkServicesPath        = "zkServicesPath"
	McpBridgeRegistryTypeConsul                = "consul"
	McpBridgeRegistryTypeConsulDataCenter      = "consulDatacenter"
	McpBridgeRegistryTypeConsulServiceTag      = "consulServiceTag"
	McpBridgeRegistryTypeConsulToken           = "consulToken"
	McpBridgeRegistryTypeConsulRefreshInterval = "consulRefreshInterval"
	McpBridgeRegistryTypeEureka                = "eureka"
	McpBridgeRegistryTypeStatic                = "static"
	McpBridgeRegistryTypeDNS                   = "dns"

	McpBridgeMcpServerExportDomains = "mcpServerExportDomains"
	McpBridgeMcpServerBaseUrl       = "mcpServerBaseUrl"
	McpBridgeEnableMcpServer        = "enableMCPServer"

	McpBridgeProtocolHttp  = "http"
	McpBridgeProtocolHttps = "https"
	McpBridgeProtocolGrpc  = "grpc"
	McpBridgeProtocolGrpcs = "grpcs"
	McpBridgeProxyTypeHttp = "http"
	McpBridgeStaticPort    = 80
)

// WasmPlugin 常量（对应 V1alpha1WasmPlugin）
const (
	WasmPluginAPIGroup = "extensions.higress.io"
	WasmPluginVersion  = "v1alpha1"
	WasmPluginKind     = "WasmPlugin"
	WasmPluginPlural   = "wasmplugins"
)

// EnvoyFilter 常量（对应 V1alpha3EnvoyFilter）
const (
	EnvoyFilterAPIGroup = "networking.istio.io"
	EnvoyFilterVersion  = "v1alpha3"
	EnvoyFilterKind     = "EnvoyFilter"
	EnvoyFilterPlural   = "envoyfilters"
)

// McpBridge 对应 V1McpBridge
type McpBridge struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              McpBridgeSpec `json:"spec,omitempty"`
}

// McpBridgeSpec 对应 V1McpBridgeSpec
type McpBridgeSpec struct {
	Registries []RegistryConfig `json:"registries,omitempty"`
	Proxies    []ProxyConfig    `json:"proxies,omitempty"`
}

// RegistryConfig 对应 V1RegistryConfig
type RegistryConfig struct {
	Protocol               string                            `json:"protocol,omitempty"`
	Sni                    string                            `json:"sni,omitempty"`
	Type                   string                            `json:"type,omitempty"`
	Name                   string                            `json:"name,omitempty"`
	Domain                 string                            `json:"domain,omitempty"`
	Port                   *int                              `json:"port,omitempty"`
	ZkServicesPath         []string                          `json:"zkServicesPath,omitempty"`
	NacosNamespaceId       string                            `json:"nacosNamespaceId,omitempty"`
	NacosGroups            []string                          `json:"nacosGroups,omitempty"`
	ConsulDataCenter       string                            `json:"consulDataCenter,omitempty"`
	ConsulServiceTag       string                            `json:"consulServiceTag,omitempty"`
	ConsulRefreshInterval  *int                              `json:"consulRefreshInterval,omitempty"`
	EnableMcpServer        *bool                             `json:"enableMCPServer,omitempty"`
	EnableScopeMcpServers  *bool                             `json:"enableScopeMcpServers,omitempty"`
	AllowMcpServers        []string                          `json:"allowMcpServers,omitempty"`
	McpServerBaseUrl       string                            `json:"mcpServerBaseUrl,omitempty"`
	McpServerExportDomains []string                          `json:"mcpServerExportDomains,omitempty"`
	AuthSecretName         string                            `json:"authSecretName,omitempty"`
	Metadata               map[string]RegistryConfigMetadata `json:"metadata,omitempty"`
	ProxyName              string                            `json:"proxyName,omitempty"`
	VPort                  *VPort                            `json:"vport,omitempty"`
}

// RegistryConfigMetadata 对应 V1RegistryConfigMetadata
type RegistryConfigMetadata struct {
	InnerMap map[string]string `json:"innerMap,omitempty"`
}

// ProxyConfig 对应 V1ProxyConfig
type ProxyConfig struct {
	Type           string `json:"type,omitempty"`
	Name           string `json:"name,omitempty"`
	ServerAddress  string `json:"serverAddress,omitempty"`
	ServerPort     *int   `json:"serverPort,omitempty"`
	ListenerPort   *int   `json:"listenerPort,omitempty"`
	ConnectTimeout *int   `json:"connectTimeout,omitempty"`
}

// VPort 对应 VPort
type VPort struct {
	DefaultValue *int           `json:"default,omitempty"`
	Services     []ServiceVPort `json:"services,omitempty"`
}

// ServiceVPort 对应 VPort.ServiceVport
type ServiceVPort struct {
	Name  string `json:"name,omitempty"`
	Value *int   `json:"value,omitempty"`
}

// WasmPlugin 对应 V1alpha1WasmPlugin
type WasmPlugin struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              WasmPluginSpec `json:"spec,omitempty"`
}

// WasmPluginSpec 对应 V1alpha1WasmPluginSpec
type WasmPluginSpec struct {
	DefaultConfigDisable *bool                  `json:"defaultConfigDisable,omitempty"`
	DefaultConfig        map[string]interface{} `json:"defaultConfig,omitempty"`
	ImagePullPolicy      string                 `json:"imagePullPolicy,omitempty"`
	ImagePullSecret      string                 `json:"imagePullSecret,omitempty"`
	MatchRules           []MatchRule            `json:"matchRules,omitempty"`
	Phase                string                 `json:"phase,omitempty"`
	Priority             *int                   `json:"priority,omitempty"`
	Sha256               string                 `json:"sha256,omitempty"`
	URL                  string                 `json:"url,omitempty"`
	VerificationKey      string                 `json:"verificationKey,omitempty"`
	FailStrategy         string                 `json:"failStrategy,omitempty"`
}

// MatchRule 对应 crd/wasm/MatchRule
type MatchRule struct {
	ConfigDisable *bool                  `json:"configDisable,omitempty"`
	Config        map[string]interface{} `json:"config,omitempty"`
	Domain        []string               `json:"domain,omitempty"`
	Ingress       []string               `json:"ingress,omitempty"`
	Service       []string               `json:"service,omitempty"`
}

// ForDomain 对应 MatchRule.forDomain
func MatchRuleForDomain(domain string) MatchRule {
	return MatchRule{Domain: []string{domain}}
}

// ForIngress 对应 MatchRule.forIngress
func MatchRuleForIngress(ingress string) MatchRule {
	return MatchRule{Ingress: []string{ingress}}
}

// ForService 对应 MatchRule.forService
func MatchRuleForService(service string) MatchRule {
	return MatchRule{Service: []string{service}}
}

// KeyEquals 对应 MatchRule.keyEquals
func (m MatchRule) KeyEquals(rule MatchRule) bool {
	return equalsUnordered(m.Domain, rule.Domain) &&
		equalsUnordered(m.Ingress, rule.Ingress) &&
		equalsUnordered(m.Service, rule.Service)
}

// IsEmpty 对应 MatchRule.isEmpty
func (m MatchRule) IsEmpty() bool {
	return len(m.Domain) == 0 && len(m.Ingress) == 0 && len(m.Service) == 0
}

func equalsUnordered(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, v := range a {
		counts[v]++
	}
	for _, v := range b {
		if counts[v] == 0 {
			return false
		}
		counts[v]--
	}
	return true
}
