package model

// RoutePredicateType 对应 Java 的 RoutePredicateTypeEnum
type RoutePredicateType string

const (
	RoutePredicateEqual   RoutePredicateType = "EQUAL"
	RoutePredicatePrefix  RoutePredicateType = "PRE"
	RoutePredicateRegular RoutePredicateType = "REGULAR"
)

// AnnotationPrefix 返回 annotation 前缀
func (t RoutePredicateType) AnnotationPrefix() string {
	switch t {
	case RoutePredicateEqual:
		return "exact"
	case RoutePredicateRegular:
		return "regex"
	default:
		return "prefix"
	}
}

// FromAnnotationPrefix 从 annotation 前缀转换为枚举
func FromAnnotationPrefix(prefix string) (RoutePredicateType, bool) {
	switch prefix {
	case "exact":
		return RoutePredicateEqual, true
	case "prefix":
		return RoutePredicatePrefix, true
	case "regex":
		return RoutePredicateRegular, true
	}
	return "", false
}

// FromRoutePredicateName 从 name（忽略大小写）转换为枚举
func FromRoutePredicateName(name string) (RoutePredicateType, bool) {
	switch name {
	case "EQUAL", "equal", "exact":
		return RoutePredicateEqual, true
	case "PRE", "pre", "prefix":
		return RoutePredicatePrefix, true
	case "REGULAR", "regular", "regex":
		return RoutePredicateRegular, true
	}
	return "", false
}

// RoutePredicate 对应 Java 的 RoutePredicate
type RoutePredicate struct {
	MatchType     *string `json:"matchType,omitempty"`
	MatchValue    *string `json:"matchValue,omitempty"`
	CaseSensitive *bool   `json:"caseSensitive,omitempty"`
}

// KeyedRoutePredicate 对应 Java 的 KeyedRoutePredicate
type KeyedRoutePredicate struct {
	Key           *string `json:"key,omitempty"`
	MatchType     *string `json:"matchType,omitempty"`
	MatchValue    *string `json:"matchValue,omitempty"`
	CaseSensitive *bool   `json:"caseSensitive,omitempty"`
}

// UpstreamService 对应 Java 的 UpstreamService
type UpstreamService struct {
	Name    *string `json:"name,omitempty"`
	Port    *int    `json:"port,omitempty"`
	Version *string `json:"version,omitempty"`
	Weight  *int    `json:"weight,omitempty"`
}

// MockConfig 对应 Java 的 MockConfig
type MockConfig struct {
	Enabled *bool   `json:"enabled,omitempty"`
	Status  *int    `json:"status,omitempty"`
	Content *string `json:"content,omitempty"`
}

// RedirectConfig 对应 Java 的 RedirectConfig
type RedirectConfig struct {
	Enabled *bool   `json:"enabled,omitempty"`
	Status  *int    `json:"status,omitempty"`
	Url     *string `json:"url,omitempty"`
}

// RateLimitConfig 对应 Java 的 RateLimitConfig
type RateLimitConfig struct {
	Enabled *bool `json:"enabled,omitempty"`
	Qps     *int  `json:"qps,omitempty"`
}

// RewriteConfig 对应 Java 的 RewriteConfig
type RewriteConfig struct {
	Enabled *bool   `json:"enabled,omitempty"`
	Path    *string `json:"path,omitempty"`
	Host    *string `json:"host,omitempty"`
}

// CorsConfig 对应 Java 的 CorsConfig
type CorsConfig struct {
	Enabled          *bool    `json:"enabled,omitempty"`
	AllowOrigins     []string `json:"allowOrigins,omitempty"`
	AllowMethods     []string `json:"allowMethods,omitempty"`
	AllowHeaders     []string `json:"allowHeaders,omitempty"`
	ExposeHeaders    []string `json:"exposeHeaders,omitempty"`
	MaxAge           *int     `json:"maxAge,omitempty"`
	AllowCredentials *bool    `json:"allowCredentials,omitempty"`
}

// HeaderControlConfig 对应 Java 的 HeaderControlConfig
type HeaderControlConfig struct {
	Enabled  *bool                     `json:"enabled,omitempty"`
	Request  *HeaderControlStageConfig `json:"request,omitempty"`
	Response *HeaderControlStageConfig `json:"response,omitempty"`
}

// HeaderControlStageConfig 对应 Java 的 HeaderControlStageConfig
type HeaderControlStageConfig struct {
	Add    []Header `json:"add,omitempty"`
	Set    []Header `json:"set,omitempty"`
	Remove []string `json:"remove,omitempty"`
}

// Header 对应 Java 的 Header
type Header struct {
	Key   *string `json:"key,omitempty"`
	Value *string `json:"value,omitempty"`
}

// ProxyNextUpstreamConfig 对应 Java 的 ProxyNextUpstreamConfig
type ProxyNextUpstreamConfig struct {
	Enabled    *bool    `json:"enabled,omitempty"`
	Attempts   *int     `json:"attempts,omitempty"`
	Timeout    *int     `json:"timeout,omitempty"`
	Conditions []string `json:"conditions,omitempty"`
}

// RouteAuthConfig 对应 Java 的 RouteAuthConfig
type RouteAuthConfig struct {
	Enabled                *bool    `json:"enabled,omitempty"`
	AllowedCredentialTypes []string `json:"allowedCredentialTypes,omitempty"`
	AllowedConsumers       []string `json:"allowedConsumers,omitempty"`
}

// Route 对应 Java 的 Route
type Route struct {
	Name              *string                  `json:"name,omitempty"`
	Version           *string                  `json:"version,omitempty"`
	Domains           []string                 `json:"domains,omitempty"`
	Path              *RoutePredicate          `json:"path,omitempty"`
	Methods           []string                 `json:"methods,omitempty"`
	Headers           []KeyedRoutePredicate    `json:"headers,omitempty"`
	UrlParams         []KeyedRoutePredicate    `json:"urlParams,omitempty"`
	Services          []UpstreamService        `json:"services,omitempty"`
	Mock              *MockConfig              `json:"mock,omitempty"`
	Redirect          *RedirectConfig          `json:"redirect,omitempty"`
	RateLimit         *RateLimitConfig         `json:"rateLimit,omitempty"`
	Rewrite           *RewriteConfig           `json:"rewrite,omitempty"`
	Timeout           *string                  `json:"timeout,omitempty"`
	ProxyNextUpstream *ProxyNextUpstreamConfig `json:"proxyNextUpstream,omitempty"`
	Cors              *CorsConfig              `json:"cors,omitempty"`
	HeaderControl     *HeaderControlConfig     `json:"headerControl,omitempty"`
	AuthConfig        *RouteAuthConfig         `json:"authConfig,omitempty"`
	CustomConfigs     map[string]string        `json:"customConfigs,omitempty"`
	CustomLabels      map[string]string        `json:"customLabels,omitempty"`
	Readonly          *bool                    `json:"readonly,omitempty"`
}

func (r *Route) GetVersion() *string  { return r.Version }
func (r *Route) SetVersion(v *string) { r.Version = v }

// RoutePageQuery 对应 Java 的 RoutePageQuery
type RoutePageQuery struct {
	CommonPageQuery
	DomainName *string `json:"domainName,omitempty"`
	All        *bool   `json:"all,omitempty"`
}
