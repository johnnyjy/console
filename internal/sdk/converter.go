package sdk

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"console/internal/consts"
	"console/internal/errs"
	k8s "console/internal/kubernetes"
	"console/internal/model"
	"console/internal/util"
)

const (
	pseudoHeaderPrefix  = ":"
	defaultWeight       = 100
	serviceFqdnTemplate = "%s.%s.svc.%s"
)

var (
	supportedAnnotations = map[string]bool{
		consts.AnnotationUseRegexKey:                    true,
		consts.AnnotationDestinationKey:                 true,
		consts.AnnotationSslRedirectKey:                 true,
		consts.AnnotationRewriteEnabledKey:              true,
		consts.AnnotationRewritePathKey:                 true,
		consts.AnnotationRewriteTargetKey:               true,
		consts.AnnotationUpstreamVhostKey:               true,
		consts.AnnotationProxyNextUpstreamEnabledKey:    true,
		consts.AnnotationProxyNextUpstreamTriesKey:      true,
		consts.AnnotationProxyNextUpstreamTimeoutKey:    true,
		consts.AnnotationProxyNextUpstreamKey:           true,
		consts.AnnotationHeaderControlEnabledKey:        true,
		consts.AnnotationRequestHeaderControlAddKey:     true,
		consts.AnnotationRequestHeaderControlUpdateKey:  true,
		consts.AnnotationRequestHeaderControlRemoveKey:  true,
		consts.AnnotationResponseHeaderControlAddKey:    true,
		consts.AnnotationResponseHeaderControlUpdateKey: true,
		consts.AnnotationResponseHeaderControlRemoveKey: true,
		consts.AnnotationCorsEnabledKey:                 true,
		consts.AnnotationCorsAllowOriginKey:             true,
		consts.AnnotationCorsAllowMethodsKey:            true,
		consts.AnnotationCorsAllowHeadersKey:            true,
		consts.AnnotationCorsExposeHeadersKey:           true,
		consts.AnnotationCorsAllowCredentialsKey:        true,
		consts.AnnotationCorsMaxAgeKey:                  true,
		consts.AnnotationMethodKey:                      true,
		consts.AnnotationIgnorePathCaseKey:              true,
		consts.AnnotationWasmPluginTitleKey:             true,
		consts.AnnotationWasmPluginDescriptionKey:       true,
		consts.AnnotationWasmPluginIconKey:              true,
		consts.AnnotationCommentKey:                     true,
	}
	builtInLabels = map[string]bool{
		consts.LabelConfigMapTypeKey:      true,
		consts.LabelResourceDefinerKey:    true,
		consts.LabelInternalKey:           true,
		consts.LabelWasmPluginNameKey:     true,
		consts.LabelWasmPluginVersionKey:  true,
		consts.LabelWasmPluginBuiltInKey:  true,
		consts.LabelWasmPluginCategoryKey: true,
	}
	mcpSupportedRegistryTypes = map[string]bool{
		k8s.McpBridgeRegistryTypeNacos3: true,
	}
)

// Converter 对应 KubernetesModelConverter
type Converter struct {
	client *k8s.Client
}

// NewConverter 创建转换器
func NewConverter(client *k8s.Client) *Converter {
	return &Converter{client: client}
}

// ---- 基础类型转换 helper ----

func strPtr(s string) *string { return &s }

func optStrPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func intPtr(v int) *int    { return &v }
func boolPtr(v bool) *bool { return &v }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func boolValue(b *bool) bool { return b != nil && *b }

func boolValueDefault(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}

func mapStrPtr(m map[string]string, key string) *string {
	if v, ok := m[key]; ok {
		return &v
	}
	return nil
}

func bytesToStrPtr(b []byte) *string {
	if b == nil {
		return nil
	}
	s := string(b)
	return &s
}

// parseBool 对应 Boolean.parseBoolean / Boolean.valueOf，仅 "true"（忽略大小写）为 true
func parseBool(s string) bool {
	return strings.EqualFold(s, "true")
}

// string2Integer 对应 TypeUtil.string2Integer
func string2Integer(s string) *int {
	if s == "" {
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &v
}

func containsString(list []string, target string) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}

func removeStringFrom(list []string, target string) []string {
	out := make([]string, 0, len(list))
	removed := false
	for _, v := range list {
		if !removed && v == target {
			removed = true
			continue
		}
		out = append(out, v)
	}
	return out
}

func cloneTargets(m map[model.WasmPluginInstanceScope]*string) map[model.WasmPluginInstanceScope]*string {
	out := make(map[model.WasmPluginInstanceScope]*string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// splitLines 对应 LINE_SPLITTER：按 \n 切分、trim、忽略空
func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, consts.SeparatorNewLine) {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// splitFields 对应 FIELD_SPLITTER：按连续空格切分、trim、忽略空
func splitFields(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == ' ' })
}

// ---- Ingress <-> Route ----

// Ingress2Route 对应 ingress2Route
func (c *Converter) Ingress2Route(ingress *networkingv1.Ingress) *model.Route {
	route := &model.Route{}
	metadata := &ingress.ObjectMeta
	fillRouteMetadata(route, metadata)
	fillRouteInfo(route, metadata, &ingress.Spec)
	fillCustomConfigs(route, metadata)
	fillCustomLabels(route, metadata)
	readonly := !c.client.IsDefinedByConsole(metadata) || !isIngressSupported(ingress)
	route.Readonly = &readonly
	return route
}

// Route2Ingress 对应 route2Ingress
func (c *Converter) Route2Ingress(route *model.Route) *networkingv1.Ingress {
	ingress := &networkingv1.Ingress{}
	fillIngressMetadata(ingress, route)
	c.fillIngressSpec(ingress, route)
	fillIngressCors(ingress, route)
	fillIngressAnnotations(ingress, route)
	fillIngressLabels(ingress, route)
	return ingress
}

func isIngressSupported(ingress *networkingv1.Ingress) bool {
	if ingress == nil || len(ingress.Spec.Rules) == 0 || len(ingress.Spec.Rules) > 1 {
		return false
	}
	rule := ingress.Spec.Rules[0]
	if rule.HTTP == nil {
		return false
	}
	if len(rule.HTTP.Paths) == 0 || len(rule.HTTP.Paths) > 1 {
		return false
	}
	path := rule.HTTP.Paths[0]
	if path.PathType == nil ||
		(*path.PathType != networkingv1.PathTypeExact && *path.PathType != networkingv1.PathTypePrefix) {
		return false
	}
	if path.Backend.Service != nil {
		return false
	}
	resource := path.Backend.Resource
	if resource != nil && (resource.APIGroup == nil || *resource.APIGroup != k8s.McpBridgeAPIGroup ||
		resource.Kind != k8s.McpBridgeKind || resource.Name != k8s.McpBridgeDefaultName) {
		return false
	}
	return true
}

func fillRouteMetadata(route *model.Route, metadata *metav1.ObjectMeta) {
	route.Name = strPtr(metadata.Name)
	route.Version = strPtr(metadata.ResourceVersion)
}

func fillRouteInfo(route *model.Route, metadata *metav1.ObjectMeta, spec *networkingv1.IngressSpec) {
	if spec == nil {
		return
	}
	rules := spec.Rules
	if len(rules) == 0 {
		return
	}
	route.Domains = []string{}
	for i := range rules {
		rule := rules[i]
		if rule.HTTP == nil {
			continue
		}
		paths := rule.HTTP.Paths
		if len(paths) > 0 {
			fillPathRoute(route, metadata, paths[0])
		}
		host := rule.Host
		if host != "" {
			route.Domains = append(route.Domains, host)
		} else if len(spec.TLS) > 0 {
			for _, tlsItem := range spec.TLS {
				route.Domains = append(route.Domains, tlsItem.Hosts...)
			}
		}
	}

	var annotations map[string]string
	if metadata != nil {
		annotations = metadata.Annotations
	}
	if len(annotations) > 0 {
		fillRewriteConfig(annotations, route)
		fillProxyNextUpstreamConfig(annotations, route)
		fillHeaderAndQueryConfig(annotations, route)
		fillMethodConfig(annotations, route)
		fillHeaderConfigConfig(annotations, route)
	}
	fillRouteCors(route, metadata)
}

func fillCustomConfigs(route *model.Route, metadata *metav1.ObjectMeta) {
	if metadata == nil || len(metadata.Annotations) == 0 {
		return
	}
	customConfigs := map[string]string{}
	for k, v := range metadata.Annotations {
		if isCustomAnnotation(k) {
			customConfigs[k] = v
		}
	}
	route.CustomConfigs = customConfigs
}

func fillCustomLabels(route *model.Route, metadata *metav1.ObjectMeta) {
	if metadata == nil || len(metadata.Labels) == 0 {
		return
	}
	customLabels := map[string]string{}
	for k, v := range metadata.Labels {
		if isCustomLabel(k) {
			customLabels[k] = v
		}
	}
	route.CustomLabels = customLabels
}

func fillPathRoute(route *model.Route, metadata *metav1.ObjectMeta, path networkingv1.HTTPIngressPath) {
	fillPathPredicates(route, metadata, path)
	fillRouteDestinations(route, metadata, &path.Backend)
}

func fillPathPredicates(route *model.Route, metadata *metav1.ObjectMeta, path networkingv1.HTTPIngressPath) {
	pathPredicate := &model.RoutePredicate{}
	route.Path = pathPredicate
	pathPredicate.MatchValue = strPtr(path.Path)

	var matchType *string
	if path.PathType != nil {
		switch *path.PathType {
		case networkingv1.PathTypeExact:
			matchType = strPtr(string(model.RoutePredicateEqual))
		case networkingv1.PathTypePrefix:
			useRegex := ""
			if metadata != nil && metadata.Annotations != nil {
				useRegex = metadata.Annotations[consts.AnnotationUseRegexKey]
			}
			if useRegex == consts.AnnotationTrueValue {
				matchType = strPtr(string(model.RoutePredicateRegular))
			} else {
				matchType = strPtr(string(model.RoutePredicatePrefix))
			}
		}
	}
	pathPredicate.MatchType = matchType

	if metadata != nil && metadata.Annotations != nil {
		if ignorePathCase := metadata.Annotations[consts.AnnotationIgnorePathCaseKey]; ignorePathCase != "" {
			cs := !parseBool(ignorePathCase)
			pathPredicate.CaseSensitive = &cs
		}
	}
}

func fillRouteDestinations(route *model.Route, metadata *metav1.ObjectMeta, backend *networkingv1.IngressBackend) {
	if backend == nil || backend.Resource == nil {
		return
	}
	if metadata == nil || metadata.Annotations == nil {
		return
	}
	rawDestination := metadata.Annotations[consts.AnnotationDestinationKey]
	if rawDestination == "" {
		return
	}
	var services []model.UpstreamService
	for _, item := range splitLines(rawDestination) {
		if service := buildDestination(item); service != nil {
			services = append(services, *service)
		}
	}
	route.Services = services
}

func buildDestination(config string) *model.UpstreamService {
	fields := splitFields(config)
	if len(fields) == 0 {
		return nil
	}
	weight := defaultWeight
	addrIndex := 0
	if strings.HasSuffix(fields[0], "%") {
		weightText := fields[0]
		w, err := strconv.Atoi(weightText[:len(weightText)-1])
		if err != nil {
			return nil
		}
		weight = w
		addrIndex++
	}
	if len(fields) < addrIndex+1 {
		return nil
	}

	address := fields[addrIndex]
	host := address
	var port *int
	if colonIndex := strings.LastIndex(address, consts.SeparatorColon); colonIndex != -1 {
		rawPort := address[colonIndex+1:]
		if p, err := strconv.Atoi(rawPort); err == nil {
			port = &p
			if p > 0 && p < 65536 {
				host = address[:colonIndex]
			}
		}
	}

	var subset *string
	if len(fields) > addrIndex+1 {
		subset = strPtr(fields[addrIndex+1])
	}

	return &model.UpstreamService{
		Name:    strPtr(host),
		Port:    port,
		Version: subset,
		Weight:  &weight,
	}
}

func fillRouteCors(route *model.Route, metadata *metav1.ObjectMeta) {
	if metadata == nil || metadata.Annotations == nil {
		return
	}
	config := &model.CorsConfig{}
	config.MaxAge = string2Integer(metadata.Annotations[consts.AnnotationCorsMaxAgeKey])
	if v := metadata.Annotations[consts.AnnotationCorsEnabledKey]; v != "" {
		b := parseBool(v)
		config.Enabled = &b
	}
	if v := metadata.Annotations[consts.AnnotationCorsAllowCredentialsKey]; v != "" {
		b := parseBool(v)
		config.AllowCredentials = &b
	}
	if v := metadata.Annotations[consts.AnnotationCorsAllowOriginKey]; v != "" {
		config.AllowOrigins = strings.Split(v, consts.SeparatorComma)
	}
	if v := metadata.Annotations[consts.AnnotationCorsAllowMethodsKey]; v != "" {
		config.AllowMethods = strings.Split(v, consts.SeparatorComma)
	}
	if v := metadata.Annotations[consts.AnnotationCorsAllowHeadersKey]; v != "" {
		config.AllowHeaders = strings.Split(v, consts.SeparatorComma)
	}
	if v := metadata.Annotations[consts.AnnotationCorsExposeHeadersKey]; v != "" {
		config.ExposeHeaders = strings.Split(v, consts.SeparatorComma)
	}
	route.Cors = config
}

func fillRewriteConfig(annotations map[string]string, route *model.Route) {
	rawEnabled := annotations[consts.AnnotationRewriteEnabledKey]
	enabled := rawEnabled == "" || parseBool(rawEnabled)
	pathRewrite := getFunctionalAnnotation(annotations, consts.AnnotationRewritePathKey, enabled)
	if pathRewrite == "" {
		pathRewrite = getFunctionalAnnotation(annotations, consts.AnnotationRewriteTargetKey, enabled)
	}
	hostRewrite := getFunctionalAnnotation(annotations, consts.AnnotationUpstreamVhostKey, enabled)

	if rawEnabled == "" && pathRewrite == "" && hostRewrite == "" {
		return
	}

	route.Rewrite = &model.RewriteConfig{
		Enabled: &enabled,
		Path:    optStrPtr(pathRewrite),
		Host:    optStrPtr(hostRewrite),
	}
}

func fillProxyNextUpstreamConfig(annotations map[string]string, route *model.Route) {
	rawEnabled := annotations[consts.AnnotationProxyNextUpstreamEnabledKey]
	enabled := rawEnabled == "" || parseBool(rawEnabled)
	tries := getFunctionalAnnotation(annotations, consts.AnnotationProxyNextUpstreamTriesKey, enabled)
	timeout := getFunctionalAnnotation(annotations, consts.AnnotationProxyNextUpstreamTimeoutKey, enabled)
	conditions := getFunctionalAnnotation(annotations, consts.AnnotationProxyNextUpstreamKey, enabled)

	if rawEnabled == "" && tries == "" && timeout == "" && conditions == "" {
		return
	}

	config := &model.ProxyNextUpstreamConfig{
		Enabled:  &enabled,
		Attempts: string2Integer(tries),
		Timeout:  string2Integer(timeout),
	}
	if conditions != "" {
		config.Conditions = strings.Split(conditions, consts.SeparatorComma)
	}
	route.ProxyNextUpstream = config
}

func buildHeaderControlStageConfig(annotations map[string]string, addKey, setKey, removeKey string,
	enabled bool) *model.HeaderControlStageConfig {
	addConfig := getFunctionalAnnotation(annotations, addKey, enabled)
	setConfig := getFunctionalAnnotation(annotations, setKey, enabled)
	removeConfig := getFunctionalAnnotation(annotations, removeKey, enabled)

	if addConfig == "" && setConfig == "" && removeConfig == "" {
		return nil
	}

	config := &model.HeaderControlStageConfig{}
	if addConfig != "" {
		for _, line := range strings.Split(addConfig, consts.SeparatorNewLine) {
			if sep := strings.Index(line, consts.SeparatorSpace); sep != -1 {
				config.Add = append(config.Add, model.Header{Key: strPtr(line[:sep]), Value: strPtr(line[sep+1:])})
			} else {
				config.Add = append(config.Add, model.Header{Key: strPtr(line), Value: strPtr("")})
			}
		}
	}
	if setConfig != "" {
		for _, line := range strings.Split(setConfig, consts.SeparatorNewLine) {
			if sep := strings.Index(line, consts.SeparatorSpace); sep != -1 {
				config.Set = append(config.Set, model.Header{Key: strPtr(line[:sep]), Value: strPtr(line[sep+1:])})
			} else {
				config.Set = append(config.Set, model.Header{Key: strPtr(line), Value: strPtr("")})
			}
		}
	}
	if removeConfig != "" {
		config.Remove = strings.Split(removeConfig, consts.SeparatorComma)
	}
	return config
}

func fillHeaderAndQueryConfig(annotations map[string]string, route *model.Route) {
	var headers []model.KeyedRoutePredicate
	var urlParams []model.KeyedRoutePredicate
	for k, v := range annotations {
		if !strings.HasPrefix(k, consts.AnnotationKeyPrefix) {
			continue
		}
		key := k[len(consts.AnnotationKeyPrefix):]

		if headerPredicate := buildKeyedRoutePredicate(key, v, consts.AnnotationHeaderMatchKeyword); headerPredicate != nil {
			headers = append(headers, *headerPredicate)
			continue
		}
		if pseudoHeaderPredicate := buildKeyedRoutePredicate(key, v, consts.AnnotationPseudoHeaderMatchKeyword); pseudoHeaderPredicate != nil {
			pseudoHeaderPredicate.Key = strPtr(pseudoHeaderPrefix + deref(pseudoHeaderPredicate.Key))
			headers = append(headers, *pseudoHeaderPredicate)
			continue
		}
		if queryPredicate := buildKeyedRoutePredicate(key, v, consts.AnnotationQueryMatchKeyword); queryPredicate != nil {
			urlParams = append(urlParams, *queryPredicate)
			continue
		}
	}
	if len(headers) > 0 {
		route.Headers = headers
	}
	if len(urlParams) > 0 {
		route.UrlParams = urlParams
	}
}

func buildKeyedRoutePredicate(annotation, value, matchKeyword string) *model.KeyedRoutePredicate {
	keywordIndex := strings.Index(annotation, matchKeyword)
	if keywordIndex == -1 {
		return nil
	}
	rawType := annotation[:keywordIndex]
	key := annotation[keywordIndex+len(matchKeyword):]

	var matchType *string
	if t, ok := model.FromAnnotationPrefix(rawType); ok {
		matchType = strPtr(string(t))
	}

	return &model.KeyedRoutePredicate{
		Key:        strPtr(key),
		MatchType:  matchType,
		MatchValue: strPtr(value),
	}
}

func fillMethodConfig(annotations map[string]string, route *model.Route) {
	methods := annotations[consts.AnnotationMethodKey]
	if strings.TrimSpace(methods) != "" {
		route.Methods = strings.Split(methods, consts.SeparatorSpace)
	}
}

func fillHeaderConfigConfig(annotations map[string]string, route *model.Route) {
	rawEnabled, exists := annotations[consts.AnnotationHeaderControlEnabledKey]
	enabled := !exists || parseBool(rawEnabled)

	requestConfig := buildHeaderControlStageConfig(annotations,
		consts.AnnotationRequestHeaderControlAddKey,
		consts.AnnotationRequestHeaderControlUpdateKey,
		consts.AnnotationRequestHeaderControlRemoveKey, enabled)
	responseConfig := buildHeaderControlStageConfig(annotations,
		consts.AnnotationResponseHeaderControlAddKey,
		consts.AnnotationResponseHeaderControlUpdateKey,
		consts.AnnotationResponseHeaderControlRemoveKey, enabled)

	if requestConfig == nil && responseConfig == nil {
		return
	}

	route.HeaderControl = &model.HeaderControlConfig{
		Enabled:  &enabled,
		Request:  requestConfig,
		Response: responseConfig,
	}
}

func fillIngressMetadata(ingress *networkingv1.Ingress, route *model.Route) {
	metadata := &ingress.ObjectMeta
	metadata.Name = deref(route.Name)
	metadata.ResourceVersion = deref(route.Version)

	domains := route.Domains
	if len(domains) == 0 {
		domains = []string{consts.DefaultDomain}
	}
	for _, domain := range domains {
		setDomainLabel(metadata, domain)
	}

	for _, query := range route.UrlParams {
		setQueryAnnotation(metadata, query)
	}
	for _, header := range route.Headers {
		setHeaderAnnotation(metadata, header)
	}
	if len(route.Methods) > 0 {
		setMethodAnnotation(metadata, route.Methods)
	}

	fillIngressRewriteConfig(metadata, route.Path, route.Rewrite)
	fillIngressProxyNextUpstreamConfig(metadata, route.ProxyNextUpstream)
	if route.HeaderControl != nil {
		enabled := route.HeaderControl.Enabled == nil || *route.HeaderControl.Enabled
		k8s.SetAnnotation(metadata, consts.AnnotationHeaderControlEnabledKey, strconv.FormatBool(enabled))
		fillIngressHeaderControlStageConfig(metadata, route.HeaderControl.Request,
			consts.AnnotationRequestHeaderControlAddKey,
			consts.AnnotationRequestHeaderControlUpdateKey,
			consts.AnnotationRequestHeaderControlRemoveKey, enabled)
		fillIngressHeaderControlStageConfig(metadata, route.HeaderControl.Response,
			consts.AnnotationResponseHeaderControlAddKey,
			consts.AnnotationResponseHeaderControlUpdateKey,
			consts.AnnotationResponseHeaderControlRemoveKey, enabled)
	}
}

func fillIngressRewriteConfig(metadata *metav1.ObjectMeta, pathPredicate *model.RoutePredicate,
	rewrite *model.RewriteConfig) {
	if rewrite == nil {
		return
	}
	enabled := rewrite.Enabled == nil || *rewrite.Enabled
	k8s.SetAnnotation(metadata, consts.AnnotationRewriteEnabledKey, strconv.FormatBool(enabled))
	if deref(rewrite.Path) != "" {
		rewriteAnnotation := consts.AnnotationRewritePathKey
		if pathPredicate != nil && strings.EqualFold(deref(pathPredicate.MatchType), string(model.RoutePredicateRegular)) {
			rewriteAnnotation = consts.AnnotationRewriteTargetKey
		}
		setFunctionalAnnotation(metadata, rewriteAnnotation, deref(rewrite.Path), enabled)
	}
	if deref(rewrite.Host) != "" {
		setFunctionalAnnotation(metadata, consts.AnnotationUpstreamVhostKey, deref(rewrite.Host), enabled)
	}
}

func fillIngressProxyNextUpstreamConfig(metadata *metav1.ObjectMeta, config *model.ProxyNextUpstreamConfig) {
	if config == nil {
		return
	}
	enabled := config.Enabled == nil || *config.Enabled
	k8s.SetAnnotation(metadata, consts.AnnotationProxyNextUpstreamEnabledKey, strconv.FormatBool(enabled))
	if config.Attempts != nil {
		setFunctionalAnnotation(metadata, consts.AnnotationProxyNextUpstreamTriesKey,
			strconv.Itoa(*config.Attempts), enabled)
	}
	if config.Timeout != nil {
		setFunctionalAnnotation(metadata, consts.AnnotationProxyNextUpstreamTimeoutKey,
			strconv.Itoa(*config.Timeout), enabled)
	}
	if len(config.Conditions) > 0 {
		setFunctionalAnnotation(metadata, consts.AnnotationProxyNextUpstreamKey,
			strings.Join(config.Conditions, consts.SeparatorComma), enabled)
	}
}

func fillIngressHeaderControlStageConfig(metadata *metav1.ObjectMeta, config *model.HeaderControlStageConfig,
	addKey, setKey, removeKey string, enabled bool) {
	if config == nil {
		return
	}
	if len(config.Add) > 0 {
		var parts []string
		for _, h := range config.Add {
			if s, ok := getHeaderConfig(h); ok {
				parts = append(parts, s)
			}
		}
		setFunctionalAnnotation(metadata, addKey, strings.Join(parts, consts.SeparatorNewLine), enabled)
	}
	if len(config.Set) > 0 {
		var parts []string
		for _, h := range config.Set {
			if s, ok := getHeaderConfig(h); ok {
				parts = append(parts, s)
			}
		}
		setFunctionalAnnotation(metadata, setKey, strings.Join(parts, consts.SeparatorNewLine), enabled)
	}
	if len(config.Remove) > 0 {
		var parts []string
		for _, s := range config.Remove {
			if s != "" {
				parts = append(parts, s)
			}
		}
		setFunctionalAnnotation(metadata, removeKey, strings.Join(parts, consts.SeparatorComma), enabled)
	}
}

func getHeaderConfig(header model.Header) (string, bool) {
	if deref(header.Key) == "" {
		return "", false
	}
	if deref(header.Value) == "" {
		return deref(header.Key) + consts.SeparatorSpace, true
	}
	return deref(header.Key) + consts.SeparatorSpace + deref(header.Value), true
}

func (c *Converter) fillIngressSpec(ingress *networkingv1.Ingress, route *model.Route) {
	metadata := &ingress.ObjectMeta
	spec := &ingress.Spec
	c.fillIngressTls(metadata, spec, route)
	fillIngressRules(metadata, spec, route)
	fillIngressDestination(metadata, route)
}

func (c *Converter) fillIngressTls(metadata *metav1.ObjectMeta, spec *networkingv1.IngressSpec, route *model.Route) {
	domains := route.Domains
	if len(domains) == 0 {
		domains = []string{consts.DefaultDomain}
	}
	httpsDomains := map[string]*model.Domain{}
	for _, domainName := range domains {
		if domainName == "" {
			continue
		}
		cm, err := c.client.ReadConfigMap(context.Background(), c.DomainName2ConfigMapName(domainName))
		if err != nil {
			panic(errs.Business("Error occurs when reading config map associated with domain " + domainName))
		}
		if cm == nil {
			continue
		}
		domain := c.ConfigMap2Domain(cm)
		if deref(domain.EnableHttps) == model.DomainHttpsOff {
			continue
		}
		if deref(domain.CertIdentifier) == "" {
			continue
		}
		httpsDomains[domainName] = domain
	}

	if len(httpsDomains) > 0 && len(httpsDomains) != len(domains) {
		panic(errs.Validation("Currently only supports domains with the same protocol"))
	}

	if len(httpsDomains) > 0 {
		first := ""
		for _, domain := range httpsDomains {
			v := deref(domain.EnableHttps)
			if first == "" {
				first = v
			} else if first != v {
				panic(errs.Validation("All domains must use consistent HTTPS configuration"))
			}
		}
	}

	var tlses []networkingv1.IngressTLS
	for domainName, domain := range httpsDomains {
		tls := networkingv1.IngressTLS{}
		if domainName != consts.DefaultDomain {
			tls.Hosts = []string{domainName}
		}
		tls.SecretName = deref(domain.CertIdentifier)
		tlses = append(tlses, tls)

		if deref(domain.EnableHttps) == model.DomainHttpsForce {
			k8s.SetAnnotation(metadata, consts.AnnotationSslRedirectKey, consts.AnnotationTrueValue)
		}
	}
	if len(tlses) > 0 {
		spec.TLS = tlses
	}
}

func fillIngressRules(metadata *metav1.ObjectMeta, spec *networkingv1.IngressSpec, route *model.Route) {
	domains := route.Domains
	if len(domains) == 0 {
		domains = []string{""}
	}

	rules := make([]networkingv1.IngressRule, 0, len(domains))
	for _, d := range domains {
		rule := networkingv1.IngressRule{}
		rule.Host = d
		httpRule := &networkingv1.HTTPIngressRuleValue{}
		rule.HTTP = httpRule
		if route.Path != nil {
			fillHttpPathRule(metadata, httpRule, route.Path)
		}
		rules = append(rules, rule)
	}
	spec.Rules = rules
}

func fillHttpPathRule(metadata *metav1.ObjectMeta, httpRule *networkingv1.HTTPIngressRuleValue,
	pathPredicate *model.RoutePredicate) {
	httpPath := networkingv1.HTTPIngressPath{}
	httpPath.Path = deref(pathPredicate.MatchValue)

	var pathType networkingv1.PathType
	switch deref(pathPredicate.MatchType) {
	case string(model.RoutePredicateEqual):
		pathType = networkingv1.PathTypeExact
	case string(model.RoutePredicatePrefix):
		pathType = networkingv1.PathTypePrefix
	case string(model.RoutePredicateRegular):
		pathType = networkingv1.PathTypePrefix
		k8s.SetAnnotation(metadata, consts.AnnotationUseRegexKey, consts.AnnotationTrueValue)
	default:
		panic(errs.Internal("Unsupported path match type: " + deref(pathPredicate.MatchType)))
	}
	httpPath.PathType = &pathType

	if pathPredicate.CaseSensitive != nil {
		k8s.SetAnnotation(metadata, consts.AnnotationIgnorePathCaseKey,
			strconv.FormatBool(!*pathPredicate.CaseSensitive))
	}

	httpPath.Backend = defaultMcpBridgeBackend()
	httpRule.Paths = []networkingv1.HTTPIngressPath{httpPath}
}

func defaultMcpBridgeBackend() networkingv1.IngressBackend {
	return networkingv1.IngressBackend{
		Resource: &corev1.TypedLocalObjectReference{
			APIGroup: strPtr(k8s.McpBridgeAPIGroup),
			Kind:     k8s.McpBridgeKind,
			Name:     k8s.McpBridgeDefaultName,
		},
	}
}

func fillIngressDestination(metadata *metav1.ObjectMeta, route *model.Route) {
	services := route.Services
	if len(services) == 0 {
		return
	}

	var b strings.Builder
	if len(services) == 1 {
		service := services[0]
		b.WriteString(deref(service.Name))
		if service.Port != nil {
			b.WriteString(consts.SeparatorColon)
			b.WriteString(strconv.Itoa(*service.Port))
		}
	} else {
		for _, service := range services {
			if b.Len() > 0 {
				b.WriteString(consts.SeparatorNewLine)
			}
			weight := defaultWeight
			if service.Weight != nil {
				weight = *service.Weight
			}
			b.WriteString(strconv.Itoa(weight))
			b.WriteString("% ")
			b.WriteString(deref(service.Name))
			if service.Port != nil {
				b.WriteString(consts.SeparatorColon)
				b.WriteString(strconv.Itoa(*service.Port))
			}
			if deref(service.Version) != "" {
				b.WriteString(consts.SeparatorSpace)
				b.WriteString(deref(service.Version))
			}
		}
	}
	if b.Len() > 0 {
		k8s.SetAnnotation(metadata, consts.AnnotationDestinationKey, b.String())
	}
}

func fillIngressCors(ingress *networkingv1.Ingress, route *model.Route) {
	cors := route.Cors
	if cors == nil {
		return
	}
	metadata := &ingress.ObjectMeta
	if cors.Enabled != nil {
		k8s.SetAnnotation(metadata, consts.AnnotationCorsEnabledKey, strconv.FormatBool(*cors.Enabled))
	}
	if cors.MaxAge != nil {
		k8s.SetAnnotation(metadata, consts.AnnotationCorsMaxAgeKey, strconv.Itoa(*cors.MaxAge))
	}
	if cors.AllowCredentials != nil {
		k8s.SetAnnotation(metadata, consts.AnnotationCorsAllowCredentialsKey, strconv.FormatBool(*cors.AllowCredentials))
	}
	if len(cors.AllowOrigins) > 0 {
		k8s.SetAnnotation(metadata, consts.AnnotationCorsAllowOriginKey, strings.Join(cors.AllowOrigins, consts.SeparatorComma))
	}
	if len(cors.AllowHeaders) > 0 {
		k8s.SetAnnotation(metadata, consts.AnnotationCorsAllowHeadersKey, strings.Join(cors.AllowHeaders, consts.SeparatorComma))
	}
	if len(cors.AllowMethods) > 0 {
		k8s.SetAnnotation(metadata, consts.AnnotationCorsAllowMethodsKey, strings.Join(cors.AllowMethods, consts.SeparatorComma))
	}
	if len(cors.ExposeHeaders) > 0 {
		k8s.SetAnnotation(metadata, consts.AnnotationCorsExposeHeadersKey, strings.Join(cors.ExposeHeaders, consts.SeparatorComma))
	}
}

func fillIngressAnnotations(ingress *networkingv1.Ingress, route *model.Route) {
	if len(route.CustomConfigs) == 0 {
		return
	}
	for key, value := range route.CustomConfigs {
		if !isCustomAnnotation(key) {
			panic(errs.Validation("Annotation [" + key + "] is already supported by Console. " +
				"Please configure it in the corresponding section instead of using custom annotations."))
		}
		if strings.HasPrefix(key, consts.AnnotationNginxIngressKeyPrefix) {
			higressKey := consts.AnnotationKeyPrefix + key[len(consts.AnnotationNginxIngressKeyPrefix):]
			if !isCustomAnnotation(higressKey) {
				panic(errs.Validation("Annotation [" + key + "] is already supported by Console. " +
					"Please configure it in the corresponding section instead of using custom annotations."))
			}
		}
		k8s.SetAnnotation(&ingress.ObjectMeta, key, value)
	}
}

func fillIngressLabels(ingress *networkingv1.Ingress, route *model.Route) {
	if len(route.CustomLabels) == 0 {
		return
	}
	for key, value := range route.CustomLabels {
		k8s.SetLabel(&ingress.ObjectMeta, key, value)
	}
}

func setDomainLabel(metadata *metav1.ObjectMeta, domainName string) {
	labelName := consts.LabelDomainKeyPrefix + k8s.NormalizeDomainName(domainName)
	k8s.SetLabel(metadata, labelName, consts.LabelDomainValueDummy)
}

func setQueryAnnotation(metadata *metav1.ObjectMeta, predicate model.KeyedRoutePredicate) {
	if deref(predicate.MatchType) == "" || deref(predicate.Key) == "" || deref(predicate.MatchValue) == "" {
		return
	}
	annotationName := fmt.Sprintf(consts.AnnotationQueryMatchKeyFormat,
		routePredicateAnnotationPrefixStrict(deref(predicate.MatchType)), deref(predicate.Key))
	k8s.SetAnnotation(metadata, annotationName, deref(predicate.MatchValue))
}

func setHeaderAnnotation(metadata *metav1.ObjectMeta, predicate model.KeyedRoutePredicate) {
	if deref(predicate.MatchType) == "" || deref(predicate.Key) == "" || deref(predicate.MatchValue) == "" {
		return
	}
	key := deref(predicate.Key)
	format := consts.AnnotationHeaderMatchKeyFormat
	if strings.HasPrefix(key, pseudoHeaderPrefix) {
		key = key[len(pseudoHeaderPrefix):]
		format = consts.AnnotationPseudoHeaderMatchKeyFormat
	}
	annotationName := fmt.Sprintf(format, routePredicateAnnotationPrefixStrict(deref(predicate.MatchType)), key)
	k8s.SetAnnotation(metadata, annotationName, deref(predicate.MatchValue))
}

func setMethodAnnotation(metadata *metav1.ObjectMeta, methods []string) {
	k8s.SetAnnotation(metadata, consts.AnnotationMethodKey, strings.Join(methods, consts.SeparatorSpace))
}

// routePredicateAnnotationPrefixStrict 对应 RoutePredicateTypeEnum.valueOf（区分大小写）
func routePredicateAnnotationPrefixStrict(matchType string) string {
	switch matchType {
	case string(model.RoutePredicateEqual):
		return "exact"
	case string(model.RoutePredicatePrefix):
		return "prefix"
	case string(model.RoutePredicateRegular):
		return "regex"
	default:
		panic(errs.Internal("Unsupported route predicate type: " + matchType))
	}
}

func getFunctionalAnnotation(annotations map[string]string, key string, enabled bool) string {
	actualKey := key
	if !enabled {
		actualKey = consts.AnnotationDisabledKeyExtraPrefix + key
	}
	return annotations[actualKey]
}

func setFunctionalAnnotation(metadata *metav1.ObjectMeta, key, value string, enabled bool) {
	actualKey := key
	if !enabled {
		actualKey = consts.AnnotationDisabledKeyExtraPrefix + key
	}
	k8s.SetAnnotation(metadata, actualKey, value)
}

func isCustomAnnotation(key string) bool {
	if strings.HasPrefix(key, consts.AnnotationDisabledKeyExtraPrefix) {
		key = key[len(consts.AnnotationDisabledKeyExtraPrefix):]
	}
	if supportedAnnotations[key] {
		return false
	}
	if !strings.HasPrefix(key, consts.AnnotationKeyPrefix) {
		return true
	}
	if strings.Contains(key, consts.AnnotationHeaderMatchKeyword) ||
		strings.Contains(key, consts.AnnotationPseudoHeaderMatchKeyword) ||
		strings.Contains(key, consts.AnnotationQueryMatchKeyword) {
		return false
	}
	return true
}

func isCustomLabel(key string) bool {
	if strings.HasPrefix(key, consts.LabelDomainKeyPrefix) {
		return false
	}
	return !builtInLabels[key]
}

// ---- Domain ----

// Domain2ConfigMap 对应 domain2ConfigMap
func (c *Converter) Domain2ConfigMap(domain *model.Domain) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{}
	cm.Name = c.DomainName2ConfigMapName(deref(domain.Name))
	cm.ResourceVersion = deref(domain.Version)
	cm.Labels = map[string]string{consts.LabelConfigMapTypeKey: consts.LabelConfigMapTypeDomain}
	cm.Data = map[string]string{
		consts.CommonDomain:   deref(domain.Name),
		consts.K8sCert:        deref(domain.CertIdentifier),
		consts.K8sEnableHttps: deref(domain.EnableHttps),
	}
	return cm
}

// ConfigMap2Domain 对应 configMap2Domain
func (c *Converter) ConfigMap2Domain(cm *corev1.ConfigMap) *model.Domain {
	domain := &model.Domain{}
	domain.Version = strPtr(cm.ResourceVersion)
	if cm.Data == nil {
		panic(errs.Internal("No data is found in the ConfigMap."))
	}
	domain.Name = mapStrPtr(cm.Data, consts.CommonDomain)
	domain.CertIdentifier = mapStrPtr(cm.Data, consts.K8sCert)
	domain.EnableHttps = mapStrPtr(cm.Data, consts.K8sEnableHttps)
	return domain
}

// DomainName2ConfigMapName 对应 domainName2ConfigMapName
func (c *Converter) DomainName2ConfigMapName(domainName string) string {
	return consts.DomainPrefix + k8s.NormalizeDomainName(domainName)
}

// ---- AiRoute ----

// AiRoute2ConfigMap 对应 aiRoute2ConfigMap
func (c *Converter) AiRoute2ConfigMap(route *model.AiRoute) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{}
	cm.Name = c.AiRouteName2ConfigMapName(deref(route.Name))
	cm.ResourceVersion = deref(route.Version)
	cm.Labels = map[string]string{consts.LabelConfigMapTypeKey: consts.LabelConfigMapTypeAiRoute}

	versionBackup := route.Version
	route.Version = nil
	defer func() { route.Version = versionBackup }()

	data, err := json.Marshal(route)
	if err != nil {
		panic(errs.Business("Error occurs when serializing AiRoute."))
	}
	cm.Data = map[string]string{consts.DataField: string(data)}
	return cm
}

// ConfigMap2AiRoute 对应 configMap2AiRoute
func (c *Converter) ConfigMap2AiRoute(cm *corev1.ConfigMap) *model.AiRoute {
	if cm.Data == nil {
		panic(errs.Internal("No data is found in the ConfigMap"))
	}
	jsonData := cm.Data[consts.DataField]
	if jsonData == "" {
		panic(errs.Internal("No \"" + consts.DataField + "\" field is found in the ConfigMap"))
	}
	route := &model.AiRoute{}
	if err := json.Unmarshal([]byte(jsonData), route); err != nil {
		panic(errs.Internal("Failed to parse AiRoute data."))
	}
	route.Version = strPtr(cm.ResourceVersion)
	return route
}

// AiRouteName2ConfigMapName 对应 aiRouteName2ConfigMapName
func (c *Converter) AiRouteName2ConfigMapName(routeName string) string {
	return consts.AiRoutePrefix + routeName
}

// ---- TlsCertificate ----

// TlsCertificate2Secret 对应 tlsCertificate2Secret
func (c *Converter) TlsCertificate2Secret(certificate *model.TlsCertificate) *corev1.Secret {
	secret := &corev1.Secret{}
	secret.Name = deref(certificate.Name)
	secret.ResourceVersion = deref(certificate.Version)
	secret.Type = corev1.SecretTypeTLS
	secret.Data = map[string][]byte{
		consts.SecretTlsCrtField: []byte(deref(certificate.Cert)),
		consts.SecretTlsKeyField: []byte(deref(certificate.Key)),
	}

	if deref(certificate.Cert) != "" {
		domains := getCertBoundDomains(deref(certificate.Cert))
		for _, d := range domains {
			setDomainLabel(&secret.ObjectMeta, d)
		}
	}
	return secret
}

// Secret2TlsCertificate 对应 secret2TlsCertificate
func (c *Converter) Secret2TlsCertificate(secret *corev1.Secret) *model.TlsCertificate {
	certificate := &model.TlsCertificate{}
	certificate.Name = strPtr(secret.Name)
	certificate.Version = strPtr(secret.ResourceVersion)
	if len(secret.Data) > 0 {
		certificate.Cert = bytesToStrPtr(secret.Data[consts.SecretTlsCrtField])
		certificate.Key = bytesToStrPtr(secret.Data[consts.SecretTlsKeyField])
	}
	fillTlsCertificateDetails(certificate)
	return certificate
}

func fillTlsCertificateDetails(certificate *model.TlsCertificate) {
	certData := deref(certificate.Cert)
	if certData == "" {
		return
	}
	x509Cert := parseCertificateData(certData)
	if x509Cert == nil {
		return
	}
	certificate.Domains = getCertBoundDomainsFromCert(x509Cert)
	certificate.ValidityStart = optStrPtr(formatCertTime(x509Cert.NotBefore))
	certificate.ValidityEnd = optStrPtr(formatCertTime(x509Cert.NotAfter))
}

func parseCertificateData(certData string) *x509.Certificate {
	block, _ := pem.Decode([]byte(certData))
	if block == nil {
		return nil
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil
	}
	return cert
}

func getCertBoundDomains(certData string) []string {
	cert := parseCertificateData(certData)
	if cert == nil {
		return []string{}
	}
	return getCertBoundDomainsFromCert(cert)
}

func getCertBoundDomainsFromCert(cert *x509.Certificate) []string {
	var domains []string
	seen := map[string]bool{}
	if cn := cert.Subject.CommonName; cn != "" {
		domains = append(domains, cn)
		seen[cn] = true
	}
	for _, name := range cert.DNSNames {
		if name != "" && !seen[name] {
			domains = append(domains, name)
			seen[name] = true
		}
	}
	return domains
}

// formatCertTime 对应 @JsonFormat(pattern = "yyyy/MM/dd HH:mm:ss'Z'")
func formatCertTime(t time.Time) string {
	return t.Local().Format("2006/01/02 15:04:05Z")
}

// ---- WasmPlugin ----

// WasmPluginFromCr 对应 wasmPluginFromCr
func (c *Converter) WasmPluginFromCr(cr *k8s.WasmPlugin) *model.WasmPlugin {
	plugin := &model.WasmPlugin{}
	plugin.Version = strPtr(cr.ResourceVersion)

	if cr.Labels != nil {
		plugin.Name = strPtr(cr.Labels[consts.LabelWasmPluginNameKey])
		plugin.PluginVersion = strPtr(cr.Labels[consts.LabelWasmPluginVersionKey])
		plugin.Category = strPtr(cr.Labels[consts.LabelWasmPluginCategoryKey])
		if builtIn := cr.Labels[consts.LabelWasmPluginBuiltInKey]; builtIn != "" {
			b := parseBool(builtIn)
			plugin.BuiltIn = &b
		}
	}
	if cr.Annotations != nil {
		plugin.Title = strPtr(cr.Annotations[consts.AnnotationWasmPluginTitleKey])
		plugin.Description = strPtr(cr.Annotations[consts.AnnotationWasmPluginDescriptionKey])
		plugin.Icon = strPtr(cr.Annotations[consts.AnnotationWasmPluginIconKey])
	}

	spec := &cr.Spec
	plugin.Phase = strPtr(pluginPhaseFromName(spec.Phase))
	plugin.Priority = spec.Priority
	plugin.ImagePullPolicy = strPtr(imagePullPolicyFromName(spec.ImagePullPolicy))
	plugin.ImagePullSecret = strPtr(spec.ImagePullSecret)
	if spec.URL != "" {
		imageURL := k8s.ParseImageUrl(spec.URL)
		plugin.ImageRepository = strPtr(imageURL.Repository)
		plugin.ImageVersion = strPtr(imageURL.Tag)
	}
	return plugin
}

// WasmPluginToCr 对应 wasmPluginToCr(plugin)
func (c *Converter) WasmPluginToCr(plugin *model.WasmPlugin) *k8s.WasmPlugin {
	return wasmPluginToCr(plugin, false)
}

// WasmPluginToCrInternal 对应 wasmPluginToCr(plugin, true)
func (c *Converter) WasmPluginToCrInternal(plugin *model.WasmPlugin) *k8s.WasmPlugin {
	return wasmPluginToCr(plugin, true)
}

func wasmPluginToCr(plugin *model.WasmPlugin, internal bool) *k8s.WasmPlugin {
	cr := &k8s.WasmPlugin{}
	cr.APIVersion = k8s.WasmPluginAPIGroup + "/" + k8s.WasmPluginVersion
	cr.Kind = k8s.WasmPluginKind
	name := deref(plugin.Name)
	version := deref(plugin.PluginVersion)

	if internal {
		cr.Name = name + consts.InternalResourceNameSuffix
	} else {
		cr.Name = name + consts.SeparatorDash + version
	}

	k8s.SetLabel(&cr.ObjectMeta, consts.LabelWasmPluginNameKey, name)
	k8s.SetLabel(&cr.ObjectMeta, consts.LabelWasmPluginVersionKey, version)
	k8s.SetLabel(&cr.ObjectMeta, consts.LabelWasmPluginCategoryKey, deref(plugin.Category))
	k8s.SetLabel(&cr.ObjectMeta, consts.LabelWasmPluginBuiltInKey, strconv.FormatBool(boolValue(plugin.BuiltIn)))

	k8s.SetAnnotation(&cr.ObjectMeta, consts.AnnotationWasmPluginTitleKey, deref(plugin.Title))
	if deref(plugin.Description) != "" {
		k8s.SetAnnotation(&cr.ObjectMeta, consts.AnnotationWasmPluginDescriptionKey, deref(plugin.Description))
	}
	if deref(plugin.Icon) != "" {
		k8s.SetAnnotation(&cr.ObjectMeta, consts.AnnotationWasmPluginIconKey, deref(plugin.Icon))
	}
	cr.ResourceVersion = deref(plugin.Version)

	spec := &cr.Spec
	spec.Phase = pluginPhaseFromName(deref(plugin.Phase))
	spec.Priority = plugin.Priority
	spec.URL = buildImageUrl(deref(plugin.ImageRepository), deref(plugin.ImageVersion))
	spec.ImagePullPolicy = imagePullPolicyFromName(deref(plugin.ImagePullPolicy))
	spec.ImagePullSecret = deref(plugin.ImagePullSecret)
	setDefaultValues(spec)
	return cr
}

func pluginPhaseFromName(name string) string {
	switch strings.ToLower(name) {
	case "authn":
		return "AUTHN"
	case "authz":
		return "AUTHZ"
	case "stats":
		return "STATS"
	default:
		return "UNSPECIFIED_PHASE"
	}
}

func imagePullPolicyFromName(name string) string {
	switch strings.ToLower(name) {
	case "ifnotpresent":
		return "IfNotPresent"
	case "always":
		return "Always"
	default:
		return "UNSPECIFIED_POLICY"
	}
}

func buildImageUrl(imageRepository, imageVersion string) string {
	if strings.TrimSpace(imageRepository) == "" {
		return ""
	}
	var b strings.Builder
	if !strings.Contains(imageRepository, consts.ProtocolKeyword) {
		b.WriteString(consts.OciProtocol)
	}
	b.WriteString(imageRepository)
	if strings.TrimSpace(imageVersion) != "" {
		b.WriteString(consts.SeparatorColon)
		b.WriteString(imageVersion)
	}
	return b.String()
}

// MergeWasmPluginSpec 对应 mergeWasmPluginSpec
func (c *Converter) MergeWasmPluginSpec(srcPlugin, dstPlugin *k8s.WasmPlugin) {
	if srcPlugin == nil || dstPlugin == nil {
		return
	}
	srcSpec := &srcPlugin.Spec
	dstSpec := &dstPlugin.Spec

	dstSpec.DefaultConfig = srcSpec.DefaultConfig
	dstSpec.DefaultConfigDisable = srcSpec.DefaultConfigDisable
	if len(srcSpec.MatchRules) > 0 {
		filtered := dstSpec.MatchRules[:0]
		for _, r := range dstSpec.MatchRules {
			remove := false
			for _, sr := range srcSpec.MatchRules {
				if r.KeyEquals(sr) {
					remove = true
					break
				}
			}
			if !remove {
				filtered = append(filtered, r)
			}
		}
		dstSpec.MatchRules = filtered
		dstSpec.MatchRules = append(dstSpec.MatchRules, srcSpec.MatchRules...)
	}
	sortWasmPluginMatchRules(dstSpec.MatchRules)
	setDefaultValues(dstSpec)
}

// GetWasmPluginInstancesFromCr 对应 getWasmPluginInstancesFromCr
func (c *Converter) GetWasmPluginInstancesFromCr(plugin *k8s.WasmPlugin) []*model.WasmPluginInstance {
	if plugin == nil || len(plugin.Labels) == 0 {
		return []*model.WasmPluginInstance{}
	}

	name := plugin.Labels[consts.LabelWasmPluginNameKey]
	version := plugin.Labels[consts.LabelWasmPluginVersionKey]
	if name == "" || version == "" {
		return nil
	}

	spec := &plugin.Spec
	var instances []*model.WasmPluginInstance

	if spec.DefaultConfigDisable != nil || spec.DefaultConfig != nil {
		instance := &model.WasmPluginInstance{
			Targets:        map[model.WasmPluginInstanceScope]*string{model.ScopeGlobal: nil},
			Enabled:        boolPtr(!boolValue(spec.DefaultConfigDisable)),
			Configurations: spec.DefaultConfig,
		}
		instances = append(instances, instance)
	}

	if len(spec.MatchRules) > 0 {
		matchRules := append([]k8s.MatchRule(nil), spec.MatchRules...)
		sortWasmPluginMatchRules(matchRules)
		for _, rule := range matchRules {
			enabled := !boolValue(rule.ConfigDisable)
			var targetsList []map[model.WasmPluginInstanceScope]*string
			for _, scope := range model.NonGlobalScopes {
				targets := getTargetsByScope(rule, scope)
				if len(targets) == 0 {
					continue
				}
				if len(targetsList) == 0 {
					for _, target := range targets {
						targetsList = append(targetsList, map[model.WasmPluginInstanceScope]*string{scope: strPtr(target)})
					}
				} else {
					var newTargetsList []map[model.WasmPluginInstanceScope]*string
					for _, existedTargets := range targetsList {
						for _, target := range targets {
							newTargets := cloneTargets(existedTargets)
							newTargets[scope] = strPtr(target)
							newTargetsList = append(newTargetsList, newTargets)
						}
					}
					targetsList = newTargetsList
				}
			}
			for _, targets := range targetsList {
				instances = append(instances, &model.WasmPluginInstance{
					Targets:        targets,
					Enabled:        &enabled,
					Configurations: rule.Config,
				})
			}
		}
	}

	filtered := instances[:0]
	for _, instance := range instances {
		if !boolValue(instance.Enabled) && len(instance.Configurations) == 0 {
			continue
		}
		filtered = append(filtered, instance)
	}
	instances = filtered

	for _, instance := range instances {
		instance.PluginName = strPtr(name)
		instance.PluginVersion = strPtr(version)
		instance.Version = strPtr(plugin.ResourceVersion)
		normalizePluginInstanceConfigurations(instance.Configurations)
		instance.RawConfigurations = strPtr(generateRawConfigurations(instance.Configurations))
		internal := k8s.IsInternalResource(plugin.Name)
		instance.Internal = &internal
		instance.SyncDeprecatedFields()
	}
	return instances
}

// GetWasmPluginInstanceFromCr 对应 getWasmPluginInstanceFromCr(plugin, targets)
func (c *Converter) GetWasmPluginInstanceFromCr(plugin *k8s.WasmPlugin,
	targets map[model.WasmPluginInstanceScope]*string) *model.WasmPluginInstance {
	return getWasmPluginInstanceFromCr(plugin, targets)
}

// GetWasmPluginInstanceFromCrByScope 对应 getWasmPluginInstanceFromCr(plugin, scope, target)
func (c *Converter) GetWasmPluginInstanceFromCrByScope(plugin *k8s.WasmPlugin,
	scope model.WasmPluginInstanceScope, target string) *model.WasmPluginInstance {
	return getWasmPluginInstanceFromCr(plugin, map[model.WasmPluginInstanceScope]*string{scope: optStrPtr(target)})
}

func getWasmPluginInstanceFromCr(plugin *k8s.WasmPlugin,
	targets map[model.WasmPluginInstanceScope]*string) *model.WasmPluginInstance {
	if plugin == nil || len(targets) == 0 || len(plugin.Labels) == 0 {
		return nil
	}

	name := plugin.Labels[consts.LabelWasmPluginNameKey]
	version := plugin.Labels[consts.LabelWasmPluginVersionKey]
	if name == "" || version == "" {
		return nil
	}

	spec := &plugin.Spec

	var enabled *bool
	var configurations map[string]any

	if _, ok := targets[model.ScopeGlobal]; ok {
		if len(targets) != 1 {
			return nil
		}
		if targets[model.ScopeGlobal] != nil {
			return nil
		}
		e := !boolValue(spec.DefaultConfigDisable)
		enabled = &e
		configurations = spec.DefaultConfig
		if configurations == nil {
			configurations = map[string]any{}
		}
	} else if len(spec.MatchRules) > 0 {
		for _, rule := range spec.MatchRules {
			matched := true
			for scope, target := range targets {
				if target == nil || *target == "" {
					continue
				}
				targetsInRule := getTargetsByScope(rule, scope)
				if len(targetsInRule) != 1 || *target != targetsInRule[0] {
					matched = false
					break
				}
			}
			if matched {
				e := !boolValue(rule.ConfigDisable)
				enabled = &e
				configurations = rule.Config
				break
			}
		}
	}

	if enabled == nil {
		return nil
	}
	if !*enabled && len(configurations) == 0 {
		return nil
	}

	normalizePluginInstanceConfigurations(configurations)
	rawConfiguration := generateRawConfigurations(configurations)

	instance := &model.WasmPluginInstance{
		Version:           strPtr(plugin.ResourceVersion),
		PluginName:        strPtr(name),
		PluginVersion:     strPtr(version),
		Targets:           cloneTargets(targets),
		Enabled:           enabled,
		Configurations:    configurations,
		RawConfigurations: strPtr(rawConfiguration),
		Internal:          boolPtr(k8s.IsInternalResource(plugin.Name)),
	}
	instance.SyncDeprecatedFields()
	return instance
}

func getTargetsByScope(rule k8s.MatchRule, scope model.WasmPluginInstanceScope) []string {
	switch scope {
	case model.ScopeGlobal:
		return nil
	case model.ScopeDomain:
		return rule.Domain
	case model.ScopeRoute:
		return rule.Ingress
	case model.ScopeService:
		return rule.Service
	default:
		panic(errs.Internal("Unsupported scope: " + string(scope)))
	}
}

func setTargetByScope(rule *k8s.MatchRule, scope model.WasmPluginInstanceScope, target string) {
	switch scope {
	case model.ScopeGlobal:
	case model.ScopeDomain:
		rule.Domain = []string{target}
	case model.ScopeRoute:
		rule.Ingress = []string{target}
	case model.ScopeService:
		rule.Service = []string{target}
	default:
		panic(errs.Internal("Unsupported scope: " + string(scope)))
	}
}

func setTargetsByScope(rule *k8s.MatchRule, scope model.WasmPluginInstanceScope, targets []string) {
	switch scope {
	case model.ScopeDomain:
		rule.Domain = targets
	case model.ScopeRoute:
		rule.Ingress = targets
	case model.ScopeService:
		rule.Service = targets
	}
}

func normalizePluginInstanceConfigurations(configurations map[string]any) {
	if len(configurations) == 0 {
		return
	}
	for key, value := range configurations {
		switch v := value.(type) {
		case nil:
			continue
		case map[string]any:
			normalizePluginInstanceConfigurations(v)
		case []any:
			for _, item := range v {
				if m, ok := item.(map[string]any); ok {
					normalizePluginInstanceConfigurations(m)
				}
			}
		case float64:
			if v == float64(int(v)) {
				configurations[key] = int(v)
			}
		}
	}
}

func generateRawConfigurations(configurations map[string]any) string {
	if len(configurations) == 0 {
		return ""
	}
	data, err := yaml.Marshal(configurations)
	if err != nil {
		panic(errs.Business("Error occurs when converting object to yaml: " + err.Error()))
	}
	raw := strings.TrimPrefix(string(data), consts.YamlSeparator)
	return strings.TrimSpace(raw)
}

// SetWasmPluginInstanceToCr 对应 setWasmPluginInstanceToCr
func (c *Converter) SetWasmPluginInstanceToCr(cr *k8s.WasmPlugin, instance *model.WasmPluginInstance) {
	instance.SyncDeprecatedFields()

	spec := &cr.Spec
	enabled := instance.Enabled == nil || *instance.Enabled
	configurations := instance.Configurations
	targets := instance.Targets

	if len(targets) > 0 {
		if _, ok := targets[model.ScopeGlobal]; ok {
			if len(targets) == 1 && targets[model.ScopeGlobal] == nil {
				spec.DefaultConfigDisable = boolPtr(!enabled)
				spec.DefaultConfig = configurations
			}
		} else {
			targetMatchRule := k8s.MatchRule{
				ConfigDisable: boolPtr(!enabled),
				Config:        configurations,
			}
			for scope, target := range targets {
				setTargetByScope(&targetMatchRule, scope, deref(target))
			}

			filtered := spec.MatchRules[:0]
			for _, r := range spec.MatchRules {
				if !r.KeyEquals(targetMatchRule) {
					filtered = append(filtered, r)
				}
			}
			spec.MatchRules = filtered
			spec.MatchRules = append(spec.MatchRules, targetMatchRule)
		}
	}

	sortWasmPluginMatchRules(spec.MatchRules)
	setDefaultValues(spec)
}

// RemoveWasmPluginInstanceFromCr 对应 removeWasmPluginInstanceFromCr(plugin, targets)
func (c *Converter) RemoveWasmPluginInstanceFromCr(cr *k8s.WasmPlugin,
	targets map[model.WasmPluginInstanceScope]*string) bool {
	return removeWasmPluginInstanceFromCr(cr, targets)
}

// RemoveWasmPluginInstanceFromCrByScope 对应 removeWasmPluginInstanceFromCr(plugin, scope, target)
func (c *Converter) RemoveWasmPluginInstanceFromCrByScope(cr *k8s.WasmPlugin,
	scope model.WasmPluginInstanceScope, target string) bool {
	return removeWasmPluginInstanceFromCr(cr, map[model.WasmPluginInstanceScope]*string{scope: optStrPtr(target)})
}

func removeWasmPluginInstanceFromCr(cr *k8s.WasmPlugin,
	targets map[model.WasmPluginInstanceScope]*string) bool {
	if cr == nil || len(targets) == 0 {
		return false
	}
	spec := &cr.Spec

	if _, ok := targets[model.ScopeGlobal]; ok {
		if len(targets) != 1 || targets[model.ScopeGlobal] != nil {
			return false
		}
		spec.DefaultConfigDisable = boolPtr(true)
		spec.DefaultConfig = nil
		return true
	}

	if len(spec.MatchRules) == 0 {
		return false
	}

	changed := false
	newRules := make([]k8s.MatchRule, 0, len(spec.MatchRules))
	for _, rule := range spec.MatchRules {
		matches := true
		for scope, target := range targets {
			targetsInRule := getTargetsByScope(rule, scope)
			if targetsInRule == nil || !containsString(targetsInRule, deref(target)) {
				matches = false
				break
			}
		}
		if !matches {
			newRules = append(newRules, rule)
			continue
		}

		removeRule := false
		for scope, target := range targets {
			targetsInRule := getTargetsByScope(rule, scope)
			newTargets := removeStringFrom(targetsInRule, deref(target))
			setTargetsByScope(&rule, scope, newTargets)
			if len(newTargets) == 0 {
				removeRule = true
			}
		}
		if !removeRule {
			newRules = append(newRules, rule)
		}
		changed = true
	}
	spec.MatchRules = newRules
	return changed
}

func sortWasmPluginMatchRules(matchRules []k8s.MatchRule) {
	if len(matchRules) == 0 {
		return
	}
	sort.SliceStable(matchRules, func(i, j int) bool {
		return compareMatchRules(matchRules[i], matchRules[j]) < 0
	})
}

func compareMatchRules(r1, r2 k8s.MatchRule) int {
	hasDomain1 := len(r1.Domain) > 0
	hasDomain2 := len(r2.Domain) > 0
	hasIngress1 := len(r1.Ingress) > 0
	hasIngress2 := len(r2.Ingress) > 0
	hasService1 := len(r1.Service) > 0
	hasService2 := len(r2.Service) > 0

	empty1 := !hasDomain1 && !hasIngress1 && !hasService1
	empty2 := !hasDomain2 && !hasIngress2 && !hasService2
	if empty1 && empty2 {
		return 0
	}
	if empty1 != empty2 {
		if empty1 {
			return 1
		}
		return -1
	}

	if hasService1 != hasService2 {
		if hasService1 {
			return -1
		}
		return 1
	}
	if hasService1 {
		if ret := compareStringLists(r1.Service, r2.Service); ret != 0 {
			return ret
		}
	}

	if hasIngress1 != hasIngress2 {
		if hasIngress1 {
			return -1
		}
		return 1
	}
	if hasIngress1 {
		if ret := compareStringLists(r1.Ingress, r2.Ingress); ret != 0 {
			return ret
		}
	}

	if hasDomain1 != hasDomain2 {
		if hasDomain1 {
			return 1
		}
		return -1
	}
	if hasDomain1 {
		return compareStringLists(r1.Domain, r2.Domain)
	}
	return 0
}

func compareStringLists(l1, l2 []string) int {
	empty1 := len(l1) == 0
	empty2 := len(l2) == 0
	if empty1 && empty2 {
		return 0
	}
	if empty1 != empty2 {
		if empty1 {
			return 1
		}
		return -1
	}
	n := len(l1)
	if len(l2) > n {
		n = len(l2)
	}
	for i := 0; i < n; i++ {
		if i >= len(l1) {
			return -1
		}
		if i >= len(l2) {
			return 1
		}
		if l1[i] < l2[i] {
			return -1
		}
		if l1[i] > l2[i] {
			return 1
		}
	}
	return 0
}

func setDefaultValues(spec *k8s.WasmPluginSpec) {
	spec.FailStrategy = "FAIL_OPEN"
	if spec.DefaultConfigDisable == nil {
		spec.DefaultConfigDisable = boolPtr(true)
	}
}

// ---- Service / ServiceSource / ProxyServer ----

// V1Service2Service 对应 v1Service2Service
func (c *Converter) V1Service2Service(v1Service *corev1.Service) *model.Service {
	result := &model.Service{}
	fqdn := fmt.Sprintf(serviceFqdnTemplate, v1Service.Name, v1Service.Namespace, c.client.ClusterDomainSuffix)
	result.Name = strPtr(fqdn)
	result.Namespace = strPtr(v1Service.Namespace)

	ports := v1Service.Spec.Ports
	if len(ports) > 0 {
		result.Port = intPtr(int(ports[0].Port))
	}
	if v1Service.ResourceVersion != "" {
		if v, err := strconv.Atoi(v1Service.ResourceVersion); err == nil {
			result.Version = &v
		}
	}
	result.Endpoints = v1Service.Spec.ClusterIPs
	return result
}

// V1RegistryConfig2ServiceSource 对应 v1RegistryConfig2ServiceSource
func (c *Converter) V1RegistryConfig2ServiceSource(rc *k8s.RegistryConfig) *model.ServiceSource {
	serviceSource := &model.ServiceSource{}
	fillServiceSourceInfo(serviceSource, rc)
	return serviceSource
}

// V1ProxyConfig2ProxyServer 对应 v1ProxyConfig2ProxyServer
func (c *Converter) V1ProxyConfig2ProxyServer(pc *k8s.ProxyConfig) *model.ProxyServer {
	proxyServer := &model.ProxyServer{}
	fillProxyServerInfo(proxyServer, pc)
	return proxyServer
}

// GenerateAuthSecretName 对应 generateAuthSecretName
func (c *Converter) GenerateAuthSecretName(serviceSourceName string) string {
	return serviceSourceName + "-auth-" + strings.ToLower(util.RandomAlphanumeric(5))
}

// InitV1McpBridge 对应 initV1McpBridge
func (c *Converter) InitV1McpBridge(mcpBridge *k8s.McpBridge) {
	mcpBridge.APIVersion = k8s.McpBridgeAPIGroup + "/" + k8s.McpBridgeVersion
	mcpBridge.Kind = k8s.McpBridgeKind
	mcpBridge.Name = k8s.McpBridgeDefaultName
	mcpBridge.Spec.Registries = []k8s.RegistryConfig{}
	mcpBridge.Spec.Proxies = []k8s.ProxyConfig{}
}

// AddV1McpBridgeRegistry 对应 addV1McpBridgeRegistry
func (c *Converter) AddV1McpBridgeRegistry(mcpBridge *k8s.McpBridge,
	serviceSource *model.ServiceSource) *k8s.RegistryConfig {
	spec := &mcpBridge.Spec
	for i := range spec.Registries {
		if spec.Registries[i].Name != "" && spec.Registries[i].Name == deref(serviceSource.Name) {
			fillRegistryConfig(&spec.Registries[i], serviceSource)
			return &spec.Registries[i]
		}
	}
	registry := serviceSource2RegistryConfig(serviceSource)
	spec.Registries = append(spec.Registries, *registry)
	return &spec.Registries[len(spec.Registries)-1]
}

// RemoveV1McpBridgeRegistry 对应 removeV1McpBridgeRegistry
func (c *Converter) RemoveV1McpBridgeRegistry(mcpBridge *k8s.McpBridge, name string) *k8s.RegistryConfig {
	spec := &mcpBridge.Spec
	if len(spec.Registries) == 0 {
		return nil
	}
	var target *k8s.RegistryConfig
	newList := make([]k8s.RegistryConfig, 0, len(spec.Registries))
	for i := range spec.Registries {
		if spec.Registries[i].Name == name {
			rc := spec.Registries[i]
			target = &rc
		} else {
			newList = append(newList, spec.Registries[i])
		}
	}
	spec.Registries = newList
	return target
}

// AddV1McpBridgeProxy 对应 addV1McpBridgeProxy
func (c *Converter) AddV1McpBridgeProxy(mcpBridge *k8s.McpBridge,
	proxyServer *model.ProxyServer) *k8s.ProxyConfig {
	spec := &mcpBridge.Spec
	for i := range spec.Proxies {
		if spec.Proxies[i].Name != "" && spec.Proxies[i].Name == deref(proxyServer.Name) {
			fillProxyConfig(&spec.Proxies[i], proxyServer)
			return &spec.Proxies[i]
		}
	}
	proxy := proxyServer2ProxyConfig(proxyServer)
	spec.Proxies = append(spec.Proxies, *proxy)
	return &spec.Proxies[len(spec.Proxies)-1]
}

// RemoveV1McpBridgeProxy 对应 removeV1McpBridgeProxy
func (c *Converter) RemoveV1McpBridgeProxy(mcpBridge *k8s.McpBridge, name string) *k8s.ProxyConfig {
	spec := &mcpBridge.Spec
	if len(spec.Proxies) == 0 {
		return nil
	}
	var target *k8s.ProxyConfig
	newList := make([]k8s.ProxyConfig, 0, len(spec.Proxies))
	for i := range spec.Proxies {
		if spec.Proxies[i].Name == name {
			pc := spec.Proxies[i]
			target = &pc
		} else {
			newList = append(newList, spec.Proxies[i])
		}
	}
	spec.Proxies = newList
	return target
}

func fillServiceSourceInfo(serviceSource *model.ServiceSource, rc *k8s.RegistryConfig) {
	if rc == nil {
		return
	}
	serviceSource.Domain = strPtr(rc.Domain)
	serviceSource.Type = strPtr(rc.Type)
	serviceSource.Port = rc.Port
	serviceSource.Name = strPtr(rc.Name)
	serviceSource.Protocol = strPtr(rc.Protocol)
	serviceSource.Sni = strPtr(rc.Sni)
	serviceSource.ProxyName = strPtr(rc.ProxyName)
	serviceSource.Properties = map[string]any{}
	serviceSource.Vport = toModelVPort(rc.VPort)

	switch rc.Type {
	case k8s.McpBridgeRegistryTypeNacos, k8s.McpBridgeRegistryTypeNacos2, k8s.McpBridgeRegistryTypeNacos3:
		serviceSource.Properties[k8s.McpBridgeRegistryTypeNacosNamespaceId] = rc.NacosNamespaceId
		serviceSource.Properties[k8s.McpBridgeRegistryTypeNacosGroups] = rc.NacosGroups
	case k8s.McpBridgeRegistryTypeZk:
		serviceSource.Properties[k8s.McpBridgeRegistryTypeZkServicesPath] = rc.ZkServicesPath
	case k8s.McpBridgeRegistryTypeConsul:
		serviceSource.Properties[k8s.McpBridgeRegistryTypeConsulDataCenter] = rc.ConsulDataCenter
		serviceSource.Properties[k8s.McpBridgeRegistryTypeConsulServiceTag] = rc.ConsulServiceTag
		serviceSource.Properties[k8s.McpBridgeRegistryTypeConsulRefreshInterval] = rc.ConsulRefreshInterval
	}

	if mcpSupportedRegistryTypes[rc.Type] {
		serviceSource.Properties[k8s.McpBridgeEnableMcpServer] = boolValueDefault(rc.EnableMcpServer, false)
		serviceSource.Properties[k8s.McpBridgeMcpServerBaseUrl] = rc.McpServerBaseUrl
		serviceSource.Properties[k8s.McpBridgeMcpServerExportDomains] = rc.McpServerExportDomains
	}
}

func serviceSource2RegistryConfig(serviceSource *model.ServiceSource) *k8s.RegistryConfig {
	if serviceSource == nil {
		return nil
	}
	rc := &k8s.RegistryConfig{}
	fillRegistryConfig(rc, serviceSource)
	return rc
}

func fillRegistryConfig(rc *k8s.RegistryConfig, serviceSource *model.ServiceSource) {
	if serviceSource == nil {
		return
	}
	rc.Domain = deref(serviceSource.Domain)
	rc.Type = deref(serviceSource.Type)
	rc.Port = serviceSource.Port
	rc.Name = deref(serviceSource.Name)
	rc.Protocol = deref(serviceSource.Protocol)
	rc.Sni = deref(serviceSource.Sni)
	rc.ProxyName = deref(serviceSource.ProxyName)
	rc.VPort = toK8sVPort(serviceSource.Vport)

	properties := serviceSource.Properties
	if properties == nil {
		properties = map[string]any{}
	}

	switch rc.Type {
	case k8s.McpBridgeRegistryTypeNacos, k8s.McpBridgeRegistryTypeNacos2, k8s.McpBridgeRegistryTypeNacos3:
		rc.NacosNamespaceId = anyStrOr(properties[k8s.McpBridgeRegistryTypeNacosNamespaceId], "")
		rc.NacosGroups = anyStrSliceOr(properties[k8s.McpBridgeRegistryTypeNacosGroups], []string{})
	case k8s.McpBridgeRegistryTypeZk:
		rc.ZkServicesPath = anyStrSliceOr(properties[k8s.McpBridgeRegistryTypeZkServicesPath], []string{})
	case k8s.McpBridgeRegistryTypeConsul:
		rc.ConsulDataCenter = anyStrOr(properties[k8s.McpBridgeRegistryTypeConsulDataCenter], "")
		rc.ConsulServiceTag = anyStrOr(properties[k8s.McpBridgeRegistryTypeConsulServiceTag], "")
		rc.ConsulRefreshInterval = anyIntOrNil(properties[k8s.McpBridgeRegistryTypeConsulRefreshInterval])
	}

	if mcpSupportedRegistryTypes[rc.Type] {
		rc.EnableMcpServer = boolPtr(anyBoolOr(properties[k8s.McpBridgeEnableMcpServer], false))
		rc.McpServerBaseUrl = anyStrOr(properties[k8s.McpBridgeMcpServerBaseUrl], "")
		rc.McpServerExportDomains = anyStrSliceOr(properties[k8s.McpBridgeMcpServerExportDomains], []string{})
	}
}

func fillProxyServerInfo(proxyServer *model.ProxyServer, pc *k8s.ProxyConfig) {
	if pc == nil {
		return
	}
	proxyServer.Type = strPtr(pc.Type)
	proxyServer.Name = strPtr(pc.Name)
	proxyServer.ServerAddress = strPtr(pc.ServerAddress)
	proxyServer.ServerPort = pc.ServerPort
	proxyServer.ConnectTimeout = pc.ConnectTimeout
}

func proxyServer2ProxyConfig(proxyServer *model.ProxyServer) *k8s.ProxyConfig {
	if proxyServer == nil {
		return nil
	}
	pc := &k8s.ProxyConfig{}
	fillProxyConfig(pc, proxyServer)
	return pc
}

func fillProxyConfig(pc *k8s.ProxyConfig, proxyServer *model.ProxyServer) {
	if proxyServer == nil {
		return
	}
	pc.Type = deref(proxyServer.Type)
	pc.Name = deref(proxyServer.Name)
	pc.ServerAddress = deref(proxyServer.ServerAddress)
	pc.ServerPort = proxyServer.ServerPort
	pc.ConnectTimeout = proxyServer.ConnectTimeout
}

// ---- VPort 映射 ----

func toModelVPort(v *k8s.VPort) *model.VPort {
	if v == nil {
		return nil
	}
	out := &model.VPort{Default: v.DefaultValue}
	if len(v.Services) > 0 {
		out.Services = make([]model.ServiceVport, 0, len(v.Services))
		for _, s := range v.Services {
			name := s.Name
			out.Services = append(out.Services, model.ServiceVport{Name: &name, Value: s.Value})
		}
	}
	return out
}

func toK8sVPort(v *model.VPort) *k8s.VPort {
	if v == nil {
		return nil
	}
	out := &k8s.VPort{DefaultValue: v.Default}
	if len(v.Services) > 0 {
		out.Services = make([]k8s.ServiceVPort, 0, len(v.Services))
		for _, s := range v.Services {
			out.Services = append(out.Services, k8s.ServiceVPort{Name: deref(s.Name), Value: s.Value})
		}
	}
	return out
}

// ---- any 类型转换 helper（用于 Properties） ----

func anyStrOr(v any, def string) string {
	if s, ok := v.(string); ok {
		return s
	}
	return def
}

func anyStrSliceOr(v any, def []string) []string {
	if v == nil {
		return def
	}
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return def
}

func anyIntOrNil(v any) *int {
	switch t := v.(type) {
	case nil:
		return nil
	case int:
		return &t
	case int64:
		i := int(t)
		return &i
	case float64:
		i := int(t)
		return &i
	case json.Number:
		if i, err := t.Int64(); err == nil {
			r := int(i)
			return &r
		}
	case string:
		if i, err := strconv.Atoi(t); err == nil {
			return &i
		}
	}
	return nil
}

func anyBoolOr(v any, def bool) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}
