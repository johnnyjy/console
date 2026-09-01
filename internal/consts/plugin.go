package consts

// BuiltInPluginName 对应 Java 的 BuiltInPluginName
const (
	PluginAiPromptDecorator   = "ai-prompt-decorator"
	PluginAiPromptTemplate    = "ai-prompt-template"
	PluginAiRag               = "ai-rag"
	PluginAiSecurityGuard     = "ai-security-guard"
	PluginAiStatistics        = "ai-statistics"
	PluginAiTokenRatelimit    = "ai-token-ratelimit"
	PluginAiTransformer       = "ai-transformer"
	PluginAiCache             = "ai-cache"
	PluginAiProxy             = "ai-proxy"
	PluginAiHistory           = "ai-history"
	PluginAiIntent            = "ai-intent"
	PluginAiQuota             = "ai-quota"
	PluginAiAgent             = "ai-agent"
	PluginModelRouter         = "model-router"
	PluginModelMapper         = "model-mapper"
	PluginDefaultMcp          = "mcp-server"
	PluginBasicAuth           = "basic-auth"
	PluginKeyAuth             = "key-auth"
	PluginOidc                = "oidc"
	PluginJwtAuth             = "jwt-auth"
	PluginHmacAuth            = "hmac-auth"
	PluginExtAuth             = "ext-auth"
	PluginCustomResponse      = "custom-response"
	PluginTransformer         = "transformer"
	PluginCacheControl        = "cache-control"
	PluginDeGraphql           = "de-graphql"
	PluginGeoIp               = "geo-ip"
	PluginFrontendGray        = "frontend-gray"
	PluginRequestBlock        = "request-block"
	PluginKeyRateLimit        = "key-rate-limit"
	PluginClusterKeyRateLimit = "cluster-key-rate-limit"
	PluginIpRestriction       = "ip-restriction"
	PluginRequestValidation   = "request-validation"
	PluginBotDetect           = "bot-detect"
	PluginWaf                 = "waf"
	PluginCors                = "cors"
)

// Plugin config 键常量
const (
	// AiProxyConfig
	AiProxyActiveProviderId            = "activeProviderId"
	AiProxyProviders                   = "providers"
	AiProxyProviderId                  = "id"
	AiProxyProviderType                = "type"
	AiProxyProviderApiTokens           = "apiTokens"
	AiProxyProtocol                    = "protocol"
	AiProxyFailover                    = "failover"
	AiProxyFailoverEnabled             = "enabled"
	AiProxyFailoverFailureThr          = "failureThreshold"
	AiProxyFailoverSuccessThr          = "successThreshold"
	AiProxyFailoverHealthCheckInterval = "healthCheckInterval"
	AiProxyFailoverHealthCheckTimeout  = "healthCheckTimeout"
	AiProxyFailoverHealthCheckModel    = "healthCheckModel"
	AiProxyRetryOnFailure              = "retryOnFailure"
	AiProxyRetryEnabled                = "enabled"

	// AiStatisticsConfig
	AiStatisticsAttributes              = "attributes"
	AiStatisticsUseDefaultAttributes    = "use_default_attributes"
	AiStatisticsUseDefaultResponseAttrs = "use_default_response_attributes"
	AiStatisticsKey                     = "key"
	AiStatisticsValueSource             = "value_source"
	AiStatisticsValue                   = "value"
	AiStatisticsRule                    = "rule"
	AiStatisticsApplyToLog              = "apply_to_log"
	AiStatisticsApplyToSpan             = "apply_to_span"

	// KeyAuthConfig
	KeyAuthConsumers     = "consumers"
	KeyAuthConsumerName  = "name"
	KeyAuthConsumerCreds = "credentials"
	KeyAuthConsumerCred  = "credential"
	KeyAuthKeys          = "keys"
	KeyAuthInHeader      = "in_header"
	KeyAuthInQuery       = "in_query"
	KeyAuthAllow         = "allow"
	KeyAuthGlobalAuth    = "global_auth"

	// ModelMapperConfig
	ModelMapperModelMapping = "modelMapping"

	// ModelRouterConfig
	ModelRouterModelToHeader = "modelToHeader"
)

// ProductCoveredPlugin 对应 Java 的 ProductCoveredPlugin
const (
	ProductCoveredAiProxy     = "ai-proxy"
	ProductCoveredModelRouter = "model-router"
	ProductCoveredModelMapper = "model-mapper"
	ProductCoveredMcpServer   = "mcp-server"
	ProductCoveredKeyAuth     = "key-auth"
)

// PluginCategory 对应 Java 的 PluginCategory
const (
	PluginCategoryAuth        = "auth"
	PluginCategorySecurity    = "security"
	PluginCategoryProtocol    = "protocol"
	PluginCategoryFlowControl = "flow-control"
	PluginCategoryFlowMonitor = "flow-monitor"
	PluginCategoryCustom      = "custom"
)

// Language 对应 Java 的 Language
const (
	LanguageEnUS = "en-US"
	LanguageZhCN = "zh-CN"
)

// PluginPhase 对应 Java 的 PluginPhase 名称
const (
	PluginPhaseUnspecified = "UNSPECIFIED_PHASE"
)
