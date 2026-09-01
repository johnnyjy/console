package sdk

import (
	"encoding/json"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"

	"console/internal/consts"
	"console/internal/errs"
	k8s "console/internal/kubernetes"
	"console/internal/model"
)

// llmProviderHandler 对应 Java 的 LlmProviderHandler 及各具体实现
type llmProviderHandler struct {
	typ                  string
	needSync             bool
	endpoints            func(providerConfig map[string]any) []*model.LlmProviderEndpoint
	normalize            func(configurations map[string]any)
	loadConfig           func(provider *model.LlmProvider, configurations map[string]any) bool
	saveConfig           func(provider *model.LlmProvider, configurations map[string]any)
	buildServiceSource   func(providerName string, providerConfig map[string]any) *model.ServiceSource
	buildUpstreamService func(providerName string, providerConfig map[string]any) *model.UpstreamService
	extraServiceSources  func(providerName string, providerConfig map[string]any, forDelete bool) []*model.ServiceSource
}

func (h *llmProviderHandler) getType() string { return h.typ }

func (h *llmProviderHandler) getServiceSourceName(providerName string) string {
	return consts.LlmServiceNamePrefix + providerName + consts.InternalResourceNameSuffix
}

func (h *llmProviderHandler) doLoadConfig(provider *model.LlmProvider, configurations map[string]any) bool {
	if h.loadConfig != nil {
		return h.loadConfig(provider, configurations)
	}
	return h.defaultLoadConfig(provider, configurations)
}

func (h *llmProviderHandler) doSaveConfig(provider *model.LlmProvider, configurations map[string]any) {
	if h.saveConfig != nil {
		h.saveConfig(provider, configurations)
		return
	}
	h.defaultSaveConfig(provider, configurations)
}

func (h *llmProviderHandler) doNormalizeConfigs(configurations map[string]any) {
	if h.normalize != nil {
		h.normalize(configurations)
	}
}

func (h *llmProviderHandler) doBuildServiceSource(providerName string, providerConfig map[string]any) *model.ServiceSource {
	if h.buildServiceSource != nil {
		return h.buildServiceSource(providerName, providerConfig)
	}
	return h.defaultBuildServiceSource(providerName, providerConfig)
}

func (h *llmProviderHandler) doGetExtraServiceSources(providerName string, providerConfig map[string]any,
	forDelete bool) []*model.ServiceSource {
	if h.extraServiceSources != nil {
		return h.extraServiceSources(providerName, providerConfig, forDelete)
	}
	return nil
}

func (h *llmProviderHandler) doBuildUpstreamService(providerName string,
	providerConfig map[string]any) *model.UpstreamService {
	if h.buildUpstreamService != nil {
		return h.buildUpstreamService(providerName, providerConfig)
	}
	return h.defaultBuildUpstreamService(providerName, providerConfig)
}

func (h *llmProviderHandler) doNeedSyncRouteAfterUpdate() bool { return h.needSync }

// ---- 默认实现（对应 AbstractLlmProviderHandler） ----

func (h *llmProviderHandler) defaultLoadConfig(provider *model.LlmProvider, configurations map[string]any) bool {
	id := anyStrOr(configurations[consts.AiProxyProviderId], "")
	if strings.TrimSpace(id) == "" {
		return false
	}

	var tokens []string
	if tokensObj, ok := configurations[consts.AiProxyProviderApiTokens].([]any); ok {
		for _, tokenObj := range tokensObj {
			if token, ok := tokenObj.(string); ok {
				tokens = append(tokens, token)
			}
		}
	}

	var failoverConfig *model.TokenFailoverConfig
	if failoverObj, ok := configurations[consts.AiProxyFailover].(map[string]any); ok {
		failoverConfig = buildTokenFailoverConfig(failoverObj)
	}

	protocol := model.LlmProviderProtocolFromPluginValue(anyStrOr(configurations[consts.AiProxyProtocol], ""))
	if protocol == "" {
		protocol = model.LlmProviderProtocolDefault
	}

	provider.Protocol = strPtr(protocol)
	provider.Name = strPtr(id)
	provider.Type = strPtr(h.getType())
	provider.Tokens = tokens
	provider.TokenFailoverConfig = failoverConfig
	provider.RawConfigs = cloneAnyMap(configurations)
	return true
}

func (h *llmProviderHandler) defaultSaveConfig(provider *model.LlmProvider, configurations map[string]any) {
	configurations[consts.AiProxyProviderId] = deref(provider.Name)
	configurations[consts.AiProxyProviderType] = h.getType()

	protocol := model.LlmProviderProtocolFromValue(deref(provider.Protocol))
	if protocol == "" {
		protocol = model.LlmProviderProtocolDefault
	}
	configurations[consts.AiProxyProtocol] = model.LlmProviderProtocolPluginValue(protocol)

	if len(provider.Tokens) > 0 {
		configurations[consts.AiProxyProviderApiTokens] = anyStringList(provider.Tokens)
	} else {
		delete(configurations, consts.AiProxyProviderApiTokens)
	}

	if provider.TokenFailoverConfig == nil {
		delete(configurations, consts.AiProxyFailover)
		delete(configurations, consts.AiProxyRetryOnFailure)
	} else {
		failoverMap := map[string]any{}
		saveTokenFailoverConfig(provider.TokenFailoverConfig, failoverMap)
		configurations[consts.AiProxyFailover] = failoverMap
		configurations[consts.AiProxyRetryOnFailure] = map[string]any{
			consts.AiProxyRetryEnabled: provider.TokenFailoverConfig.Enabled,
		}
	}
}

func (h *llmProviderHandler) defaultBuildServiceSource(providerName string,
	providerConfig map[string]any) *model.ServiceSource {
	serviceSource := &model.ServiceSource{}
	serviceSource.Name = strPtr(h.getServiceSourceName(providerName))
	endpoints := h.endpoints(providerConfig)
	if len(endpoints) == 0 {
		panic(errs.Validation("No endpoints found for provider: " + providerName))
	}
	var typ, protocol, contextPath string
	domains := make([]string, 0, len(endpoints))
	var port *int
	for _, endpoint := range endpoints {
		if endpoint == nil {
			continue
		}
		validateEndpoint(endpoint)

		epProtocol := deref(endpoint.Protocol)
		if protocol != "" && protocol != epProtocol {
			panic(errs.Validation("Multiple protocols found in the endpoints of provider: " + providerName))
		}
		protocol = epProtocol

		epContextPath := deref(endpoint.ContextPath)
		if contextPath != "" && contextPath != epContextPath {
			panic(errs.Validation("Multiple context paths found in the endpoints of provider: " + providerName))
		}
		contextPath = epContextPath

		var endpointSourceType string
		if checkIpAddress(deref(endpoint.Address)) {
			endpointSourceType = k8s.McpBridgeRegistryTypeStatic
			domains = append(domains, deref(endpoint.Address)+consts.SeparatorColon+strconv.Itoa(derefInt(endpoint.Port)))
			port = intPtr(k8s.McpBridgeStaticPort)
		} else {
			if len(endpoints) > 1 {
				panic(errs.Validation("Multiple endpoints only work with static IP addresses, provider: " + providerName))
			}
			port = endpoint.Port
			endpointSourceType = k8s.McpBridgeRegistryTypeDNS
			domains = append(domains, deref(endpoint.Address))
		}
		if typ != "" && typ != endpointSourceType {
			panic(errs.Validation("Multiple types of endpoints found for provider: " + providerName))
		}
		typ = endpointSourceType
	}
	serviceSource.Type = strPtr(typ)
	serviceSource.Protocol = strPtr(protocol)
	serviceSource.Domain = strPtr(strings.Join(domains, consts.SeparatorComma))
	serviceSource.Port = port
	return serviceSource
}

func (h *llmProviderHandler) defaultBuildUpstreamService(providerName string,
	providerConfig map[string]any) *model.UpstreamService {
	serviceSource := h.doBuildServiceSource(providerName, providerConfig)
	service := &model.UpstreamService{}
	service.Name = strPtr(deref(serviceSource.Name) + consts.SeparatorDot + deref(serviceSource.Type))
	service.Port = serviceSource.Port
	service.Weight = intPtr(100)
	return service
}

// ---- provider handlers 注册表 ----

var (
	providerHandlersOnce sync.Once
	providerHandlersMap  map[string]*llmProviderHandler
)

// getProviderHandlers 延迟初始化并返回 provider handlers 注册表，避免初始化循环
func getProviderHandlers() map[string]*llmProviderHandler {
	providerHandlersOnce.Do(func() {
		providerHandlersMap = buildProviderHandlers()
	})
	return providerHandlersMap
}

func buildProviderHandlers() map[string]*llmProviderHandler {
	handlers := []*llmProviderHandler{
		newOpenaiHandler(),
		newDefaultHandler(model.LlmProviderTypeMoonshot, "api.moonshot.cn", 443, k8s.McpBridgeProtocolHttps),
		newQwenHandler(),
		newAzureHandler(),
		newDefaultHandler(model.LlmProviderTypeAi360, "api.360.cn", 443, k8s.McpBridgeProtocolHttps),
		newDefaultHandler(model.LlmProviderTypeGithub, "models.inference.ai.azure.com", 443, k8s.McpBridgeProtocolHttps),
		newDefaultHandler(model.LlmProviderTypeGroq, "api.groq.com", 443, k8s.McpBridgeProtocolHttps),
		newDefaultHandler(model.LlmProviderTypeBaichuan, "api.baichuan-ai.com", 443, k8s.McpBridgeProtocolHttps),
		newDefaultHandler(model.LlmProviderTypeYi, "api.lingyiwanwu.com", 443, k8s.McpBridgeProtocolHttps),
		newDefaultHandler(model.LlmProviderTypeDeepseek, "api.deepseek.com", 443, k8s.McpBridgeProtocolHttps),
		newZhipuaiHandler(),
		newOllamaHandler(),
		newClaudeHandler(),
		newDefaultHandler(model.LlmProviderTypeBaidu, "qianfan.baidubce.com", 443, k8s.McpBridgeProtocolHttps),
		newDefaultHandler(model.LlmProviderTypeStepfun, "api.stepfun.com", 443, k8s.McpBridgeProtocolHttps),
		newDefaultHandler(model.LlmProviderTypeMinimax, "api.minimax.chat", 443, k8s.McpBridgeProtocolHttps),
		newDefaultHandler(model.LlmProviderTypeGemini, "generativelanguage.googleapis.com", 443, k8s.McpBridgeProtocolHttps),
		newDefaultHandler(model.LlmProviderTypeMistral, "api.mistral.ai", 443, k8s.McpBridgeProtocolHttps),
		newDefaultHandler(model.LlmProviderTypeCohere, "api.cohere.com", 443, k8s.McpBridgeProtocolHttps),
		newDefaultHandler(model.LlmProviderTypeDoubao, "ark.cn-beijing.volces.com", 443, k8s.McpBridgeProtocolHttps),
		newDefaultHandler(model.LlmProviderTypeCoze, "api.coze.cn", 443, k8s.McpBridgeProtocolHttps),
		newDefaultHandler(model.LlmProviderTypeOpenrouter, "openrouter.ai", 443, k8s.McpBridgeProtocolHttps),
		newDefaultHandler(model.LlmProviderTypeGrok, "api.x.ai", 443, k8s.McpBridgeProtocolHttps),
		newBedrockHandler(),
		newVertexHandler(),
		newVllmHandler(),
	}
	m := make(map[string]*llmProviderHandler, len(handlers))
	for _, h := range handlers {
		m[h.getType()] = h
	}
	return m
}

func newDefaultHandler(typ, domain string, port int, protocol string) *llmProviderHandler {
	return &llmProviderHandler{
		typ: typ,
		endpoints: func(providerConfig map[string]any) []*model.LlmProviderEndpoint {
			return []*model.LlmProviderEndpoint{{
				Protocol:    strPtr(protocol),
				Address:     strPtr(domain),
				Port:        intPtr(port),
				ContextPath: strPtr("/"),
			}}
		},
	}
}

// ---- LlmProviderService ----

// LlmProviderService 对应 Java 的 LlmProviderServiceImpl
type LlmProviderService struct {
	serviceSourceService      *ServiceSourceService
	wasmPluginInstanceService *WasmPluginInstanceService
	aiRouteService            *AiRouteService
}

// NewLlmProviderService 创建 LlmProviderService
func NewLlmProviderService(serviceSourceService *ServiceSourceService,
	wasmPluginInstanceService *WasmPluginInstanceService) *LlmProviderService {
	return &LlmProviderService{
		serviceSourceService:      serviceSourceService,
		wasmPluginInstanceService: wasmPluginInstanceService,
	}
}

// SetAiRouteService 设置 AiRouteService（解决循环依赖）
func (s *LlmProviderService) SetAiRouteService(aiRouteService *AiRouteService) {
	s.aiRouteService = aiRouteService
}

// AddOrUpdate 对应 addOrUpdate
func (s *LlmProviderService) AddOrUpdate(provider *model.LlmProvider) *model.LlmProvider {
	handler := getProviderHandlers()[deref(provider.Type)]
	if handler == nil {
		panic(errs.Validation("Provider type " + deref(provider.Type) + " is not supported"))
	}

	if provider.RawConfigs == nil {
		provider.RawConfigs = map[string]any{}
	}

	handler.doNormalizeConfigs(provider.RawConfigs)

	fillDefaultValues(provider)

	instances := s.wasmPluginInstanceService.List(consts.PluginAiProxy, boolPtr(true))

	var instancesToUpdate, instancesToDelete []*model.WasmPluginInstance
	pluginName := consts.PluginAiProxy

	var globalInstance *model.WasmPluginInstance
	for _, i := range instances {
		if i.HasScopedTarget(model.ScopeGlobal) {
			globalInstance = i
			break
		}
	}
	if globalInstance == nil {
		globalInstance = s.wasmPluginInstanceService.CreateEmptyInstance(pluginName)
		globalInstance.Internal = boolPtr(true)
		globalInstance.SetGlobalTarget()
	}
	globalInstance.Enabled = boolPtr(true)

	configurations := globalInstance.Configurations
	if len(configurations) == 0 {
		configurations = map[string]any{}
		globalInstance.Configurations = configurations
	}

	providersObj, ok := configurations[consts.AiProxyProviders].([]any)
	if !ok {
		providersObj = []any{}
		configurations[consts.AiProxyProviders] = providersObj
	}

	providerConfig := cloneAnyMap(provider.RawConfigs)
	handler.doSaveConfig(provider, providerConfig)

	found := false
	providers := providersObj
	for i := range providers {
		providerObj, ok := providers[i].(map[string]any)
		if !ok {
			continue
		}
		if deref(provider.Name) == anyStrOr(providerObj[consts.AiProxyProviderId], "") {
			providers[i] = providerConfig
			found = true
			break
		}
	}
	if !found {
		providers = append(providers, providerConfig)
	}
	configurations[consts.AiProxyProviders] = providers

	instancesToUpdate = append(instancesToUpdate, globalInstance)

	var serviceSources []*model.ServiceSource
	if source := handler.doBuildServiceSource(deref(provider.Name), providerConfig); source != nil {
		serviceSources = append(serviceSources, source)
	}
	if extra := handler.doGetExtraServiceSources(deref(provider.Name), providerConfig, false); len(extra) > 0 {
		serviceSources = append(serviceSources, extra...)
	}

	upstreamService := handler.doBuildUpstreamService(deref(provider.Name), providerConfig)
	upstreamServiceName := deref(upstreamService.Name)

	var existedServiceInstance *model.WasmPluginInstance
	for _, i := range instances {
		if i.HasScopedTargetWithTarget(model.ScopeService, upstreamServiceName) {
			existedServiceInstance = i
			break
		}
	}
	if existedServiceInstance != nil {
		boundProviderName := anyStrOr(existedServiceInstance.Configurations[consts.AiProxyActiveProviderId], "")
		if deref(provider.Name) != boundProviderName {
			panic(errs.Validation("The service instance for provider " + boundProviderName +
				" is already existed. Cannot bind it to provider " + deref(provider.Name)))
		}
	}

	needNewServiceInstance := true
	for _, instance := range instances {
		boundProviderId := anyStrOr(instance.Configurations[consts.AiProxyActiveProviderId], "")
		if deref(provider.Name) != boundProviderId {
			continue
		}
		service := ""
		if v := instance.Targets[model.ScopeService]; v != nil {
			service = *v
		}
		if upstreamServiceName == service {
			needNewServiceInstance = false
		} else {
			instancesToDelete = append(instancesToDelete, instance)
		}
	}
	if needNewServiceInstance {
		serviceInstance := &model.WasmPluginInstance{}
		serviceInstance.PluginName = globalInstance.PluginName
		serviceInstance.PluginVersion = globalInstance.PluginVersion
		serviceInstance.SetTarget(model.ScopeService, strPtr(upstreamServiceName))
		serviceInstance.Enabled = boolPtr(true)
		serviceInstance.Internal = boolPtr(true)
		serviceInstance.Configurations = map[string]any{consts.AiProxyActiveProviderId: deref(provider.Name)}
		instancesToUpdate = append(instancesToUpdate, serviceInstance)
	}

	if len(serviceSources) > 0 {
		for _, source := range serviceSources {
			source.ProxyName = provider.ProxyName
			s.serviceSourceService.AddOrUpdate(source)
		}
	}

	s.wasmPluginInstanceService.AddOrUpdateAll(instancesToUpdate)

	if handler.doNeedSyncRouteAfterUpdate() {
		s.syncRelatedAiRoutes(provider)
	}

	if len(instancesToDelete) > 0 {
		for _, instance := range instancesToDelete {
			s.wasmPluginInstanceService.DeleteByTargets(instance.Targets, deref(instance.PluginName),
				instance.Internal)
		}
	}

	return s.Query(deref(provider.Name))
}

// List 对应 list
func (s *LlmProviderService) List(query *model.CommonPageQuery) *model.PaginatedResult[model.LlmProvider] {
	return model.CreateFromFullList(s.getProviders(), query)
}

// Query 对应 query
func (s *LlmProviderService) Query(providerName string) *model.LlmProvider {
	providers := s.getProviders()
	for i := range providers {
		if deref(providers[i].Name) == providerName {
			return &providers[i]
		}
	}
	return nil
}

// Delete 对应 delete
func (s *LlmProviderService) Delete(providerName string) {
	instances := s.wasmPluginInstanceService.List(consts.PluginAiProxy, boolPtr(true))
	if len(instances) == 0 {
		return
	}

	var globalInstance *model.WasmPluginInstance
	for _, i := range instances {
		if i.HasScopedTarget(model.ScopeGlobal) {
			globalInstance = i
			break
		}
	}
	if globalInstance == nil {
		return
	}

	globalConfigurations := globalInstance.Configurations
	if len(globalConfigurations) == 0 {
		return
	}

	providersObj, ok := globalConfigurations[consts.AiProxyProviders].([]any)
	if !ok {
		return
	}

	var deletedProvider map[string]any
	providers := providersObj
	for i := len(providers) - 1; i >= 0; i-- {
		providerObj, ok := providers[i].(map[string]any)
		if !ok {
			continue
		}
		if providerName == anyStrOr(providerObj[consts.AiProxyProviderId], "") {
			providers = append(providers[:i], providers[i+1:]...)
			deletedProvider = providerObj
			break
		}
	}

	if deletedProvider == nil {
		return
	}

	if len(providers) == 0 {
		globalInstance.Enabled = boolPtr(false)
	}
	globalConfigurations[consts.AiProxyProviders] = providers

	if typ := anyStrOr(deletedProvider[consts.AiProxyProviderType], ""); typ != "" {
		if handler := getProviderHandlers()[typ]; handler != nil {
			upstreamService := handler.doBuildUpstreamService(providerName, deletedProvider)
			s.wasmPluginInstanceService.Delete(model.ScopeService, deref(upstreamService.Name),
				consts.PluginAiProxy, boolPtr(true))

			if source := handler.doBuildServiceSource(providerName, deletedProvider); source != nil {
				s.serviceSourceService.Delete(deref(source.Name))
			}

			if extra := handler.doGetExtraServiceSources(providerName, deletedProvider, true); len(extra) > 0 {
				for _, extraSource := range extra {
					s.serviceSourceService.Delete(deref(extraSource.Name))
				}
			}
		}
	}

	s.wasmPluginInstanceService.AddOrUpdate(globalInstance)
}

// BuildUpstreamService 对应 buildUpstreamService
func (s *LlmProviderService) BuildUpstreamService(providerName string) *model.UpstreamService {
	provider := s.Query(providerName)
	if provider == nil {
		panic(errs.Validation("Unknown provider: " + providerName))
	}
	handler := getProviderHandlers()[deref(provider.Type)]
	if handler == nil {
		panic(errs.Validation("Provider type " + deref(provider.Type) + " of provider " + providerName +
			" is not supported"))
	}
	return handler.doBuildUpstreamService(deref(provider.Name), provider.RawConfigs)
}

func (s *LlmProviderService) syncRelatedAiRoutes(provider *model.LlmProvider) {
	if s.aiRouteService == nil {
		panic(errs.Internal("AiRouteService is not available when AI route syncing is needed."))
	}
	aiRoutes := s.aiRouteService.List(nil)
	if aiRoutes == nil || len(aiRoutes.Data) == 0 {
		return
	}
	providerName := deref(provider.Name)
	for i := range aiRoutes.Data {
		aiRoute := &aiRoutes.Data[i]
		if len(aiRoute.Upstreams) == 0 {
			continue
		}
		if hasProvider(aiRoute.Upstreams, providerName) ||
			(aiRoute.FallbackConfig != nil && boolValue(aiRoute.FallbackConfig.Enabled) &&
				hasProvider(aiRoute.FallbackConfig.Upstreams, providerName)) {
			s.aiRouteService.Update(aiRoute)
		}
	}
}

func hasProvider(upstreams []model.AiUpstream, providerName string) bool {
	for i := range upstreams {
		if deref(upstreams[i].Provider) == providerName {
			return true
		}
	}
	return false
}

func (s *LlmProviderService) getProviders() []model.LlmProvider {
	instance := s.wasmPluginInstanceService.Query(model.ScopeGlobal, "", consts.PluginAiProxy, boolPtr(true))
	if instance == nil || len(instance.Configurations) == 0 {
		return []model.LlmProvider{}
	}
	providersObj, ok := instance.Configurations[consts.AiProxyProviders].([]any)
	if !ok {
		return []model.LlmProvider{}
	}
	var providers []model.LlmProvider
	for _, providerObj := range providersObj {
		providerMap, ok := providerObj.(map[string]any)
		if !ok {
			continue
		}
		provider := extractProvider(providerMap)
		if provider == nil {
			continue
		}
		providers = append(providers, *provider)
	}
	sort.Slice(providers, func(i, j int) bool {
		return deref(providers[i].Name) < deref(providers[j].Name)
	})
	s.fillProxyInfo(providers)
	return providers
}

func extractProvider(configurations map[string]any) *model.LlmProvider {
	typ := anyStrOr(configurations[consts.AiProxyProviderType], "")
	if strings.TrimSpace(typ) == "" {
		return nil
	}
	handler := getProviderHandlers()[typ]
	if handler == nil {
		return nil
	}
	provider := &model.LlmProvider{}
	if !handler.doLoadConfig(provider, configurations) {
		return nil
	}
	return provider
}

func (s *LlmProviderService) fillProxyInfo(providers []model.LlmProvider) {
	if len(providers) == 0 {
		return
	}
	serviceSources := s.serviceSourceService.List(nil)
	if serviceSources == nil || len(serviceSources.Data) == 0 {
		return
	}
	serviceSourceMap := map[string]*model.ServiceSource{}
	for i := range serviceSources.Data {
		serviceSourceMap[deref(serviceSources.Data[i].Name)] = &serviceSources.Data[i]
	}
	for i := range providers {
		provider := &providers[i]
		handler := getProviderHandlers()[deref(provider.Type)]
		if handler == nil {
			continue
		}
		serviceSourceName := handler.getServiceSourceName(deref(provider.Name))
		if source := serviceSourceMap[serviceSourceName]; source != nil {
			provider.ProxyName = source.ProxyName
		}
	}
}

func fillDefaultValues(provider *model.LlmProvider) {
	if strings.TrimSpace(deref(provider.Protocol)) == "" {
		provider.Protocol = strPtr(model.LlmProviderProtocolDefault)
	}
}

// ---- 辅助函数 ----

func buildTokenFailoverConfig(failoverMap map[string]any) *model.TokenFailoverConfig {
	if len(failoverMap) == 0 {
		return nil
	}
	cfg := &model.TokenFailoverConfig{}
	cfg.Enabled = boolPtr(anyBoolOr(failoverMap[consts.AiProxyFailoverEnabled], false))
	cfg.FailureThreshold = anyIntOrNil(failoverMap[consts.AiProxyFailoverFailureThr])
	cfg.SuccessThreshold = anyIntOrNil(failoverMap[consts.AiProxyFailoverSuccessThr])
	cfg.HealthCheckInterval = anyIntOrNil(failoverMap[consts.AiProxyFailoverHealthCheckInterval])
	cfg.HealthCheckTimeout = anyIntOrNil(failoverMap[consts.AiProxyFailoverHealthCheckTimeout])
	cfg.HealthCheckModel = anyStrPtrOrNil(failoverMap[consts.AiProxyFailoverHealthCheckModel])
	return cfg
}

func saveTokenFailoverConfig(cfg *model.TokenFailoverConfig, failoverMap map[string]any) {
	failoverMap[consts.AiProxyFailoverEnabled] = boolPtrOrNil(cfg.Enabled)
	failoverMap[consts.AiProxyFailoverFailureThr] = intPtrOrNil(cfg.FailureThreshold)
	failoverMap[consts.AiProxyFailoverSuccessThr] = intPtrOrNil(cfg.SuccessThreshold)
	failoverMap[consts.AiProxyFailoverHealthCheckInterval] = intPtrOrNil(cfg.HealthCheckInterval)
	failoverMap[consts.AiProxyFailoverHealthCheckTimeout] = intPtrOrNil(cfg.HealthCheckTimeout)
	failoverMap[consts.AiProxyFailoverHealthCheckModel] = strPtrOrNil(cfg.HealthCheckModel)
}

func anyStrPtrOrNil(v any) *string {
	if s, ok := v.(string); ok && s != "" {
		return &s
	}
	return nil
}

func strPtrOrNil(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

func boolPtrOrNil(v *bool) any {
	if v == nil {
		return nil
	}
	return *v
}

func intPtrOrNil(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func derefInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func cloneAnyMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func validateEndpoint(endpoint *model.LlmProviderEndpoint) {
	if strings.TrimSpace(deref(endpoint.Protocol)) == "" {
		panic(errs.Business("Protocol cannot be null or empty."))
	}
	if strings.TrimSpace(deref(endpoint.Address)) == "" {
		panic(errs.Business("Address cannot be null or empty."))
	}
	if !checkPort(derefInt(endpoint.Port)) {
		panic(errs.Business("Port must be a positive integer."))
	}
}

func checkPort(port int) bool {
	return port >= 1 && port <= 65535
}

func checkIpAddress(s string) bool {
	return net.ParseIP(s) != nil
}

func getIntConfig(providerConfig map[string]any, key string) int {
	v, ok := providerConfig[key]
	if !ok {
		panic(errs.Validation(key + " must be a number."))
	}
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return int(i)
		}
	case string:
		if i, err := strconv.Atoi(t); err == nil {
			return i
		}
		panic(errs.Validation(key + " must be a number."))
	}
	panic(errs.Validation(key + " must be a number."))
}

// endpointFromUri 对应 LlmProviderEndpoint.fromUri
func endpointFromUri(u *url.URL) *model.LlmProviderEndpoint {
	if u == nil || !u.IsAbs() {
		panic(errs.Business("URI must be absolute."))
	}
	protocol := u.Scheme
	if protocol == "" {
		protocol = k8s.McpBridgeProtocolHttp
	}
	switch strings.ToLower(protocol) {
	case k8s.McpBridgeProtocolHttp, k8s.McpBridgeProtocolHttps:
	default:
		protocol = k8s.McpBridgeProtocolHttp
	}
	port := 80
	if protocol == k8s.McpBridgeProtocolHttps {
		port = 443
	}
	if u.Port() != "" {
		if p, err := strconv.Atoi(u.Port()); err == nil {
			port = p
		}
	}
	contextPath := u.Path
	if contextPath == "" {
		contextPath = "/"
	}
	return &model.LlmProviderEndpoint{
		Protocol:    strPtr(protocol),
		Address:     strPtr(u.Hostname()),
		Port:        intPtr(port),
		ContextPath: strPtr(contextPath),
	}
}
