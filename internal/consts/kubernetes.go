package consts

// Kubernetes 常量（对应 Java 的 KubernetesConstants）
const (
	KubeSystemNs      = "kube-system"
	K8sCert           = "cert"
	K8sEnableHttps    = "enableHttps"
	DataField         = "data"
	TypeField         = "type"
	SecretTypeTLS     = "kubernetes.io/tls"
	SecretTlsCrtField = "tls.crt"
	SecretTlsKeyField = "tls.key"
	YamlSeparator     = "---\n"
	HigressConfigName = "higress-config"
)

// Annotation 常量（对应 KubernetesConstants.Annotation）
const (
	AnnotationKeyPrefix                      = "higress.io/"
	AnnotationNginxIngressKeyPrefix          = "nginx.ingress.kubernetes.io/"
	AnnotationDisabledKeyExtraPrefix         = "disabled."
	AnnotationTrueValue                      = "true"
	AnnotationUseRegexKey                    = "higress.io/use-regex"
	AnnotationDestinationKey                 = "higress.io/destination"
	AnnotationSslRedirectKey                 = "higress.io/ssl-redirect"
	AnnotationRewriteEnabledKey              = "higress.io/enable-rewrite"
	AnnotationRewritePathKey                 = "higress.io/rewrite-path"
	AnnotationRewriteTargetKey               = "higress.io/rewrite-target"
	AnnotationUpstreamVhostKey               = "higress.io/upstream-vhost"
	AnnotationProxyNextUpstreamEnabledKey    = "higress.io/enable-proxy-next-upstream"
	AnnotationProxyNextUpstreamTriesKey      = "higress.io/proxy-next-upstream-tries"
	AnnotationProxyNextUpstreamTimeoutKey    = "higress.io/proxy-next-upstream-timeout"
	AnnotationProxyNextUpstreamKey           = "higress.io/proxy-next-upstream"
	AnnotationHeaderControlEnabledKey        = "higress.io/enable-header-control"
	AnnotationRequestHeaderControlAddKey     = "higress.io/request-header-control-add"
	AnnotationRequestHeaderControlUpdateKey  = "higress.io/request-header-control-update"
	AnnotationRequestHeaderControlRemoveKey  = "higress.io/request-header-control-remove"
	AnnotationResponseHeaderControlAddKey    = "higress.io/response-header-control-add"
	AnnotationResponseHeaderControlUpdateKey = "higress.io/response-header-control-update"
	AnnotationResponseHeaderControlRemoveKey = "higress.io/response-header-control-remove"
	AnnotationCorsEnabledKey                 = "higress.io/enable-cors"
	AnnotationCorsAllowOriginKey             = "higress.io/cors-allow-origin"
	AnnotationCorsAllowMethodsKey            = "higress.io/cors-allow-methods"
	AnnotationCorsAllowHeadersKey            = "higress.io/cors-allow-headers"
	AnnotationCorsExposeHeadersKey           = "higress.io/cors-expose-headers"
	AnnotationCorsAllowCredentialsKey        = "higress.io/cors-allow-credentials"
	AnnotationCorsMaxAgeKey                  = "higress.io/cors-max-age"
	AnnotationQueryMatchKeyword              = "-match-query-"
	AnnotationHeaderMatchKeyword             = "-match-header-"
	AnnotationPseudoHeaderMatchKeyword       = "-match-pseudo-header-"
	AnnotationQueryMatchKeyFormat            = "higress.io/%s-match-query-%s"
	AnnotationHeaderMatchKeyFormat           = "higress.io/%s-match-header-%s"
	AnnotationPseudoHeaderMatchKeyFormat     = "higress.io/%s-match-pseudo-header-%s"
	AnnotationMethodKey                      = "higress.io/match-method"
	AnnotationIgnorePathCaseKey              = "higress.io/ignore-path-case"
	AnnotationWasmPluginTitleKey             = "higress.io/wasm-plugin-title"
	AnnotationWasmPluginDescriptionKey       = "higress.io/wasm-plugin-description"
	AnnotationWasmPluginIconKey              = "higress.io/wasm-plugin-icon"
	AnnotationCommentKey                     = "higress.io/comment"
)

// Label 常量（对应 KubernetesConstants.Label）
const (
	LabelDomainKeyPrefix       = "higress.io/domain_"
	LabelDomainValueDummy      = "true"
	LabelConfigMapTypeKey      = "higress.io/config-map-type"
	LabelConfigMapTypeDomain   = "domain"
	LabelConfigMapTypeAiRoute  = "ai-route"
	LabelResourceDefinerKey    = "higress.io/resource-definer"
	LabelInternalKey           = "higress.io/internal"
	LabelResourceDefinerValue  = "higress"
	LabelWasmPluginNameKey     = "higress.io/wasm-plugin-name"
	LabelWasmPluginVersionKey  = "higress.io/wasm-plugin-version"
	LabelWasmPluginBuiltInKey  = "higress.io/wasm-plugin-built-in"
	LabelWasmPluginCategoryKey = "higress.io/wasm-plugin-category"
	LabelResourceBizTypeKey    = "higress.io/biz-type"
)

// IngressPathType 常量
const (
	IngressPathTypeExact                  = "Exact"
	IngressPathTypePrefix                 = "Prefix"
	IngressPathTypeImplementationSpecific = "ImplementationSpecific"
)
