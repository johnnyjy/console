package sdk

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"console/internal/consts"
	"console/internal/errs"
	k8s "console/internal/kubernetes"
	"console/internal/model"
)

// ---- OpenAI ----

const (
	openaiCustomUrlKey          = "openaiCustomUrl"
	openaiExtraCustomUrlsKey    = "openaiExtraCustomUrls"
	openaiCustomServiceNameKey  = "openaiCustomServiceName"
	openaiCustomServicePortKey  = "openaiCustomServicePort"
	openaiDefaultServiceDomain  = "api.openai.com"
	openaiDefaultServiceContext = "/v1"
)

func newOpenaiHandler() *llmProviderHandler {
	return &llmProviderHandler{
		typ:      model.LlmProviderTypeOpenai,
		needSync: true,
		normalize: func(configurations map[string]any) {
			if len(configurations) == 0 {
				return
			}
			uris := getCustomUrisWithKeys(configurations, openaiCustomUrlKey, openaiExtraCustomUrlsKey)
			for _, uri := range uris {
				validateUriScheme(uri)
			}
		},
		buildServiceSource: func(providerName string, providerConfig map[string]any) *model.ServiceSource {
			if svc := openaiCustomUpstreamService(providerConfig); svc != nil {
				return nil
			}
			h := getProviderHandlers()[model.LlmProviderTypeOpenai]
			return h.defaultBuildServiceSource(providerName, providerConfig)
		},
		buildUpstreamService: func(providerName string, providerConfig map[string]any) *model.UpstreamService {
			if svc := openaiCustomUpstreamService(providerConfig); svc != nil {
				return svc
			}
			h := getProviderHandlers()[model.LlmProviderTypeOpenai]
			return h.defaultBuildUpstreamService(providerName, providerConfig)
		},
		endpoints: func(providerConfig map[string]any) []*model.LlmProviderEndpoint {
			uris := getCustomUrisWithKeys(providerConfig, openaiCustomUrlKey, openaiExtraCustomUrlsKey)
			if len(uris) == 0 {
				return []*model.LlmProviderEndpoint{{
					Protocol:    strPtr(k8s.McpBridgeProtocolHttps),
					Address:     strPtr(openaiDefaultServiceDomain),
					Port:        intPtr(443),
					ContextPath: strPtr(openaiDefaultServiceContext),
				}}
			}
			out := make([]*model.LlmProviderEndpoint, 0, len(uris))
			for _, uri := range uris {
				out = append(out, endpointFromUri(uri))
			}
			return out
		},
	}
}

func openaiCustomUpstreamService(providerConfig map[string]any) *model.UpstreamService {
	if len(providerConfig) == 0 {
		return nil
	}
	name, ok := providerConfig[openaiCustomServiceNameKey].(string)
	if !ok {
		return nil
	}
	port := anyIntOrNil(providerConfig[openaiCustomServicePortKey])
	if port == nil {
		return nil
	}
	return &model.UpstreamService{Name: strPtr(name), Port: port}
}

// ---- Qwen ----

const (
	qwenDefaultServiceDomain = "dashscope.aliyuncs.com"
	qwenCustomDomainKey      = "qwenDomain"
	qwenEnableSearchKey      = "qwenEnableSearch"
	qwenEnableCompatibleKey  = "qwenEnableCompatible"
	qwenFileIdsKey           = "qwenFileIds"
)

func newQwenHandler() *llmProviderHandler {
	return &llmProviderHandler{
		typ: model.LlmProviderTypeQwen,
		normalize: func(configurations map[string]any) {
			if configurations == nil {
				return
			}
			configurations[qwenEnableSearchKey] = anyBoolOr(configurations[qwenEnableSearchKey], false)
			configurations[qwenEnableCompatibleKey] = anyBoolOr(configurations[qwenEnableCompatibleKey], true)

			if fileIdsVal, ok := configurations[qwenFileIdsKey]; ok {
				if _, ok := fileIdsVal.([]any); !ok {
					panic(errs.Validation("Invalid configuration: " + qwenFileIdsKey))
				}
			}
		},
		endpoints: func(providerConfig map[string]any) []*model.LlmProviderEndpoint {
			if customUrl := qwenCustomUrl(providerConfig); customUrl != nil {
				return []*model.LlmProviderEndpoint{endpointFromUri(customUrl)}
			}
			return []*model.LlmProviderEndpoint{{
				Protocol:    strPtr(k8s.McpBridgeProtocolHttps),
				Address:     strPtr(qwenDefaultServiceDomain),
				Port:        intPtr(443),
				ContextPath: strPtr("/"),
			}}
		},
	}
}

func qwenCustomUrl(providerConfig map[string]any) *url.URL {
	raw, ok := providerConfig[qwenCustomDomainKey].(string)
	if !ok {
		return nil
	}
	domain := strings.TrimSpace(raw)
	if domain == "" {
		return nil
	}
	u, err := url.Parse(k8s.McpBridgeProtocolHttps + "://" + domain + "/")
	if err != nil {
		panic(errs.Validation(qwenCustomDomainKey + " contains an invalid domain name: " + domain))
	}
	return u
}

// ---- Azure ----

const (
	azureServiceUrlKey        = "azureServiceUrl"
	azureApiVersionQueryParam = "api-version"
)

func newAzureHandler() *llmProviderHandler {
	return &llmProviderHandler{
		typ: model.LlmProviderTypeAzure,
		normalize: func(configurations map[string]any) {
			uri := azureServiceUri(configurations)
			if uri.Scheme == "" {
				panic(errs.Validation("Azure service URL must have a scheme."))
			}
			scheme := strings.ToLower(uri.Scheme)
			if scheme != k8s.McpBridgeProtocolHttp && scheme != k8s.McpBridgeProtocolHttps {
				panic(errs.Validation("Azure service URL must have a valid scheme."))
			}
			if uri.Query().Get(azureApiVersionQueryParam) == "" {
				panic(errs.Validation("Azure service URL must have a non-empty " + azureApiVersionQueryParam +
					" query parameter."))
			}
		},
		endpoints: func(providerConfig map[string]any) []*model.LlmProviderEndpoint {
			return []*model.LlmProviderEndpoint{endpointFromUri(azureServiceUri(providerConfig))}
		},
	}
}

func azureServiceUri(providerConfig map[string]any) *url.URL {
	if len(providerConfig) == 0 {
		panic(errs.Validation("Missing Azure specific configurations."))
	}
	raw, ok := providerConfig[azureServiceUrlKey].(string)
	if !ok {
		panic(errs.Validation(azureServiceUrlKey + " must be a string."))
	}
	if strings.TrimSpace(raw) == "" {
		panic(errs.Validation(azureServiceUrlKey + " cannot be empty."))
	}
	u, err := url.Parse(raw)
	if err != nil {
		panic(errs.Validation(azureServiceUrlKey + " is not a valid URL."))
	}
	return u
}

// ---- Bedrock ----

const (
	bedrockAccessKeyKey = "awsAccessKey"
	bedrockSecretKeyKey = "awsSecretKey"
	bedrockRegionKey    = "awsRegion"
	bedrockDomainFormat = "bedrock-runtime.%s.amazonaws.com"
)

func newBedrockHandler() *llmProviderHandler {
	return &llmProviderHandler{
		typ: model.LlmProviderTypeBedrock,
		normalize: func(configurations map[string]any) {
			if len(configurations) == 0 {
				panic(errs.Validation("Missing Bedrock specific configurations."))
			}
			if strings.TrimSpace(anyStrOr(configurations[bedrockRegionKey], "")) == "" {
				panic(errs.Validation(bedrockRegionKey + " cannot be empty."))
			}
			if strings.TrimSpace(anyStrOr(configurations[bedrockAccessKeyKey], "")) == "" {
				panic(errs.Validation(bedrockAccessKeyKey + " cannot be empty."))
			}
			if strings.TrimSpace(anyStrOr(configurations[bedrockSecretKeyKey], "")) == "" {
				panic(errs.Validation(bedrockSecretKeyKey + " cannot be empty."))
			}
		},
		endpoints: func(providerConfig map[string]any) []*model.LlmProviderEndpoint {
			region := anyStrOr(providerConfig[bedrockRegionKey], "")
			if strings.TrimSpace(region) == "" {
				panic(errs.Validation(bedrockRegionKey + " cannot be empty."))
			}
			domain := "bedrock-runtime." + region + ".amazonaws.com"
			return []*model.LlmProviderEndpoint{{
				Protocol:    strPtr(k8s.McpBridgeProtocolHttps),
				Address:     strPtr(domain),
				Port:        intPtr(443),
				ContextPath: strPtr("/"),
			}}
		},
	}
}

// ---- Ollama ----

const (
	ollamaServerHostKey = "ollamaServerHost"
	ollamaServerPortKey = "ollamaServerPort"
	ollamaDefaultPort   = 11434
)

func newOllamaHandler() *llmProviderHandler {
	return &llmProviderHandler{
		typ:      model.LlmProviderTypeOllama,
		needSync: true,
		normalize: func(configurations map[string]any) {
			if len(configurations) == 0 {
				panic(errs.Validation("Missing Ollama specific configurations."))
			}
			host, ok := configurations[ollamaServerHostKey].(string)
			if !ok {
				panic(errs.Validation(ollamaServerHostKey + " must be a string."))
			}
			if strings.TrimSpace(host) == "" {
				panic(errs.Validation(ollamaServerHostKey + " cannot be empty."))
			}
			port := getIntConfig(configurations, ollamaServerPortKey)
			if !checkPort(port) {
				panic(errs.Validation(ollamaServerPortKey + " must be a valid port number."))
			}
			configurations[ollamaServerPortKey] = port
		},
		endpoints: func(providerConfig map[string]any) []*model.LlmProviderEndpoint {
			return []*model.LlmProviderEndpoint{{
				Protocol:    strPtr(k8s.McpBridgeProtocolHttp),
				Address:     strPtr(ollamaServiceDomain(providerConfig)),
				Port:        intPtr(ollamaServicePort(providerConfig)),
				ContextPath: strPtr("/"),
			}}
		},
	}
}

func ollamaServiceDomain(providerConfig map[string]any) string {
	if len(providerConfig) == 0 {
		return ""
	}
	return anyStrOr(providerConfig[ollamaServerHostKey], "")
}

func ollamaServicePort(providerConfig map[string]any) int {
	if len(providerConfig) == 0 {
		return ollamaDefaultPort
	}
	port := getIntConfig(providerConfig, ollamaServerPortKey)
	if checkPort(port) {
		return port
	}
	return ollamaDefaultPort
}

// ---- Vertex ----

const (
	vertexAuthKeyKey             = "vertexAuthKey"
	vertexRegionKey              = "vertexRegion"
	vertexProjectIdKey           = "vertexProjectId"
	vertexAuthServiceNameKey     = "vertexAuthServiceName"
	vertexTokenRefreshAheadKey   = "vertexTokenRefreshAhead"
	geminiSafetySettingKey       = "geminiSafetySetting"
	vertexGlobalRegion           = "global"
	vertexGlobalDomain           = "aiplatform.googleapis.com"
	vertexRegionalDomainFormat   = "%s-aiplatform.googleapis.com"
	vertexDefaultAuthServiceName = "vertex-auth" + consts.InternalResourceNameSuffix
	vertexAuthServiceDomain      = "oauth2.googleapis.com"
)

func newVertexHandler() *llmProviderHandler {
	return &llmProviderHandler{
		typ: model.LlmProviderTypeVertex,
		normalize: func(configurations map[string]any) {
			if len(configurations) == 0 {
				panic(errs.Validation("Missing Vertex specific configurations."))
			}
			region := anyStrOr(configurations[vertexRegionKey], "")
			if strings.TrimSpace(region) == "" {
				panic(errs.Validation(vertexRegionKey + " cannot be empty."))
			}
			configurations[vertexRegionKey] = strings.ToLower(region)

			if strings.TrimSpace(anyStrOr(configurations[vertexProjectIdKey], "")) == "" {
				panic(errs.Validation(vertexProjectIdKey + " cannot be empty."))
			}
			authKey := anyStrOr(configurations[vertexAuthKeyKey], "")
			if strings.TrimSpace(authKey) == "" {
				panic(errs.Validation(vertexAuthKeyKey + " cannot be empty."))
			}
			var authKeyObj map[string]any
			if err := json.Unmarshal([]byte(authKey), &authKeyObj); err != nil {
				panic(errs.Validation(vertexAuthKeyKey + " must contain a valid JSON object."))
			}
			for _, key := range []string{"client_email", "private_key_id", "private_key", "token_uri"} {
				if _, ok := authKeyObj[key].(string); !ok {
					panic(errs.Validation(vertexAuthKeyKey +
						" must contain a valid JSON object with a string property: " + key))
				}
			}

			if ahead := anyIntOrNil(configurations[vertexTokenRefreshAheadKey]); ahead != nil && *ahead < 0 {
				panic(errs.Validation(vertexTokenRefreshAheadKey + " must be a non-negative number."))
			}

			if raw, ok := configurations[geminiSafetySettingKey]; ok && raw != nil {
				geminiMap, ok := raw.(map[string]any)
				if !ok {
					panic(errs.Validation(geminiSafetySettingKey + " must be an object."))
				}
				for k, v := range geminiMap {
					if _, ok := v.(string); !ok {
						panic(errs.Validation(geminiSafetySettingKey +
							" must be an object with string key-value pairs."))
					}
					_ = k
				}
			}

			configurations[vertexAuthServiceNameKey] = vertexDefaultAuthServiceName
		},
		endpoints: func(providerConfig map[string]any) []*model.LlmProviderEndpoint {
			region := anyStrOr(providerConfig[vertexRegionKey], "")
			if strings.TrimSpace(region) == "" {
				panic(errs.Validation(vertexRegionKey + " cannot be empty."))
			}
			domain := vertexGlobalDomain
			if region != vertexGlobalRegion {
				domain = region + "-aiplatform.googleapis.com"
			}
			return []*model.LlmProviderEndpoint{{
				Protocol:    strPtr(k8s.McpBridgeProtocolHttps),
				Address:     strPtr(domain),
				Port:        intPtr(443),
				ContextPath: strPtr("/"),
			}}
		},
		extraServiceSources: func(providerName string, providerConfig map[string]any,
			forDelete bool) []*model.ServiceSource {
			if forDelete {
				return []*model.ServiceSource{}
			}
			return []*model.ServiceSource{{
				Name:     strPtr(vertexDefaultAuthServiceName),
				Type:     strPtr(k8s.McpBridgeRegistryTypeDNS),
				Protocol: strPtr(k8s.McpBridgeProtocolHttps),
				Port:     intPtr(443),
				Domain:   strPtr(vertexAuthServiceDomain),
			}}
		},
	}
}

// ---- vLLM ----

const (
	vllmCustomUrlKey       = "vllmCustomUrl"
	vllmExtraCustomUrlsKey = "vllmExtraCustomUrls"
)

func newVllmHandler() *llmProviderHandler {
	return &llmProviderHandler{
		typ:      model.LlmProviderTypeVllm,
		needSync: true,
		normalize: func(configurations map[string]any) {
			uris := getCustomUrisWithKeys(configurations, vllmCustomUrlKey, vllmExtraCustomUrlsKey)
			if len(uris) == 0 {
				panic(errs.Validation("No vLLM service URL is configured."))
			}
			for _, uri := range uris {
				validateUriScheme(uri)
			}
		},
		endpoints: func(providerConfig map[string]any) []*model.LlmProviderEndpoint {
			uris := getCustomUrisWithKeys(providerConfig, vllmCustomUrlKey, vllmExtraCustomUrlsKey)
			if len(uris) == 0 {
				panic(errs.Validation("No vLLM service URL is configured."))
			}
			out := make([]*model.LlmProviderEndpoint, 0, len(uris))
			for _, uri := range uris {
				out = append(out, endpointFromUri(uri))
			}
			return out
		},
	}
}

// ---- Zhipu AI ----

const (
	zhipuDefaultServiceDomain = "open.bigmodel.cn"
	zhipuDomainKey            = "zhipuDomain"
	zhipuCodePlanModeKey      = "zhipuCodePlanMode"
)

func newZhipuaiHandler() *llmProviderHandler {
	return &llmProviderHandler{
		typ: model.LlmProviderTypeZhipuai,
		normalize: func(configurations map[string]any) {
			if configurations == nil {
				return
			}
			configurations[zhipuCodePlanModeKey] = toBooleanDefault(configurations[zhipuCodePlanModeKey], true)
		},
		endpoints: func(providerConfig map[string]any) []*model.LlmProviderEndpoint {
			if domain := zhipuCustomDomain(providerConfig); domain != "" {
				u, err := url.Parse(k8s.McpBridgeProtocolHttps + "://" + domain + "/")
				if err != nil {
					panic(errs.Validation(zhipuDomainKey + " contains an invalid domain name: " + domain))
				}
				return []*model.LlmProviderEndpoint{endpointFromUri(u)}
			}
			return []*model.LlmProviderEndpoint{{
				Protocol:    strPtr(k8s.McpBridgeProtocolHttps),
				Address:     strPtr(zhipuDefaultServiceDomain),
				Port:        intPtr(443),
				ContextPath: strPtr("/"),
			}}
		},
	}
}

func zhipuCustomDomain(providerConfig map[string]any) string {
	if len(providerConfig) == 0 {
		return ""
	}
	raw, ok := providerConfig[zhipuDomainKey].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(raw)
}

// ---- Claude ----

const (
	claudeCustomUrlKey         = "claudeCustomUrl"
	claudeProviderDomainKey    = "providerDomain"
	claudeProviderBasePathKey  = "providerBasePath"
	claudeDefaultServiceDomain = "api.anthropic.com"
	claudeVersionKey           = "claudeVersion"
	claudeCodeModeKey          = "claudeCodeMode"
	claudeDefaultVersion       = "2023-06-01"
)

func newClaudeHandler() *llmProviderHandler {
	h := &llmProviderHandler{
		typ:      model.LlmProviderTypeClaude,
		needSync: true,
		loadConfig: func(provider *model.LlmProvider, configurations map[string]any) bool {
			base := getProviderHandlers()[model.LlmProviderTypeClaude]
			if !base.defaultLoadConfig(provider, configurations) {
				return false
			}
			rawConfigs := provider.RawConfigs
			if len(rawConfigs) == 0 {
				return true
			}
			endpoints := claudeGetProviderEndpoints(rawConfigs)
			if len(endpoints) == 0 {
				return true
			}
			endpoint := endpoints[0]
			if endpoint == nil || strings.TrimSpace(deref(endpoint.Protocol)) == "" ||
				strings.TrimSpace(deref(endpoint.Address)) == "" {
				return true
			}
			if claudeIsDefaultEndpoint(endpoint) {
				delete(rawConfigs, claudeCustomUrlKey)
				return true
			}
			path := deref(endpoint.ContextPath)
			if path == "" {
				path = "/"
			}
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			if strings.Trim(path, "/") == "" {
				path = ""
			}
			host := strings.TrimSpace(deref(endpoint.Address))
			port := derefInt(endpoint.Port)
			if port == 0 {
				if deref(endpoint.Protocol) == k8s.McpBridgeProtocolHttp {
					port = 80
				} else {
					port = 443
				}
			}
			omitPort := (deref(endpoint.Protocol) == k8s.McpBridgeProtocolHttp && port == 80) ||
				(deref(endpoint.Protocol) == k8s.McpBridgeProtocolHttps && port == 443)
			customUrl := deref(endpoint.Protocol) + "://" + host
			if !omitPort {
				customUrl += ":" + strconv.Itoa(port)
			}
			customUrl += path
			rawConfigs[claudeCustomUrlKey] = customUrl
			return true
		},
		normalize: func(configurations map[string]any) {
			if len(configurations) == 0 {
				return
			}
			if codeModeObj, ok := configurations[claudeCodeModeKey]; ok && codeModeObj != nil {
				if codeMode := toBooleanPtr(codeModeObj); codeMode != nil {
					configurations[claudeCodeModeKey] = *codeMode
				}
			}
			if versionObj, ok := configurations[claudeVersionKey]; !ok || versionObj == nil ||
				(stringValue(versionObj) != "" && strings.TrimSpace(stringValue(versionObj)) == "") {
				configurations[claudeVersionKey] = claudeDefaultVersion
			}
			legacyUris := claudeLegacyCustomUris(configurations)
			if len(legacyUris) > 0 {
				for _, uri := range legacyUris {
					validateUriScheme(uri)
					configurations[claudeProviderDomainKey] = uri.Hostname()
					path := uri.Path
					if path == "" {
						path = "/"
					}
					configurations[claudeProviderBasePathKey] = path
				}
				return
			}
			claudeValidateWasmDomainOverrides(configurations)
		},
		endpoints: claudeGetProviderEndpoints,
	}
	return h
}

func claudeGetProviderEndpoints(providerConfig map[string]any) []*model.LlmProviderEndpoint {
	legacyUris := claudeLegacyCustomUris(providerConfig)
	if len(legacyUris) > 0 {
		out := make([]*model.LlmProviderEndpoint, 0, len(legacyUris))
		for _, uri := range legacyUris {
			out = append(out, endpointFromUri(uri))
		}
		return out
	}
	if wasmEndpoints := claudeBuildEndpointsFromWasmDomainOverrides(providerConfig); len(wasmEndpoints) > 0 {
		return wasmEndpoints
	}
	return []*model.LlmProviderEndpoint{{
		Protocol:    strPtr(k8s.McpBridgeProtocolHttps),
		Address:     strPtr(claudeDefaultServiceDomain),
		Port:        intPtr(443),
		ContextPath: strPtr("/"),
	}}
}

func claudeIsDefaultEndpoint(endpoint *model.LlmProviderEndpoint) bool {
	if endpoint == nil {
		return true
	}
	return deref(endpoint.Protocol) == k8s.McpBridgeProtocolHttps &&
		deref(endpoint.Address) == claudeDefaultServiceDomain &&
		(endpoint.Port == nil || derefInt(endpoint.Port) == 443) &&
		(deref(endpoint.ContextPath) == "/" || strings.TrimSpace(deref(endpoint.ContextPath)) == "")
}

func claudeLegacyCustomUris(providerConfig map[string]any) []*url.URL {
	if len(providerConfig) == 0 {
		return nil
	}
	raw, ok := providerConfig[claudeCustomUrlKey].(string)
	if !ok {
		return nil
	}
	if strings.TrimSpace(raw) == "" {
		panic(errs.Validation(claudeCustomUrlKey + " cannot be empty."))
	}
	u, err := url.Parse(raw)
	if err != nil {
		panic(errs.Validation("Claude custom URL is invalid: " + raw))
	}
	return []*url.URL{u}
}

func claudeBuildEndpointsFromWasmDomainOverrides(providerConfig map[string]any) []*model.LlmProviderEndpoint {
	if len(providerConfig) == 0 {
		return nil
	}
	domain := anyStrOr(providerConfig[claudeProviderDomainKey], "")
	basePath := anyStrOr(providerConfig[claudeProviderBasePathKey], "")
	if strings.TrimSpace(domain) == "" && strings.TrimSpace(basePath) == "" {
		return nil
	}
	if strings.TrimSpace(domain) == "" {
		domain = claudeDefaultServiceDomain
	} else {
		domain = strings.TrimSpace(domain)
	}
	if strings.TrimSpace(basePath) == "" {
		basePath = "/"
	} else {
		basePath = strings.TrimSpace(basePath)
		if !strings.HasPrefix(basePath, "/") {
			basePath = "/" + basePath
		}
	}
	scheme := claudeResolveWasmScheme(providerConfig)
	uri, err := claudeBuildUri(scheme, domain, basePath)
	if err != nil {
		panic(errs.Validation("Invalid " + claudeProviderDomainKey + " / " + claudeProviderBasePathKey + ": " +
			domain + " / " + basePath))
	}
	return []*model.LlmProviderEndpoint{endpointFromUri(uri)}
}

func claudeResolveWasmScheme(providerConfig map[string]any) string {
	legacyUris := claudeLegacyCustomUris(providerConfig)
	if len(legacyUris) > 0 {
		if strings.EqualFold(legacyUris[0].Scheme, k8s.McpBridgeProtocolHttp) {
			return k8s.McpBridgeProtocolHttp
		}
		return k8s.McpBridgeProtocolHttps
	}
	return k8s.McpBridgeProtocolHttps
}

func claudeValidateWasmDomainOverrides(configurations map[string]any) {
	domain := anyStrOr(configurations[claudeProviderDomainKey], "")
	basePath := anyStrOr(configurations[claudeProviderBasePathKey], "")
	if strings.TrimSpace(domain) == "" && strings.TrimSpace(basePath) == "" {
		return
	}
	normDomain := claudeDefaultServiceDomain
	if strings.TrimSpace(domain) != "" {
		normDomain = strings.TrimSpace(domain)
	}
	normPath := "/"
	if strings.TrimSpace(basePath) != "" {
		normPath = strings.TrimSpace(basePath)
		if !strings.HasPrefix(normPath, "/") {
			normPath = "/" + normPath
		}
		if basePath != normPath {
			configurations[claudeProviderBasePathKey] = normPath
		}
	}
	if _, err := claudeBuildUri(claudeResolveWasmScheme(configurations), normDomain, normPath); err != nil {
		panic(errs.Validation("Invalid " + claudeProviderDomainKey + " / " + claudeProviderBasePathKey))
	}
}

func claudeBuildUri(scheme, domain, path string) (*url.URL, error) {
	host := strings.TrimSpace(domain)
	port := -1
	if idx := strings.LastIndex(host, ":"); idx > 0 && idx < len(host)-1 {
		maybePort := host[idx+1:]
		if isAllDigits(maybePort) && len(maybePort) <= 5 {
			host = host[:idx]
			port, _ = strconv.Atoi(maybePort)
		}
	}
	if port == -1 {
		if scheme == k8s.McpBridgeProtocolHttp {
			port = 80
		} else {
			port = 443
		}
	}
	return &url.URL{Scheme: scheme, Host: host + ":" + strconv.Itoa(port), Path: path}, nil
}

// ---- 通用辅助 ----

func getCustomUrisWithKeys(providerConfig map[string]any, customUrlKey, extraCustomUrlsKey string) []*url.URL {
	if len(providerConfig) == 0 {
		return nil
	}
	raw, ok := providerConfig[customUrlKey].(string)
	if !ok {
		return nil
	}
	if strings.TrimSpace(raw) == "" {
		panic(errs.Validation(customUrlKey + " cannot be empty."))
	}

	customUrls := []string{raw}
	if extraObj, ok := providerConfig[extraCustomUrlsKey].([]any); ok && len(extraObj) > 0 {
		for _, extra := range extraObj {
			if s, ok := extra.(string); ok && strings.TrimSpace(s) != "" {
				customUrls = append(customUrls, s)
			} else {
				panic(errs.Validation(extraCustomUrlsKey + " must contain non-empty strings."))
			}
		}
	}

	uris := make([]*url.URL, 0, len(customUrls))
	for _, customUrl := range customUrls {
		u, err := url.Parse(customUrl)
		if err != nil {
			panic(errs.Validation(customUrlKey + " contains an invalid URL: " + customUrl))
		}
		uris = append(uris, u)
	}
	return uris
}

func validateUriScheme(uri *url.URL) {
	if uri.Scheme == "" {
		panic(errs.Validation("Custom service URL must have a scheme: " + uri.String()))
	}
	scheme := strings.ToLower(uri.Scheme)
	if scheme != k8s.McpBridgeProtocolHttp && scheme != k8s.McpBridgeProtocolHttps {
		panic(errs.Validation("Custom service URL must have a valid scheme: " + uri.String()))
	}
}

func toBooleanDefault(v any, def bool) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	if s, ok := v.(string); ok {
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "true", "1", "yes":
			return true
		case "false", "0", "no", "":
			return false
		}
	}
	if n, ok := v.(float64); ok {
		return int(n) != 0
	}
	if n, ok := v.(json.Number); ok {
		if i, err := n.Int64(); err == nil {
			return i != 0
		}
	}
	return def
}

func toBooleanPtr(v any) *bool {
	switch t := v.(type) {
	case bool:
		return &t
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "true", "1", "yes":
			b := true
			return &b
		case "false", "0", "no", "":
			b := false
			return &b
		}
	case float64:
		b := int(t) != 0
		return &b
	case json.Number:
		if i, err := t.Int64(); err == nil {
			b := i != 0
			return &b
		}
	}
	return nil
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
