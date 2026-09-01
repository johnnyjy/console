package consts

// HigressConstants 对应 Java 的 HigressConstants
const (
	NsDefault                     = "higress-system"
	ControllerServiceNameDefault  = "higress-controller"
	ControllerIngressClassName    = "higress"
	NginxIngressClassName         = "nginx"
	ControllerServiceHostDefault  = "localhost"
	ControllerServicePortDefault  = 15014
	ControllerJwtPolicyDefault    = KubernetesJwtPolicyThirdParty
	DefaultDomain                 = "higress-default-domain"
	InternalResourceNameSuffix    = ".internal"
	FallbackRouteNameSuffix       = ".fallback"
	FallbackFromHeader            = "x-higress-fallback-from"
	ModelRoutingHeader            = "x-higress-llm-model"
	InternalResourceComment       = "PLEASE DO NOT EDIT DIRECTLY. This resource is managed by Higress."
	ServiceListSupportRegistryDef = true
	ClusterDomainSuffixDefault    = "cluster.local"
	FallbackResponseCode4xx       = "4xx"
	FallbackResponseCode5xx       = "5xx"
)

// KubernetesJwtPolicy 常量
const (
	KubernetesJwtPolicyFirstParty = "first-party-jwt"
	KubernetesJwtPolicyThirdParty = "third-party-jwt"
)
