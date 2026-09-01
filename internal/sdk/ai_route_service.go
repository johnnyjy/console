package sdk

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"console/internal/consts"
	"console/internal/errs"
	k8s "console/internal/kubernetes"
	"console/internal/model"
)

const (
	aiRouteAutoEnableAiStatsEnvKey       = "AI_ROUTE_AUTO_ENABLE_AI_STATS"
	aiRouteAutoEnableModelRouterEnvKey   = "AI_ROUTE_AUTO_ENABLE_MODEL_ROUTER"
	routeFallbackEnvoyFilterTemplateBase = `apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: "${name}"
spec:
  configPatches:
    - applyTo: HTTP_ROUTE
      match:
        context: GATEWAY
        routeConfiguration:
          vhost:
            route:
              name: "${routeName}"
      patch:
        operation: MERGE
        value:
          typed_per_filter_config:
            envoy.filters.http.custom_response:
              "@type": type.googleapis.com/udpa.type.v1.TypedStruct
              type_url: type.googleapis.com/envoy.extensions.filters.http.custom_response.v3.CustomResponse
              value:
                custom_response_matcher:
                  matcher_list:
                    matchers:
                      - predicate:
%s
                        on_match:
                          action:
                            name: action
                            typed_config:
                              "@type": type.googleapis.com/udpa.type.v1.TypedStruct
                              type_url: type.googleapis.com/envoy.extensions.http.custom_response.redirect_policy.v3.RedirectPolicy
                              value:
                                max_internal_redirects: 10
                                use_original_request_uri: true
                                keep_original_response_code: false
                                use_original_request_body: true
                                only_redirect_upstream_code: false
                                request_headers_to_add:
                                  - header:
                                      key: "${fallbackHeader}"
                                      value: "${routeName}"
                                    append: false
                                response_headers_to_add:
                                  - header:
                                      key: "${fallbackHeader}"
                                      value: "${routeName}"
                                    append: false
                with_request_body:
                  max_request_bytes: 1024000
`
)

var modelRoutingHeaderRegexEscape = regexp.MustCompile(`[\[\]{}()^$|*+?.\\]`)

func defaultProxyNextUpstreamConfig() *model.ProxyNextUpstreamConfig {
	return &model.ProxyNextUpstreamConfig{
		Enabled:    boolPtr(true),
		Attempts:   intPtr(3),
		Timeout:    intPtr(120),
		Conditions: []string{"error", "timeout", "non_idempotent"},
	}
}

func defaultPathPredicate() *model.RoutePredicate {
	return &model.RoutePredicate{MatchType: strPtr("PRE"), MatchValue: strPtr("/"), CaseSensitive: boolPtr(true)}
}

// AiRouteService 对应 Java 的 AiRouteServiceImpl
type AiRouteService struct {
	converter                 *Converter
	client                    *k8s.Client
	routeService              *RouteService
	llmProviderService        *LlmProviderService
	wasmPluginInstanceService *WasmPluginInstanceService
}

// NewAiRouteService 创建 AiRouteService
func NewAiRouteService(converter *Converter, client *k8s.Client, routeService *RouteService,
	llmProviderService *LlmProviderService, wasmPluginInstanceService *WasmPluginInstanceService) *AiRouteService {
	return &AiRouteService{
		converter:                 converter,
		client:                    client,
		routeService:              routeService,
		llmProviderService:        llmProviderService,
		wasmPluginInstanceService: wasmPluginInstanceService,
	}
}

// Add 对应 add
func (s *AiRouteService) Add(route *model.AiRoute) *model.AiRoute {
	s.fillDefaultValues(route)

	s.writeAiRouteResources(route)
	s.writeAiRouteFallbackResources(route)

	configMap := s.converter.AiRoute2ConfigMap(route)
	newConfigMap, err := s.client.CreateConfigMap(context.Background(), configMap)
	if err != nil {
		if apierrors.IsConflict(err) {
			panic(errs.Conflict(""))
		}
		panic(errs.Business("Error occurs when adding a new AI route."))
	}

	return s.configMap2AiRoute(newConfigMap)
}

// List 对应 list
func (s *AiRouteService) List(query *model.CommonPageQuery) *model.PaginatedResult[model.AiRoute] {
	configMaps, err := s.client.ListConfigMap(context.Background(), map[string]string{
		consts.LabelConfigMapTypeKey: consts.LabelConfigMapTypeAiRoute,
	})
	if err != nil {
		panic(errs.Business("Error occurs when listing ConfigMap."))
	}
	routes := make([]model.AiRoute, 0, len(configMaps))
	for i := range configMaps {
		if r := s.configMap2AiRoute(&configMaps[i]); r != nil {
			routes = append(routes, *r)
		}
	}
	return model.CreateFromFullList(routes, query)
}

// Query 对应 query
func (s *AiRouteService) Query(routeName string) *model.AiRoute {
	configMapName := s.converter.AiRouteName2ConfigMapName(routeName)
	configMap, err := s.client.ReadConfigMap(context.Background(), configMapName)
	if err != nil {
		panic(errs.Business("Error occurs when reading the ConfigMap with name: " + configMapName))
	}
	if configMap == nil {
		return nil
	}
	return s.configMap2AiRoute(configMap)
}

// Delete 对应 delete
func (s *AiRouteService) Delete(routeName string) {
	s.deleteAiRouteResources(routeName)

	configMapName := s.converter.AiRouteName2ConfigMapName(routeName)
	if err := s.client.DeleteConfigMap(context.Background(), configMapName); err != nil {
		panic(errs.Business("Error occurs when deleting the ConfigMap with name: " + configMapName))
	}
}

// Update 对应 update
func (s *AiRouteService) Update(route *model.AiRoute) *model.AiRoute {
	s.fillDefaultValues(route)

	s.writeAiRouteResources(route)
	s.writeAiRouteFallbackResources(route)

	configMap := s.converter.AiRoute2ConfigMap(route)
	updatedConfigMap, err := s.client.ReplaceConfigMap(context.Background(), configMap)
	if err != nil {
		if apierrors.IsConflict(err) {
			panic(errs.Conflict(""))
		}
		panic(errs.Business("Error occurs when replacing the ConfigMap generated by AI route: " + deref(route.Name)))
	}

	return s.configMap2AiRoute(updatedConfigMap)
}

func (s *AiRouteService) configMap2AiRoute(configMap *corev1.ConfigMap) *model.AiRoute {
	route := s.converter.ConfigMap2AiRoute(configMap)
	if route != nil {
		s.fillDefaultValues(route)
	}
	return route
}

func (s *AiRouteService) fillDefaultValues(route *model.AiRoute) {
	if route.PathPredicate == nil {
		route.PathPredicate = defaultPathPredicate()
	}
	fillDefaultWeights(route.Upstreams)
	if route.FallbackConfig != nil && boolValue(route.FallbackConfig.Enabled) {
		fillDefaultWeights(route.FallbackConfig.Upstreams)
		if strings.TrimSpace(deref(route.FallbackConfig.FallbackStrategy)) == "" {
			route.FallbackConfig.FallbackStrategy = strPtr(model.AiRouteFallbackRandom)
		}
	}
	if route.ProxyNextUpstream == nil {
		route.ProxyNextUpstream = defaultProxyNextUpstreamConfig()
	}
}

func fillDefaultWeights(upstreams []model.AiUpstream) {
	if len(upstreams) != 1 {
		return
	}
	upstreams[0].Weight = intPtr(100)
}

func (s *AiRouteService) writeAiRouteResources(aiRoute *model.AiRoute) {
	routeName := buildRouteResourceName(deref(aiRoute.Name))
	route := s.buildRoute(routeName, aiRoute)
	s.setUpstreams(route, aiRoute.Upstreams)
	s.saveRoute(route)
	s.writeModelRouterResources(aiRoute.ModelPredicates)
	s.writeModelMappingResources(routeName, aiRoute.Upstreams)
	s.writeAiStatisticsResources(routeName)
}

func (s *AiRouteService) writeAiRouteFallbackResources(aiRoute *model.AiRoute) {
	fallbackConfig := aiRoute.FallbackConfig
	if fallbackConfig == nil || !boolValue(fallbackConfig.Enabled) || len(fallbackConfig.Upstreams) == 0 {
		s.deleteFallbackRouteResources(deref(aiRoute.Name))
		return
	}

	originalRouteName := buildRouteResourceName(deref(aiRoute.Name))
	fallbackRouteName := buildFallbackRouteResourceName(deref(aiRoute.Name))
	route := s.buildRoute(fallbackRouteName, aiRoute)

	fallbackHeaderPredicate := model.KeyedRoutePredicate{
		Key:           strPtr(consts.FallbackFromHeader),
		MatchType:     strPtr(string(model.RoutePredicateEqual)),
		MatchValue:    strPtr(originalRouteName),
		CaseSensitive: boolPtr(true),
	}
	headers := make([]model.KeyedRoutePredicate, 0, len(route.Headers)+1)
	headers = append(headers, route.Headers...)
	headers = append(headers, fallbackHeaderPredicate)
	route.Headers = headers

	fallbackStrategy := deref(fallbackConfig.FallbackStrategy)
	var fallbackUpstreams []model.AiUpstream
	if strings.TrimSpace(fallbackStrategy) == "" || fallbackStrategy == model.AiRouteFallbackRandom {
		fallbackUpstreams = fallbackConfig.Upstreams
		for i := range fallbackUpstreams {
			fallbackUpstreams[i].Weight = intPtr(1)
		}
	} else if fallbackStrategy == model.AiRouteFallbackSequence {
		fallbackUpstreams = fallbackConfig.Upstreams[:1]
	} else {
		panic(errs.Business("Unknown fallback strategy: " + fallbackStrategy))
	}
	s.setUpstreams(route, fallbackUpstreams)
	s.saveRoute(route)

	envoyFilterConfig := getRouteFallbackEnvoyFilterConfig(fallbackConfig.ResponseCodes)
	envoyFilterConfig = strings.ReplaceAll(envoyFilterConfig, "${name}", originalRouteName)
	envoyFilterConfig = strings.ReplaceAll(envoyFilterConfig, "${routeName}", originalRouteName)
	envoyFilterConfig = strings.ReplaceAll(envoyFilterConfig, "${fallbackHeader}", consts.FallbackFromHeader)
	envoyFilter := loadEnvoyFilterFromYaml(envoyFilterConfig)

	existedFilter, err := s.client.ReadEnvoyFilter(context.Background(), envoyFilter.GetName())
	if err != nil {
		panic(errs.Business("Error occurs when writing the fallback EnvoyFilter for AI route: " + deref(aiRoute.Name)))
	}
	if existedFilter == nil {
		if _, err := s.client.CreateEnvoyFilter(context.Background(), envoyFilter); err != nil {
			panic(errs.Business("Error occurs when writing the fallback EnvoyFilter for AI route: " +
				deref(aiRoute.Name)))
		}
	} else {
		envoyFilter.SetResourceVersion(existedFilter.GetResourceVersion())
		if _, err := s.client.ReplaceEnvoyFilter(context.Background(), envoyFilter); err != nil {
			panic(errs.Business("Error occurs when writing the fallback EnvoyFilter for AI route: " +
				deref(aiRoute.Name)))
		}
	}

	s.writeModelMappingResources(fallbackRouteName, fallbackUpstreams)
	s.writeAiStatisticsResources(fallbackRouteName)
}

func (s *AiRouteService) writeModelRouterResources(modelPredicates []model.AiModelPredicate) {
	if len(modelPredicates) == 0 {
		return
	}
	if !getBoolEnv(aiRouteAutoEnableModelRouterEnvKey, true) {
		return
	}

	pluginName := consts.PluginModelRouter
	instance := s.wasmPluginInstanceService.Query(model.ScopeGlobal, "", pluginName, boolPtr(true))
	if instance == nil {
		instance = s.wasmPluginInstanceService.CreateEmptyInstance(pluginName)
		instance.Internal = boolPtr(true)
		instance.SetGlobalTarget()
	}
	instance.Enabled = boolPtr(true)

	configurations := instance.Configurations
	if len(configurations) == 0 {
		configurations = map[string]any{}
		instance.Configurations = configurations
	}
	configurations[consts.ModelRouterModelToHeader] = consts.ModelRoutingHeader

	s.wasmPluginInstanceService.AddOrUpdate(instance)
}

func (s *AiRouteService) writeModelMappingResources(routeName string, upstreams []model.AiUpstream) {
	if len(upstreams) == 0 {
		s.wasmPluginInstanceService.Delete(model.ScopeRoute, routeName, consts.PluginModelMapper, boolPtr(true))
		return
	}

	pluginName := consts.PluginModelMapper
	for i := range upstreams {
		upstream := &upstreams[i]
		upstreamService := s.llmProviderService.BuildUpstreamService(deref(upstream.Provider))

		targets := map[model.WasmPluginInstanceScope]*string{
			model.ScopeRoute:   strPtr(routeName),
			model.ScopeService: upstreamService.Name,
		}

		if len(upstream.ModelMapping) == 0 {
			s.wasmPluginInstanceService.DeleteByTargets(targets, pluginName, boolPtr(true))
			continue
		}

		instance := s.wasmPluginInstanceService.QueryByTargets(targets, pluginName, boolPtr(true))
		if instance == nil {
			instance = s.wasmPluginInstanceService.CreateEmptyInstance(pluginName)
			instance.Internal = boolPtr(true)
			instance.Targets = targets
		}
		instance.Enabled = boolPtr(true)

		configurations := instance.Configurations
		if len(configurations) == 0 {
			configurations = map[string]any{}
			instance.Configurations = configurations
		}
		modelMapping := make(map[string]string, len(upstream.ModelMapping))
		for k, v := range upstream.ModelMapping {
			modelMapping[k] = v
		}
		configurations[consts.ModelMapperModelMapping] = modelMapping

		s.wasmPluginInstanceService.AddOrUpdate(instance)
	}
}

func (s *AiRouteService) writeAiStatisticsResources(routeName string) {
	if !getBoolEnv(aiRouteAutoEnableAiStatsEnvKey, true) {
		return
	}

	existedInstance := s.wasmPluginInstanceService.Query(model.ScopeRoute, routeName, consts.PluginAiStatistics,
		boolPtr(false))
	if existedInstance != nil {
		return
	}

	instance := s.wasmPluginInstanceService.CreateEmptyInstance(consts.PluginAiStatistics)
	instance.SetTarget(model.ScopeRoute, strPtr(routeName))
	instance.Enabled = boolPtr(true)
	instance.Internal = boolPtr(false)
	instance.Configurations = map[string]any{consts.AiStatisticsUseDefaultResponseAttrs: true}

	s.wasmPluginInstanceService.AddOrUpdate(instance)
}

func (s *AiRouteService) buildRoute(routeName string, aiRoute *model.AiRoute) *model.Route {
	route := &model.Route{}
	route.Name = strPtr(routeName)
	if aiRoute.PathPredicate != nil {
		route.Path = aiRoute.PathPredicate
	} else {
		route.Path = defaultPathPredicate()
	}
	route.Domains = aiRoute.Domains

	var headerPredicates []model.KeyedRoutePredicate
	if len(aiRoute.HeaderPredicates) > 0 {
		headerPredicates = append(headerPredicates, aiRoute.HeaderPredicates...)
	}
	if len(aiRoute.ModelPredicates) > 0 {
		headerRoutePredicate := model.KeyedRoutePredicate{Key: strPtr(consts.ModelRoutingHeader)}
		if len(aiRoute.ModelPredicates) == 1 {
			modelPredicate := aiRoute.ModelPredicates[0]
			headerRoutePredicate.MatchType = modelPredicate.MatchType
			headerRoutePredicate.MatchValue = modelPredicate.MatchValue
		} else {
			headerRoutePredicate.MatchType = strPtr(string(model.RoutePredicateRegular))
			headerRoutePredicate.MatchValue = strPtr(s.buildModelRoutingHeaderRegex(aiRoute.ModelPredicates))
		}
		headerPredicates = append(headerPredicates, headerRoutePredicate)
	}
	route.Headers = headerPredicates
	route.UrlParams = aiRoute.UrlParamPredicates

	route.AuthConfig = aiRoute.AuthConfig
	route.Cors = aiRoute.Cors
	route.HeaderControl = aiRoute.HeaderControl
	route.ProxyNextUpstream = aiRoute.ProxyNextUpstream
	route.CustomConfigs = aiRoute.CustomConfigs
	route.CustomLabels = aiRoute.CustomLabels

	return route
}

func (s *AiRouteService) buildModelRoutingHeaderRegex(modelPredicates []model.AiModelPredicate) string {
	var b strings.Builder
	b.WriteString("^(")
	for i := range modelPredicates {
		modelPredicate := &modelPredicates[i]
		if i > 0 {
			b.WriteString("|")
		}
		if deref(modelPredicate.MatchType) == string(model.RoutePredicateRegular) {
			panic(errs.Business("Regular expression match is not supported for model routing header."))
		}
		b.WriteString(escapeForRegexMatch(deref(modelPredicate.MatchValue)))
		if t, _ := model.FromRoutePredicateName(deref(modelPredicate.MatchType)); t == model.RoutePredicatePrefix {
			b.WriteString(".*")
		}
	}
	b.WriteString(")")
	return b.String()
}

func escapeForRegexMatch(value string) string {
	return modelRoutingHeaderRegexEscape.ReplaceAllString(value, `\$0`)
}

func getRouteFallbackEnvoyFilterConfig(responseCodes []string) string {
	return fmt.Sprintf(routeFallbackEnvoyFilterTemplateBase, buildFallbackPredicateBlock(responseCodes))
}

func buildFallbackPredicateBlock(responseCodes []string) string {
	if len(responseCodes) == 0 {
		responseCodes = []string{consts.FallbackResponseCode4xx, consts.FallbackResponseCode5xx}
	}
	var b strings.Builder
	if len(responseCodes) == 1 {
		writeSinglePredicate(&b, 26, responseCodes[0])
	} else {
		writeIndent(&b, 26, "or_matcher:")
		writeIndent(&b, 28, "predicate:")
		for _, code := range responseCodes {
			writeIndent(&b, 30, "- single_predicate:")
			writeSinglePredicateBody(&b, 34, code)
		}
	}
	return b.String()
}

func writeSinglePredicate(b *strings.Builder, indent int, code string) {
	writeIndent(b, indent, "single_predicate:")
	writeSinglePredicateBody(b, indent+2, code)
}

func writeSinglePredicateBody(b *strings.Builder, indent int, code string) {
	writeIndent(b, indent, "input:")
	writeIndent(b, indent+2, "name: \""+code+"_response\"")
	writeIndent(b, indent+2, "typed_config:")
	writeIndent(b, indent+4, "\"@type\": type.googleapis.com/envoy.type.matcher.v3.HttpResponseStatusCodeClassMatchInput")
	writeIndent(b, indent, "value_match:")
	writeIndent(b, indent+2, "exact: \""+code+"\"")
}

func writeIndent(b *strings.Builder, indent int, line string) {
	b.WriteString(strings.Repeat(" ", indent))
	b.WriteString(line)
	b.WriteString("\n")
}

func (s *AiRouteService) setUpstreams(route *model.Route, upstreams []model.AiUpstream) {
	if len(upstreams) == 0 {
		route.Services = []model.UpstreamService{}
		return
	}
	services := make([]model.UpstreamService, 0, len(upstreams))
	for i := range upstreams {
		upstream := &upstreams[i]
		service := s.llmProviderService.BuildUpstreamService(deref(upstream.Provider))
		service.Version = nil
		service.Weight = upstream.Weight
		services = append(services, *service)
	}
	route.Services = services
}

func (s *AiRouteService) deleteAiRouteResources(aiRouteName string) {
	resourceName := buildRouteResourceName(aiRouteName)
	s.routeService.Delete(resourceName)

	if err := s.client.DeleteEnvoyFilter(context.Background(), resourceName); err != nil {
		panic(errs.Business("Error occurs when deleting the EnvoyFilter with name: " + resourceName))
	}

	s.deleteFallbackRouteResources(aiRouteName)
}

func (s *AiRouteService) deleteFallbackRouteResources(aiRouteName string) {
	originalResourceName := buildRouteResourceName(aiRouteName)
	if err := s.client.DeleteEnvoyFilter(context.Background(), originalResourceName); err != nil {
		panic(errs.Business("Error occurs when deleting the fallback EnvoyFilter: " + originalResourceName))
	}

	fallbackResourceName := buildFallbackRouteResourceName(aiRouteName)
	s.routeService.Delete(fallbackResourceName)
}

func (s *AiRouteService) saveRoute(route *model.Route) {
	existedRoute := s.routeService.Query(deref(route.Name))
	if existedRoute == nil {
		s.routeService.Add(route)
	} else {
		route.Version = existedRoute.Version
		s.routeService.Update(route)
	}
}

func buildRouteResourceName(routeName string) string {
	return consts.AiRoutePrefix + routeName + consts.InternalResourceNameSuffix
}

func buildFallbackRouteResourceName(routeName string) string {
	return consts.AiRoutePrefix + routeName + consts.FallbackRouteNameSuffix + consts.InternalResourceNameSuffix
}

func loadEnvoyFilterFromYaml(content string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	if err := yaml.Unmarshal([]byte(content), u); err != nil {
		panic(errs.Internal("Error occurs when loading route fallback envoy filter from resource."))
	}
	return u
}

func getBoolEnv(name string, def bool) bool {
	v, ok := os.LookupEnv(name)
	if !ok {
		return def
	}
	return strings.EqualFold(v, "true")
}
