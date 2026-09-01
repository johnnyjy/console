package consts

// SystemConfigKey 对应 Java 的 SystemConfigKey
const (
	configKeyPrefix = "higress-console."

	VersionKey                     = configKeyPrefix + "build.version"
	DevBuildKey                    = configKeyPrefix + "build.dev"
	DevBuildDefault                = true
	NsKey                          = configKeyPrefix + "ns"
	KubeConfigKey                  = configKeyPrefix + "kube-config"
	ControllerAccessTokenKey       = configKeyPrefix + "controller.access-token"
	ControllerJwtPolicyKey         = configKeyPrefix + "controller.jwt-policy"
	ControllerServicePortKey       = configKeyPrefix + "controller.service.port"
	ControllerServiceHostKey       = configKeyPrefix + "controller.service.host"
	ControllerWatchedNsKey         = configKeyPrefix + "controller.watched-namespace"
	ControllerIngressClassKey      = configKeyPrefix + "controller.ingress-class-name"
	ControllerServiceNameKey       = configKeyPrefix + "controller.service.name"
	ConsoleServiceHostKey          = configKeyPrefix + "service.host"
	DefaultConsoleServiceHost      = "higress-console.higress-system.svc.cluster.local"
	ConsoleServicePortKey          = configKeyPrefix + "service.port"
	DefaultConsoleServicePort      = 8080
	ConfigMapNameKey               = configKeyPrefix + "config-map.name"
	ConfigMapNameDefault           = "higress-console"
	SecretNameKey                  = configKeyPrefix + "secret.name"
	SecretNameDefault              = "higress-console"
	AdminCookieNameKey             = configKeyPrefix + "admin.cookie.name"
	AdminCookieNameDefault         = "_hi_sess"
	AdminCookieMaxAgeKey           = configKeyPrefix + "admin.cookie.max-age"
	AdminCookieMaxAgeDefault       = 30 * 24 * 60 * 60
	AdminConfigTtlKey              = configKeyPrefix + "admin.config-ttl"
	AdminConfigTtlDefault          = 10 * 1000
	DashboardOverwriteStartup      = configKeyPrefix + "dashboard.overwrite-when-startup"
	DashboardOverwriteStartupDef   = true
	DashboardBaseUrlKey            = configKeyPrefix + "dashboard.base-url"
	DashboardUsernameKey           = configKeyPrefix + "dashboard.username"
	DashboardUsernameDefault       = "admin"
	DashboardPasswordKey           = configKeyPrefix + "dashboard.password"
	DashboardPasswordDefault       = "admin"
	DashboardDsPromNameKey         = configKeyPrefix + "dashboard.datasource.prom.name"
	DashboardDsPromNameDefault     = "Prometheus"
	DashboardDsPromUrlKey          = configKeyPrefix + "dashboard.datasource.prom.url"
	DashboardDsLokiNameKey         = configKeyPrefix + "dashboard.datasource.loki.name"
	DashboardDsLokiNameDefault     = "Loki"
	DashboardDsLokiUrlKey          = configKeyPrefix + "dashboard.datasource.loki.url"
	DashboardProxyConnTimeout      = configKeyPrefix + "dashboard.proxy.connection-timeout"
	DashboardProxyConnTimeoutDef   = 1200
	DashboardProxySocketTimeout    = configKeyPrefix + "dashboard.proxy.socket-timeout"
	DashboardProxySocketTimeoutDef = 2 * 60 * 1000
	AiProxyServiceUrlKey           = configKeyPrefix + "ai-proxy.service.url"
	AiProxyServiceTokenKey         = configKeyPrefix + "ai-proxy.service.token"
	AiProxyConnTimeoutKey          = configKeyPrefix + "ai-proxy.connection-timeout"
	AiProxyConnTimeoutDefault      = 1200
	AiProxySocketTimeoutKey        = configKeyPrefix + "ai-proxy.socket-timeout"
	AiProxySocketTimeoutDefault    = 2 * 60 * 1000
	ClusterDomainSuffixEnv         = "CLUSTER_DOMAIN_SUFFIX"
)

// UserConfigKey 对应 Java 的 UserConfigKey
const (
	DefaultRouteInitialized     = "route.default.initialized"
	SystemInitialized           = "system.initialized"
	LoginPagePromptKey          = "login.prompt"
	DashboardUrl                = "dashboard.url"
	DashboardUrlPrefix          = "dashboard.url."
	DashboardBuiltin            = "dashboard.builtin"
	ChatEnabled                 = "chat.enabled"
	AdminPasswordChangeDisabled = "admin.password-change.disabled"
)

// CapabilityKey 对应 Java 的 CapabilityKey
const (
	CapabilityConfigIngressV1 = "config.ingress.v1"
)
