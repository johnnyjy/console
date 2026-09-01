package model

// AiUpstream 对应 Java 的 AiUpstream
type AiUpstream struct {
	Provider     *string           `json:"provider,omitempty"`
	Weight       *int              `json:"weight,omitempty"`
	ModelMapping map[string]string `json:"modelMapping,omitempty"`
}

// AiModelPredicate 对应 Java 的 AiModelPredicate
type AiModelPredicate struct {
	MatchType     *string `json:"matchType,omitempty"`
	MatchValue    *string `json:"matchValue,omitempty"`
	CaseSensitive *bool   `json:"caseSensitive,omitempty"`
}

// AiRouteFallbackConfig 对应 Java 的 AiRouteFallbackConfig
type AiRouteFallbackConfig struct {
	Enabled          *bool        `json:"enabled,omitempty"`
	Upstreams        []AiUpstream `json:"upstreams,omitempty"`
	FallbackStrategy *string      `json:"fallbackStrategy,omitempty"`
	ResponseCodes    []string     `json:"responseCodes,omitempty"`
}

// AiRouteFallbackStrategy 常量
const (
	AiRouteFallbackRandom   = "RAND"
	AiRouteFallbackSequence = "SEQ"
)

// AiRoute 对应 Java 的 AiRoute
type AiRoute struct {
	Name               *string                  `json:"name,omitempty"`
	Version            *string                  `json:"version,omitempty"`
	Domains            []string                 `json:"domains,omitempty"`
	PathPredicate      *RoutePredicate          `json:"pathPredicate,omitempty"`
	HeaderPredicates   []KeyedRoutePredicate    `json:"headerPredicates,omitempty"`
	UrlParamPredicates []KeyedRoutePredicate    `json:"urlParamPredicates,omitempty"`
	Upstreams          []AiUpstream             `json:"upstreams,omitempty"`
	ModelPredicates    []AiModelPredicate       `json:"modelPredicates,omitempty"`
	AuthConfig         *RouteAuthConfig         `json:"authConfig,omitempty"`
	FallbackConfig     *AiRouteFallbackConfig   `json:"fallbackConfig,omitempty"`
	Cors               *CorsConfig              `json:"cors,omitempty"`
	HeaderControl      *HeaderControlConfig     `json:"headerControl,omitempty"`
	ProxyNextUpstream  *ProxyNextUpstreamConfig `json:"proxyNextUpstream,omitempty"`
	CustomConfigs      map[string]string        `json:"customConfigs,omitempty"`
	CustomLabels       map[string]string        `json:"customLabels,omitempty"`
}

// LlmProviderProtocol 常量
const (
	LlmProviderProtocolOpenaiV1 = "openai/v1"
	LlmProviderProtocolOriginal = "original"
	LlmProviderProtocolDefault  = LlmProviderProtocolOpenaiV1
)

// TokenFailoverConfig 对应 Java 的 TokenFailoverConfig
type TokenFailoverConfig struct {
	Enabled             *bool   `json:"enabled,omitempty"`
	FailureThreshold    *int    `json:"failureThreshold,omitempty"`
	SuccessThreshold    *int    `json:"successThreshold,omitempty"`
	HealthCheckInterval *int    `json:"healthCheckInterval,omitempty"`
	HealthCheckTimeout  *int    `json:"healthCheckTimeout,omitempty"`
	HealthCheckModel    *string `json:"healthCheckModel,omitempty"`
}

// LlmProviderEndpoint 对应 Java 的 LlmProviderEndpoint
type LlmProviderEndpoint struct {
	Protocol    *string `json:"protocol,omitempty"`
	Address     *string `json:"address,omitempty"`
	Port        *int    `json:"port,omitempty"`
	ContextPath *string `json:"contextPath,omitempty"`
}

// LlmProvider 对应 Java 的 LlmProvider
type LlmProvider struct {
	Name                *string              `json:"name,omitempty"`
	Type                *string              `json:"type,omitempty"`
	Protocol            *string              `json:"protocol,omitempty"`
	ProxyName           *string              `json:"proxyName,omitempty"`
	Tokens              []string             `json:"tokens,omitempty"`
	TokenFailoverConfig *TokenFailoverConfig `json:"tokenFailoverConfig,omitempty"`
	RawConfigs          map[string]any       `json:"rawConfigs,omitempty"`
}

// LlmProviderType 对应 Java 的 LlmProviderType
const (
	LlmProviderTypeQwen       = "qwen"
	LlmProviderTypeOpenai     = "openai"
	LlmProviderTypeMoonshot   = "moonshot"
	LlmProviderTypeAzure      = "azure"
	LlmProviderTypeAi360      = "ai360"
	LlmProviderTypeGithub     = "github"
	LlmProviderTypeGroq       = "groq"
	LlmProviderTypeBaichuan   = "baichuan"
	LlmProviderTypeYi         = "yi"
	LlmProviderTypeDeepseek   = "deepseek"
	LlmProviderTypeZhipuai    = "zhipuai"
	LlmProviderTypeOllama     = "ollama"
	LlmProviderTypeClaude     = "claude"
	LlmProviderTypeBaidu      = "baidu"
	LlmProviderTypeHunyuan    = "hunyuan"
	LlmProviderTypeStepfun    = "stepfun"
	LlmProviderTypeMinimax    = "minimax"
	LlmProviderTypeCloudflare = "cloudflare"
	LlmProviderTypeSpark      = "spark"
	LlmProviderTypeGemini     = "gemini"
	LlmProviderTypeDeepl      = "deepl"
	LlmProviderTypeMistral    = "mistral"
	LlmProviderTypeCohere     = "cohere"
	LlmProviderTypeDoubao     = "doubao"
	LlmProviderTypeCoze       = "coze"
	LlmProviderTypeTogetherAi = "together-ai"
	LlmProviderTypeBedrock    = "bedrock"
	LlmProviderTypeVertex     = "vertex"
	LlmProviderTypeOpenrouter = "openrouter"
	LlmProviderTypeGrok       = "grok"
	LlmProviderTypeVllm       = "vllm"
)

// LlmProviderProtocolFromValue 将协议 value 转为内部表示，未知返回空串
func LlmProviderProtocolFromValue(value string) string {
	switch value {
	case LlmProviderProtocolOpenaiV1, LlmProviderProtocolOriginal:
		return value
	}
	return ""
}

// LlmProviderProtocolFromPluginValue 将 ai-proxy 插件内的协议值转为对外 value，未知返回空串
func LlmProviderProtocolFromPluginValue(pluginValue string) string {
	switch pluginValue {
	case "openai":
		return LlmProviderProtocolOpenaiV1
	case "original":
		return LlmProviderProtocolOriginal
	}
	return ""
}

// LlmProviderProtocolPluginValue 将对外 value 转为 ai-proxy 插件内的协议值
func LlmProviderProtocolPluginValue(value string) string {
	switch value {
	case LlmProviderProtocolOpenaiV1:
		return "openai"
	case LlmProviderProtocolOriginal:
		return "original"
	}
	return "openai"
}
