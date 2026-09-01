package controller

import (
	"fmt"
	"regexp"
	"strings"

	"console/internal/consts"
	"console/internal/errs"
	"console/internal/kubernetes"
	"console/internal/model"
)

// ---- 字符串与取值工具 ----

func strPtr(s string) *string { return &s }

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func boolValue(b *bool) bool { return b != nil && *b }

func intValue(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

// isBlank 对应 StringUtils.isBlank
func isBlank(s *string) bool {
	return s == nil || strings.TrimSpace(*s) == ""
}

// isEmpty 对应 StringUtils.isEmpty
func isEmpty(s *string) bool {
	return s == nil || *s == ""
}

// ---- 校验正则（对应 ValidateUtil） ----

var (
	serviceNamePattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)
	proxyNamePattern   = regexp.MustCompile(`^[a-z][a-z0-9-_.]{0,62}$`)
	domainPattern      = regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)
	domainWildcard     = regexp.MustCompile(`^(\*\.)?(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,6}$`)
	ipv4Pattern        = regexp.MustCompile(`^(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}$`)
)

func checkServiceName(name string) bool {
	return name != "" && serviceNamePattern.MatchString(name)
}

func checkProxyName(name string) bool {
	return name != "" && proxyNamePattern.MatchString(name)
}

func checkDomain(domain string) bool {
	return domain != "" && domainPattern.MatchString(domain)
}

func checkDomainWithWildcard(domain string) bool {
	return domain != "" && domainWildcard.MatchString(domain)
}

func checkIpAddress(ip string) bool {
	return ip != "" && ipv4Pattern.MatchString(ip)
}

func checkPort(port int) bool {
	return port > 0 && port <= 65535
}

func checkUrlPath(path string) bool {
	return path != "" && path[0] == '/' && !strings.Contains(path, "?")
}

func isAsciiPrintable(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

func asString(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

func asBool(v any) (bool, bool) {
	b, ok := v.(bool)
	return b, ok
}

func asInt(v any) (int, bool) {
	switch t := v.(type) {
	case int:
		return t, true
	case int64:
		return int(t), true
	case float64:
		return int(t), true
	case float32:
		return int(t), true
	}
	return 0, false
}

func asStringList(v any) ([]string, bool) {
	list, ok := v.([]any)
	if !ok {
		// 也可能已经是 []string
		if ss, ok2 := v.([]string); ok2 {
			return ss, true
		}
		return nil, false
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		s, ok := asString(item)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

// ---- RoutePredicate 校验 ----

func routePredicateTypeFromName(name *string) (model.RoutePredicateType, bool) {
	if name == nil || *name == "" {
		return "", false
	}
	switch strings.ToUpper(*name) {
	case "EQUAL":
		return model.RoutePredicateEqual, true
	case "PRE":
		return model.RoutePredicatePrefix, true
	case "REGULAR":
		return model.RoutePredicateRegular, true
	}
	return "", false
}

func validateRoutePredicate(p *model.RoutePredicate) {
	if p.MatchType == nil {
		panic(errs.Validation("matchType is required"))
	}
	typ, ok := routePredicateTypeFromName(p.MatchType)
	if !ok {
		panic(errs.Validation("Unknown matchType: " + derefStr(p.MatchType)))
	}
	_ = typ
	if p.MatchValue == nil {
		panic(errs.Validation("matchValue is required"))
	}
}

func validateKeyedRoutePredicate(p *model.KeyedRoutePredicate, location string) {
	validateRoutePredicate(&model.RoutePredicate{MatchType: p.MatchType, MatchValue: p.MatchValue, CaseSensitive: p.CaseSensitive})

	keyValue := derefStr(p.Key)
	matchValue := derefStr(p.MatchValue)
	if keyValue != "" && !isAsciiPrintable(keyValue) {
		panic(errs.Validation(buildNonAsciiError(location, keyValue, matchValue)))
	}
	if matchValue != "" && !isAsciiPrintable(matchValue) {
		panic(errs.Validation(buildNonAsciiError(location, keyValue, matchValue)))
	}
}

func buildNonAsciiError(location, key, matchValue string) string {
	if location != "" {
		return fmt.Sprintf("Route %s predicate contains non-ASCII characters: key=%s, matchValue=%s",
			location, key, matchValue)
	}
	return fmt.Sprintf("Route predicate contains non-ASCII characters: key=%s, matchValue=%s", key, matchValue)
}

// ---- Route 校验 ----

func validateRoute(r *model.Route) {
	if isBlank(r.Name) {
		panic(errs.Validation("name cannot be blank."))
	}
	if len(r.Services) == 0 {
		panic(errs.Validation("services cannot be empty."))
	}
	if r.Path != nil {
		validateRoutePredicate(r.Path)
	}
	for i := range r.Headers {
		validateKeyedRoutePredicate(&r.Headers[i], "header")
	}
	for i := range r.UrlParams {
		validateKeyedRoutePredicate(&r.UrlParams[i], "query")
	}
	// UpstreamService.validate 与 RouteAuthConfig.validate 在 Java 中为空实现。
}

// ---- AI 模型校验 ----

func validateAiUpstream(u *model.AiUpstream) {
	if isEmpty(u.Provider) {
		panic(errs.Validation("provider cannot be null or empty."))
	}
}

func validateAiModelPredicate(p *model.AiModelPredicate) {
	validateRoutePredicate(&model.RoutePredicate{MatchType: p.MatchType, MatchValue: p.MatchValue, CaseSensitive: p.CaseSensitive})
	typ, ok := routePredicateTypeFromName(p.MatchType)
	if !ok {
		panic(errs.Validation("Unknown matchType: " + derefStr(p.MatchType)))
	}
	if typ == model.RoutePredicateRegular {
		panic(errs.Validation("AiModelPredicate does not support regular expression matchType"))
	}
}

func validateAiRouteFallbackConfig(c *model.AiRouteFallbackConfig) {
	if !boolValue(c.Enabled) {
		return
	}
	if !isEmpty(c.FallbackStrategy) {
		switch derefStr(c.FallbackStrategy) {
		case model.AiRouteFallbackRandom, model.AiRouteFallbackSequence:
		default:
			panic(errs.Validation("unknown fallback strategy: " + derefStr(c.FallbackStrategy)))
		}
	}
	if len(c.Upstreams) == 0 {
		panic(errs.Validation("upstreams cannot be empty when fallback is enabled."))
	}
	if len(c.ResponseCodes) == 0 {
		panic(errs.Validation("response codes cannot be empty when fallback is enabled."))
	} else {
		seen := map[string]bool{}
		for _, code := range c.ResponseCodes {
			if seen[code] {
				continue
			}
			seen[code] = true
			if code != consts.FallbackResponseCode4xx && code != consts.FallbackResponseCode5xx {
				panic(errs.Validation("invalid response code:" + code))
			}
		}
	}
	for i := range c.Upstreams {
		validateAiUpstream(&c.Upstreams[i])
	}
}

func validateAiRoute(r *model.AiRoute) {
	if isBlank(r.Name) {
		panic(errs.Validation("name cannot be blank."))
	}
	if len(r.Upstreams) == 0 {
		panic(errs.Validation("upstreams cannot be empty."))
	}
	if r.PathPredicate != nil {
		validateRoutePredicate(r.PathPredicate)
		typ, _ := routePredicateTypeFromName(r.PathPredicate.MatchType)
		if typ != model.RoutePredicatePrefix {
			panic(errs.Validation("pathPredicate must be of type PRE."))
		}
	}
	for i := range r.HeaderPredicates {
		validateKeyedRoutePredicate(&r.HeaderPredicates[i], "")
		if strings.EqualFold(derefStr(r.HeaderPredicates[i].Key), consts.ModelRoutingHeader) {
			panic(errs.Validation("headerPredicates cannot contain the model routing header."))
		}
	}
	for i := range r.UrlParamPredicates {
		validateKeyedRoutePredicate(&r.UrlParamPredicates[i], "")
	}
	for i := range r.Upstreams {
		validateAiUpstream(&r.Upstreams[i])
	}
	weightSum := 0
	for i := range r.Upstreams {
		weightSum += intValue(r.Upstreams[i].Weight)
	}
	if weightSum != 100 {
		panic(errs.Validation("The sum of upstream weights must be 100."))
	}
	if r.FallbackConfig != nil {
		validateAiRouteFallbackConfig(r.FallbackConfig)
	}
	for i := range r.ModelPredicates {
		validateAiModelPredicate(&r.ModelPredicates[i])
	}
}

// ---- LLM Provider 校验 ----

func validateLlmProvider(p *model.LlmProvider) {
	if isBlank(p.Name) {
		panic(errs.Validation("name cannot be blank."))
	}
	if strings.Contains(derefStr(p.Name), "/") {
		panic(errs.Validation("slashes (/) are not allowed in name."))
	}
	if isBlank(p.Type) {
		panic(errs.Validation("type cannot be blank."))
	}
	if isBlank(p.Protocol) {
		p.Protocol = strPtr(model.LlmProviderProtocolDefault)
	} else if model.LlmProviderProtocolFromValue(derefStr(p.Protocol)) == "" {
		panic(errs.Validation("Unknown protocol: " + derefStr(p.Protocol)))
	}
}

// ---- Consumer 校验 ----

func validateCredential(c *model.Credential, forUpdate bool) {
	if isBlank(c.Source) {
		panic(errs.Validation("source cannot be blank."))
	}
	source := derefStr(c.Source)
	keyRequired := false
	switch source {
	case "BEARER":
		keyRequired = false
	case "HEADER", "QUERY":
		keyRequired = true
	default:
		panic(errs.Validation("unknown source value: " + source))
	}
	if keyRequired && isBlank(c.Key) {
		panic(errs.Validation("key cannot be blank."))
	}
	if !forUpdate && len(c.Values) == 0 {
		panic(errs.Validation("value cannot be blank."))
	}
}

func validateConsumer(c *model.Consumer, forUpdate bool) {
	if isBlank(c.Name) {
		panic(errs.Validation("name cannot be blank."))
	}
	if len(c.Credentials) == 0 {
		panic(errs.Validation("credentials cannot be empty."))
	}
	for i := range c.Credentials {
		validateCredential(&c.Credentials[i], forUpdate)
	}
}

// ---- Wasm Plugin 校验 ----

func imagePullPolicyFromName(name string) bool {
	if name == "" {
		return true
	}
	switch strings.ToLower(name) {
	case "unspecified_policy", "ifnotpresent", "always", "unspecified", "default":
		return true
	}
	return false
}

func pluginPhaseFromName(name string) bool {
	if name == "" {
		return true
	}
	switch strings.ToLower(name) {
	case "unspecified_phase", "authn", "authz", "stats", "unspecified", "default":
		return true
	}
	return false
}

func validateWasmPlugin(p *model.WasmPlugin) {
	if isBlank(p.Name) {
		panic(errs.Validation("name cannot be blank."))
	}
	if !checkServiceName(derefStr(p.Name)) {
		panic(errs.Validation("Invalid name format."))
	}
	if isBlank(p.Title) {
		panic(errs.Validation("title cannot be blank."))
	}
	if isBlank(p.Category) {
		panic(errs.Validation("category cannot be blank."))
	}
	if isBlank(p.ImageRepository) {
		panic(errs.Validation("imageRepository cannot be blank."))
	}
	if !imagePullPolicyFromName(derefStr(p.ImagePullPolicy)) {
		panic(errs.Validation("Invalid imagePullPolicy: " + derefStr(p.ImagePullPolicy)))
	}
	if !pluginPhaseFromName(derefStr(p.Phase)) {
		panic(errs.Validation("Invalid phase: " + derefStr(p.Phase)))
	}
}

// ---- Proxy Server 校验 ----

func proxyServerValid(p *model.ProxyServer) bool {
	if isBlank(p.Name) || !checkProxyName(derefStr(p.Name)) {
		return false
	}
	if isBlank(p.Type) {
		return false
	}
	addr := derefStr(p.ServerAddress)
	if isBlank(p.ServerAddress) || (!checkIpAddress(addr) && !checkDomain(addr)) {
		return false
	}
	if !checkPort(intValue(p.ServerPort)) {
		return false
	}
	if p.ConnectTimeout != nil && *p.ConnectTimeout < 0 {
		return false
	}
	return true
}

// ---- Service Source 校验 ----

var (
	allowableServiceSourceTypes = []string{
		kubernetes.McpBridgeRegistryTypeNacos,
		kubernetes.McpBridgeRegistryTypeNacos2,
		kubernetes.McpBridgeRegistryTypeNacos3,
		kubernetes.McpBridgeRegistryTypeZk,
		kubernetes.McpBridgeRegistryTypeConsul,
		kubernetes.McpBridgeRegistryTypeEureka,
		kubernetes.McpBridgeRegistryTypeStatic,
		kubernetes.McpBridgeRegistryTypeDNS,
	}
	proxySupportedRegistryTypes = map[string]bool{
		kubernetes.McpBridgeRegistryTypeStatic: true,
		kubernetes.McpBridgeRegistryTypeDNS:    true,
	}
	mcpSupportedRegistryTypes = map[string]bool{
		kubernetes.McpBridgeRegistryTypeNacos3: true,
	}
)

func containsString(list []string, target string) bool {
	for _, s := range list {
		if s == target {
			return true
		}
	}
	return false
}

func validateServiceSource(s *model.ServiceSource) {
	if isBlank(s.Name) {
		panic(errs.Validation("Service source name is required."))
	}
	if isBlank(s.Type) {
		panic(errs.Validation("Service source type is required."))
	}
	if isBlank(s.Domain) {
		panic(errs.Validation("Service source domain/IP is required."))
	}
	if !checkServiceName(derefStr(s.Name)) {
		panic(errs.Validation("Invalid service source name format. Name can only contain letters, numbers, and hyphens(-), " +
			"cannot start or end with a hyphen, and must be 1-63 characters long."))
	}
	typ := derefStr(s.Type)
	if !containsString(allowableServiceSourceTypes, typ) {
		panic(errs.Validation(fmt.Sprintf("Unsupported service source type: %s. Supported types: %s", typ,
			strings.Join(allowableServiceSourceTypes, ", "))))
	}
	domain := derefStr(s.Domain)
	if typ != kubernetes.McpBridgeRegistryTypeStatic && typ != kubernetes.McpBridgeRegistryTypeDNS &&
		!checkIpAddress(domain) && !checkDomain(domain) {
		panic(errs.Validation("Invalid domain format. For " + typ + " type, domain must be a valid domain name or IP address."))
	}
	if s.Port == nil {
		panic(errs.Validation("Service source port is required."))
	}
	if !checkPort(*s.Port) {
		panic(errs.Validation("Invalid port range. Port must be an integer between 1-65535."))
	}

	validateMcpConfigs(s)
	validateServiceSourceByType(s)

	if !isEmpty(s.ProxyName) && !proxySupportedRegistryTypes[typ] {
		panic(errs.Validation("Proxy server name is only supported for static and dns types."))
	}

	if s.Vport != nil {
		if s.Vport.Default != nil && !checkPort(*s.Vport.Default) {
			panic(errs.Validation("Invalid VPort default value. Must be an integer between 1-65535."))
		}
		seen := map[string]bool{}
		for i := range s.Vport.Services {
			svc := &s.Vport.Services[i]
			if !checkPort(intValue(svc.Value)) {
				panic(errs.Validation("Invalid VPort value for service " + derefStr(svc.Name) +
					". Must be an integer between 1-65535."))
			}
			name := derefStr(svc.Name)
			if seen[name] {
				panic(errs.Validation("Duplicate service name in VPort configuration: " + name))
			}
			seen[name] = true
		}
	}
}

func validateMcpConfigs(s *model.ServiceSource) {
	if !mcpSupportedRegistryTypes[derefStr(s.Type)] {
		return
	}
	if len(s.Properties) == 0 {
		return
	}
	enabledVal, ok := s.Properties[kubernetes.McpBridgeEnableMcpServer]
	if !ok {
		return
	}
	enabled, ok := asBool(enabledVal)
	if !ok {
		panic(errs.Validation("Invalid type for enableMCPServer. Must be a boolean."))
	}
	if !enabled {
		return
	}

	if rawExport, ok := s.Properties[kubernetes.McpBridgeMcpServerExportDomains]; ok && rawExport != nil {
		exportDomains, ok := asStringList(rawExport)
		if !ok {
			panic(errs.Validation("Invalid type for mcpServerExportDomains. Must be a list of strings."))
		}
		for _, d := range exportDomains {
			if !checkDomain(d) {
				panic(errs.Validation("Invalid domain format in mcpServerExportDomains: " + d))
			}
		}
	}

	rawBaseUrl, ok := s.Properties[kubernetes.McpBridgeMcpServerBaseUrl]
	if !ok || rawBaseUrl == nil {
		rawBaseUrl = ""
	}
	baseUrl, ok := asString(rawBaseUrl)
	if !ok {
		panic(errs.Validation("Invalid type for mcpServerBaseUrl. Must be a string."))
	}
	if !checkUrlPath(baseUrl) {
		panic(errs.Validation("Invalid mcpServerBaseUrl format: " + baseUrl +
			". Must start with '/' and cannot contain '?' characters."))
	}
}

func validateServiceSourceByType(s *model.ServiceSource) {
	typ := derefStr(s.Type)
	switch typ {
	case kubernetes.McpBridgeRegistryTypeNacos, kubernetes.McpBridgeRegistryTypeNacos2, kubernetes.McpBridgeRegistryTypeNacos3:
		if len(s.Properties) == 0 {
			panic(errs.Validation("Nacos service source requires properties configuration."))
		}
		enabledMCP := false
		if v, ok := s.Properties[kubernetes.McpBridgeEnableMcpServer]; ok {
			if b, ok := asBool(v); ok {
				enabledMCP = b
			}
		}
		if typ != kubernetes.McpBridgeRegistryTypeNacos3 || !enabledMCP {
			groupsVal, ok := s.Properties[kubernetes.McpBridgeRegistryTypeNacosGroups]
			if !ok {
				panic(errs.Validation("Nacos service source requires at least one group in nacosGroups."))
			}
			groups, ok := asStringList(groupsVal)
			if !ok || len(groups) == 0 {
				panic(errs.Validation("Nacos service source requires at least one group in nacosGroups."))
			}
		}
	case kubernetes.McpBridgeRegistryTypeConsul:
		if len(s.Properties) == 0 {
			panic(errs.Validation("Consul service source requires properties configuration."))
		}
		dcVal, ok := s.Properties[kubernetes.McpBridgeRegistryTypeConsulDataCenter]
		if !ok {
			panic(errs.Validation("Consul service source requires consulDatacenter configuration."))
		}
		dc, ok := asString(dcVal)
		if !ok || strings.TrimSpace(dc) == "" {
			panic(errs.Validation("Consul service source requires consulDatacenter configuration."))
		}
		if rawInterval, ok := s.Properties[kubernetes.McpBridgeRegistryTypeConsulRefreshInterval]; ok && rawInterval != nil {
			interval, ok := asInt(rawInterval)
			if !ok {
				panic(errs.Validation("Invalid type for consulRefreshInterval. Must be an integer."))
			}
			if interval < 10 || interval > 600 {
				panic(errs.Validation(fmt.Sprintf("Invalid consulRefreshInterval value: %d. Must be between 10 and 600 seconds.", interval)))
			}
		}
	case kubernetes.McpBridgeRegistryTypeStatic:
		if isBlank(s.Domain) {
			panic(errs.Validation("Static service source requires domain configuration."))
		}
		addresses := splitAndTrim(derefStr(s.Domain), ",")
		if len(addresses) == 0 {
			panic(errs.Validation("Static service source domain format is invalid. " +
				"Must provide a list of IP:Port addresses separated by commas."))
		}
		for _, address := range addresses {
			segments := strings.Split(address, ":")
			if len(segments) != 2 {
				panic(errs.Validation("Invalid address format: " + address + ". " +
					"Correct format is IP:Port, e.g., 192.168.1.1:8080"))
			}
			if !checkIpAddress(segments[0]) {
				panic(errs.Validation("Invalid IP address format: " + segments[0]))
			}
			port, err := parsePort(segments[1])
			if err != nil {
				panic(errs.Validation("Invalid port number: " + segments[1] + ". Port must be an integer."))
			}
			if !checkPort(port) {
				panic(errs.Validation(fmt.Sprintf("Invalid port range: %d. Port must be between 1-65535.", port)))
			}
		}
	case kubernetes.McpBridgeRegistryTypeDNS:
		if isBlank(s.Domain) {
			panic(errs.Validation("DNS service source requires domain configuration."))
		}
		domains := splitAndTrim(derefStr(s.Domain), ",")
		if len(domains) == 0 {
			panic(errs.Validation("DNS service source domain format is invalid. " +
				"Must provide a list of domain names separated by commas."))
		}
		for _, d := range domains {
			if !checkDomain(d) {
				panic(errs.Validation("Invalid domain format: " + d))
			}
		}
	}
}

func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parsePort(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		return 0, err
	}
	return n, nil
}
