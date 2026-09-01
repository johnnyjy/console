package service

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"console/internal/consts"
	"console/internal/errs"
	"console/internal/kubernetes"
	"console/internal/model"
	"console/resource"
)

const (
	dashboardDatasourceUidPlaceholder = "${datasource.id}"
	dashboardPromDatasourceType       = "prometheus"
	dashboardLokiDatasourceType       = "loki"
	dashboardDatasourceAccess         = "proxy"
	dashboardSearchTypeDb             = "dash-db"
	dashboardInitializeRetryInterval  = 5 * time.Second
)

var (
	ignoreRequestHeaders = map[string]bool{
		"connection":          true,
		"keep-alive":          true,
		"proxy-authenticate":  true,
		"proxy-authorization": true,
		"te":                  true,
		"trailers":            true,
		"upgrade":             true,
		"transfer-encoding":   true,
		"content-length":      true,
		"accept-encoding":     true,
	}
	ignoreResponseHeaders = map[string]bool{
		"connection":          true,
		"keep-alive":          true,
		"proxy-authenticate":  true,
		"proxy-authorization": true,
		"te":                  true,
		"trailers":            true,
		"upgrade":             true,
		"transfer-encoding":   true,
		"content-length":      true,
		"content-encoding":    true,
		"server":              true,
	}
)

type dashboardConfiguration struct {
	typ       model.DashboardType
	configKey string
	raw       string
	dashboard *GrafanaDashboard
}

// DashboardService 对应 Java 的 DashboardServiceImpl
type DashboardService struct {
	configService *ConfigService
	grafanaClient *GrafanaClient

	realServerClient  *http.Client
	realServerBaseUrl string
	apiBaseUrlPath    string

	overwriteWhenStartUp bool
	username             string
	password             string
	promDatasourceName   string
	promDatasourceUrl    string
	lokiDatasourceName   string
	lokiDatasourceUrl    string
	proxyConnTimeout     time.Duration
	proxySocketTimeout   time.Duration

	dashboardConfigurations map[model.DashboardType]*dashboardConfiguration
}

// NewDashboardService 对应 DashboardServiceImpl 构造 + @PostConstruct initialize
func NewDashboardService(configService *ConfigService) *DashboardService {
	s := &DashboardService{
		configService: configService,
		overwriteWhenStartUp: kubernetes.EnvGet(consts.DashboardOverwriteStartup,
			strconv.FormatBool(consts.DashboardOverwriteStartupDef)) == "true",
		username:           kubernetes.EnvGet(consts.DashboardUsernameKey, consts.DashboardUsernameDefault),
		password:           kubernetes.EnvGet(consts.DashboardPasswordKey, consts.DashboardPasswordDefault),
		promDatasourceName: kubernetes.EnvGet(consts.DashboardDsPromNameKey, consts.DashboardDsPromNameDefault),
		promDatasourceUrl:  kubernetes.EnvGet(consts.DashboardDsPromUrlKey, ""),
		lokiDatasourceName: kubernetes.EnvGet(consts.DashboardDsLokiNameKey, consts.DashboardDsLokiNameDefault),
		lokiDatasourceUrl:  kubernetes.EnvGet(consts.DashboardDsLokiUrlKey, ""),
		proxyConnTimeout:   time.Duration(kubernetes.EnvInt(consts.DashboardProxyConnTimeout, consts.DashboardProxyConnTimeoutDef)) * time.Millisecond,
		proxySocketTimeout: time.Duration(kubernetes.EnvInt(consts.DashboardProxySocketTimeout, consts.DashboardProxySocketTimeoutDef)) * time.Millisecond,
	}

	s.dashboardConfigurations = map[model.DashboardType]*dashboardConfiguration{
		model.DashboardMain: s.newDashboardConfiguration(model.DashboardMain, resource.DashboardMain),
		model.DashboardAi:   s.newDashboardConfiguration(model.DashboardAi, resource.DashboardAi),
		model.DashboardLog:  s.newDashboardConfiguration(model.DashboardLog, resource.DashboardLogs),
	}

	if s.isBuiltIn() {
		apiBaseUrl := kubernetes.EnvGet(consts.DashboardBaseUrlKey, "")
		u, err := url.Parse(apiBaseUrl)
		if err != nil {
			panic(errs.Internal("Invalid dashboard base url: " + apiBaseUrl))
		}
		s.apiBaseUrlPath = u.Path
		s.realServerBaseUrl = apiBaseUrl[:len(apiBaseUrl)-len(u.Path)]

		transport := &http.Transport{
			DialContext:           (&net.Dialer{Timeout: s.proxyConnTimeout}).DialContext,
			ResponseHeaderTimeout: s.proxySocketTimeout,
		}
		s.realServerClient = &http.Client{
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		s.grafanaClient = NewGrafanaClient(apiBaseUrl, s.username, s.password)
		s.startInitializer()
	}

	return s
}

func (s *DashboardService) newDashboardConfiguration(typ model.DashboardType, raw string) *dashboardConfiguration {
	configKey := consts.DashboardUrl
	if typ != model.DashboardMain {
		configKey = consts.DashboardUrlPrefix + strings.ToLower(string(typ))
	}
	dashboard, err := ParseDashboardData(raw)
	if err != nil {
		panic(errs.Internal("Error occurs when loading dashboard configurations from resource."))
	}
	return &dashboardConfiguration{typ: typ, configKey: configKey, raw: raw, dashboard: dashboard}
}

func (s *DashboardService) IsBuiltIn() bool {
	apiBaseUrl := kubernetes.EnvGet(consts.DashboardBaseUrlKey, "")
	promUrl := kubernetes.EnvGet(consts.DashboardDsPromUrlKey, "")
	lokiUrl := kubernetes.EnvGet(consts.DashboardDsLokiUrlKey, "")
	return strings.TrimSpace(apiBaseUrl) != "" && strings.TrimSpace(promUrl) != "" && strings.TrimSpace(lokiUrl) != ""
}

func (s *DashboardService) isBuiltIn() bool {
	return s.IsBuiltIn()
}

func (s *DashboardService) GetDashboardInfo(typ model.DashboardType) *model.DashboardInfo {
	if s.isBuiltIn() {
		return s.getBuiltInDashboardInfo(typ)
	}
	return s.getConfiguredDashboardInfo(typ)
}

func (s *DashboardService) InitializeDashboard(overwrite bool) {
	if !s.isBuiltIn() {
		panic(errs.Internal("No built-in dashboard is available."))
	}

	datasources, err := s.grafanaClient.GetDatasources()
	if err != nil {
		panic(errs.Business("Error occurs when loading datasources from Grafana."))
	}
	promUid := s.configurePrometheusDatasource(datasources)
	lokiUid := s.configureLokiDatasource(datasources)

	results, err := s.grafanaClient.Search("", dashboardSearchTypeDb, "", nil)
	if err != nil {
		panic(errs.Business("Error occurs when loading dashboard info from Grafana."))
	}
	for _, cfg := range s.dashboardConfigurations {
		datasourceUid := promUid
		if cfg.typ == model.DashboardLog {
			datasourceUid = lokiUid
		}
		s.configureDashboard(results, cfg.dashboard.getTitle(), cfg.raw, datasourceUid, overwrite)
	}
}

func (s *DashboardService) SetDashboardUrl(typ model.DashboardType, url string) {
	if strings.TrimSpace(url) == "" {
		panic(errs.Internal("url cannot be null or blank."))
	}
	if s.isBuiltIn() {
		panic(errs.Internal("Manual dashboard configuration is disabled."))
	}
	cfg := s.getDashboardConfiguration(typ)
	s.configService.SetConfig(cfg.configKey, url)
}

func (s *DashboardService) BuildConfigData(typ model.DashboardType, datasourceUid string) string {
	cfg := s.getDashboardConfiguration(typ)
	return strings.ReplaceAll(cfg.raw, dashboardDatasourceUidPlaceholder, datasourceUid)
}

func (s *DashboardService) ForwardDashboardRequest(w http.ResponseWriter, r *http.Request) {
	if !s.isBuiltIn() {
		panic(errs.Internal("Dashboard request forward function is only available for built-in dashboard."))
	}

	servletPath := r.URL.Path
	if !strings.HasPrefix(servletPath, s.apiBaseUrlPath) {
		panic(errs.Internal("Invalid dashboard request path: " + servletPath))
	}

	target := s.realServerBaseUrl + servletPath
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		panic(errs.Business("Error occurs when reading dashboard request body."))
	}
	_ = r.Body.Close()

	proxyRequest, err := http.NewRequestWithContext(r.Context(), r.Method, target, bytes.NewReader(body))
	if err != nil {
		panic(errs.Business("Error occurs when building dashboard request."))
	}
	for name, values := range r.Header {
		if ignoreRequestHeaders[strings.ToLower(name)] {
			continue
		}
		for _, v := range values {
			proxyRequest.Header.Add(name, v)
		}
	}

	proxyResponse, err := s.realServerClient.Do(proxyRequest)
	if err != nil {
		panic(errs.Business("Error occurs when forwarding dashboard request."))
	}
	defer proxyResponse.Body.Close()

	for name, values := range proxyResponse.Header {
		if ignoreResponseHeaders[strings.ToLower(name)] {
			continue
		}
		for _, v := range values {
			w.Header().Add(name, v)
		}
	}
	w.WriteHeader(proxyResponse.StatusCode)
	_, _ = io.Copy(w, proxyResponse.Body)
}

func (s *DashboardService) configurePrometheusDatasource(existed []Datasource) string {
	if uid := findDatasourceUidByUrl(existed, s.promDatasourceUrl); uid != "" {
		return uid
	}
	ds := &Datasource{
		Type:   dashboardPromDatasourceType,
		Name:   s.promDatasourceName,
		Url:    s.promDatasourceUrl,
		Access: dashboardDatasourceAccess,
	}
	result, err := s.grafanaClient.CreateDatasource(ds)
	if err != nil {
		panic(errs.Business("Error occurs when creating Prometheus datasource in Grafana."))
	}
	if result.Datasource == nil {
		panic(errs.Business("Creating data source call returns success but no datasource object. Message=" + result.Message))
	}
	return result.Datasource.Uid
}

func (s *DashboardService) configureLokiDatasource(existed []Datasource) string {
	if uid := findDatasourceUidByUrl(existed, s.lokiDatasourceUrl); uid != "" {
		return uid
	}
	ds := &Datasource{
		Type:   dashboardLokiDatasourceType,
		Name:   s.lokiDatasourceName,
		Url:    s.lokiDatasourceUrl,
		Access: dashboardDatasourceAccess,
	}
	result, err := s.grafanaClient.CreateDatasource(ds)
	if err != nil {
		panic(errs.Business("Error occurs when creating Loki datasource in Grafana."))
	}
	if result.Datasource == nil {
		panic(errs.Business("Creating data source call returns success but no datasource object. Message=" + result.Message))
	}
	return result.Datasource.Uid
}

func (s *DashboardService) configureDashboard(results []GrafanaSearchResult, title, rawConfig, datasourceUid string, overwrite bool) {
	if title == "" {
		panic(errs.Internal("No title is found in the configured dashboard."))
	}

	existedUid := ""
	for _, r := range results {
		if r.Title == title {
			existedUid = r.Uid
			break
		}
	}
	if existedUid != "" && !overwrite {
		return
	}

	dashboardData := strings.ReplaceAll(rawConfig, dashboardDatasourceUidPlaceholder, datasourceUid)
	dashboard, err := ParseDashboardData(dashboardData)
	if err != nil {
		panic(errs.Internal("Unable to parse the configured dashboard data."))
	}
	dashboard.setId(nil)
	dashboard.setUid("")
	dashboard.setVersion(nil)

	if existedUid != "" {
		existed, err := s.grafanaClient.GetDashboard(existedUid)
		if err != nil {
			panic(errs.Business("Error occurs when creating Higress dashboard in Grafana."))
		}
		if existed != nil {
			dashboard.setId(existed.getId())
			dashboard.setUid(existedUid)
			dashboard.setVersion(existed.getVersion())
		}
	}

	if dashboard.getId() == nil {
		if _, err := s.grafanaClient.CreateDashboard(dashboard); err != nil {
			panic(errs.Business("Error occurs when creating Higress dashboard in Grafana."))
		}
	} else {
		if _, err := s.grafanaClient.UpdateDashboard(dashboard); err != nil {
			panic(errs.Business("Error occurs when creating Higress dashboard in Grafana."))
		}
	}
}

func (s *DashboardService) getBuiltInDashboardInfo(typ model.DashboardType) *model.DashboardInfo {
	cfg := s.getDashboardConfiguration(typ)
	results, err := s.grafanaClient.Search("", dashboardSearchTypeDb, "", nil)
	if err != nil {
		panic(errs.Business("Error occurs when loading dashboard info from Grafana."))
	}
	if len(results) == 0 {
		return &model.DashboardInfo{BuiltIn: boolPtr(true)}
	}
	expectedTitle := cfg.dashboard.getTitle()
	if expectedTitle == "" {
		panic(errs.Internal("No title is found in the configured dashboard."))
	}
	for _, r := range results {
		if r.Title == expectedTitle {
			return &model.DashboardInfo{BuiltIn: boolPtr(true), Uid: strPtr(r.Uid), Url: strPtr(r.Url)}
		}
	}
	return nil
}

func (s *DashboardService) getConfiguredDashboardInfo(typ model.DashboardType) *model.DashboardInfo {
	cfg := s.getDashboardConfiguration(typ)
	url := s.configService.GetString(cfg.configKey)
	return &model.DashboardInfo{BuiltIn: boolPtr(false), Url: strPtr(url)}
}

func (s *DashboardService) getDashboardConfiguration(typ model.DashboardType) *dashboardConfiguration {
	cfg := s.dashboardConfigurations[typ]
	if cfg == nil {
		panic(errs.Internal("Invalid dashboard type: " + string(typ)))
	}
	return cfg
}

func (s *DashboardService) startInitializer() {
	go func() {
		for {
			done := func() (ok bool) {
				defer func() {
					if recover() != nil {
						ok = false
					}
				}()
				s.InitializeDashboard(s.overwriteWhenStartUp)
				return true
			}()
			if done {
				return
			}
			time.Sleep(dashboardInitializeRetryInterval)
		}
	}()
}

func findDatasourceUidByUrl(existed []Datasource, url string) string {
	for _, ds := range existed {
		if ds.Url == url {
			return ds.Uid
		}
	}
	return ""
}

func boolPtr(b bool) *bool {
	return &b
}
