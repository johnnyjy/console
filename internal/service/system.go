package service

import (
	"context"
	"crypto/x509"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"console/internal/consts"
	"console/internal/errs"
	"console/internal/kubernetes"
	"console/internal/model"
	"console/internal/util"
)

const (
	defaultTlsCertificateName     = "default"
	defaultTlsCertificateHost     = "higress-gateway"
	defaultTlsCertificateDuration = 365 * 24 * time.Hour
	defaultRouteName              = "default"
	unknownVersion                = "unknown"
)

var requiredHigressConfigKeys = []string{"higress", "mesh", "meshNetworks"}

// 以下接口对应 SDK 服务中 SystemService 用到的 add 方法，
// 由具体 SDK 服务（todo 8）实现并注入。
type tlsCertService interface {
	Add(cert *model.TlsCertificate) *model.TlsCertificate
}

type domainService interface {
	Add(domain *model.Domain) *model.Domain
}

type routeService interface {
	Add(route *model.Route) *model.Route
}

// SystemService 对应 Java 的 SystemServiceImpl
type SystemService struct {
	client    *kubernetes.Client
	config    *ConfigService
	session   *SessionService
	dashboard *DashboardService

	tls    tlsCertService
	domain domainService
	route  routeService

	fullVersion        string
	capabilities       []string
	consoleServiceHost string
	consoleServicePort int
}

// NewSystemService 对应 SystemServiceImpl 构造 + @PostConstruct initInternalState
func NewSystemService(client *kubernetes.Client, config *ConfigService, session *SessionService,
	dashboard *DashboardService, tls tlsCertService, domain domainService, route routeService) *SystemService {
	s := &SystemService{
		client:             client,
		config:             config,
		session:            session,
		dashboard:          dashboard,
		tls:                tls,
		domain:             domain,
		route:              route,
		consoleServiceHost: kubernetes.EnvGet(consts.ConsoleServiceHostKey, consts.DefaultConsoleServiceHost),
		consoleServicePort: kubernetes.EnvInt(consts.ConsoleServicePortKey, consts.DefaultConsoleServicePort),
	}
	s.initInternalState()
	return s
}

func (s *SystemService) initInternalState() {
	version := kubernetes.EnvGet(consts.VersionKey, "")
	s.fullVersion = version
	if version == "" {
		s.fullVersion = unknownVersion
	}
	devBuild := kubernetes.EnvGet(consts.DevBuildKey, strconv.FormatBool(consts.DevBuildDefault)) == "true"
	if devBuild {
		s.fullVersion += "-dev-" + unknownVersion
	}

	if s.client.IngressV1Supported {
		s.capabilities = append(s.capabilities, consts.CapabilityConfigIngressV1)
	}

	s.config.SetConfigs(map[string]interface{}{
		consts.SystemInitialized: s.session.IsAdminInitialized(),
		consts.DashboardBuiltin:  s.dashboard.isBuiltIn(),
	})

	s.initDefaultRoutes()
}

// InitSystem 对应 initSystem
func (s *SystemService) InitSystem(adminUser *model.User, configs map[string]interface{}) {
	if s.config.GetBooleanDefault(consts.SystemInitialized, false) {
		panic(errs.Internal("System already initialized."))
	}
	s.initAdminUser(adminUser)
	s.initConfigs(configs)
}

func (s *SystemService) initAdminUser(adminUser *model.User) {
	if !s.session.IsAdminInitialized() {
		s.session.InitializeAdmin(adminUser)
	}
}

func (s *SystemService) initConfigs(configs map[string]interface{}) {
	full := map[string]interface{}{}
	for k, v := range configs {
		full[k] = v
	}
	full[consts.SystemInitialized] = true
	s.config.SetConfigs(full)
}

func (s *SystemService) initDefaultRoutes() {
	if s.config.GetBooleanDefault(consts.DefaultRouteInitialized, false) {
		return
	}

	ingresses, err := s.client.ListAllIngresses(context.Background())
	if err != nil {
		return
	}
	if len(ingresses) > 0 {
		s.config.SetConfig(consts.DefaultRouteInitialized, "true")
		return
	}

	// 初始化默认 TLS 证书与域名，异常时忽略（与 Java 一致）。
	func() {
		defer func() { _ = recover() }()

		keyPair, err := util.GenerateRsaKeyPair(4096)
		if err != nil {
			panic(errs.Business("Error occurs when generating RSA key pair."))
		}
		certDER, err := util.GenerateSelfSignedCertificate(keyPair, defaultTlsCertificateHost, defaultTlsCertificateDuration)
		if err != nil {
			panic(errs.Business("Error occurs when generating self-signed certificate."))
		}
		keyDER := x509.MarshalPKCS1PrivateKey(keyPair)

		cert := &model.TlsCertificate{
			Name:    strPtr(defaultTlsCertificateName),
			Domains: []string{defaultTlsCertificateHost},
			Key:     strPtr(util.ToPem(util.RsaPrivateKeyPemType, keyDER)),
			Cert:    strPtr(util.ToPem(util.CertificatePemType, certDER)),
		}
		s.tls.Add(cert)

		domain := &model.Domain{
			Name:           strPtr(consts.DefaultDomain),
			EnableHttps:    strPtr(model.DomainHttpsOn),
			CertIdentifier: strPtr(defaultTlsCertificateName),
		}
		s.domain.Add(domain)
	}()

	// 初始化默认路由。
	func() {
		defer func() { _ = recover() }()

		route := &model.Route{
			Name: strPtr(defaultRouteName),
			Path: &model.RoutePredicate{
				MatchType:  strPtr(string(model.RoutePredicateEqual)),
				MatchValue: strPtr("/"),
			},
			Services: []model.UpstreamService{{
				Name: strPtr(s.consoleServiceHost),
				Port: intPtr(s.consoleServicePort),
			}},
			Rewrite: &model.RewriteConfig{Enabled: boolPtr(true), Path: strPtr("/landing")},
		}
		s.route.Add(route)

		s.config.SetConfig(consts.DefaultRouteInitialized, "true")
	}()
}

// GetSystemInfo 对应 getSystemInfo
func (s *SystemService) GetSystemInfo() *model.SystemInfo {
	return &model.SystemInfo{Version: strPtr(s.fullVersion), Capabilities: s.capabilities}
}

// GetHigressConfig 对应 getHigressConfig
func (s *SystemService) GetHigressConfig() string {
	cm, err := s.client.ReadConfigMap(context.Background(), consts.HigressConfigName)
	if err != nil {
		panic(errs.Business("Failed to load " + consts.HigressConfigName + " config map."))
	}
	s.cleanUpConfigMap(cm)
	out, err := yaml.Marshal(cm)
	if err != nil {
		panic(errs.Business("Failed to serialize " + consts.HigressConfigName + " ConfigMap."))
	}
	return string(out)
}

// SetHigressConfig 对应 setHigressConfig
func (s *SystemService) SetHigressConfig(config string) string {
	var newCm corev1.ConfigMap
	if err := yaml.Unmarshal([]byte(config), &newCm); err != nil {
		panic(errs.Business("Failed to parse " + consts.HigressConfigName + " ConfigMap YAML."))
	}
	s.validateConfigMap(&newCm)

	currentCm, err := s.client.ReadConfigMap(context.Background(), consts.HigressConfigName)
	if err != nil {
		panic(errs.Business("Failed to load " + consts.HigressConfigName + " config map."))
	}

	resourceVersion := newCm.ResourceVersion
	newCm.ObjectMeta = currentCm.ObjectMeta
	newCm.ResourceVersion = resourceVersion

	updated, err := s.client.ReplaceConfigMap(context.Background(), &newCm)
	if err != nil {
		if apierrors.IsConflict(err) {
			panic(errs.Conflict("ConfigMap conflict."))
		}
		panic(errs.Business("Error occurs when replacing the " + consts.HigressConfigName + " ConfigMap."))
	}

	s.cleanUpConfigMap(updated)
	out, err := yaml.Marshal(updated)
	if err != nil {
		panic(errs.Business("Failed to serialize " + consts.HigressConfigName + " ConfigMap."))
	}
	return string(out)
}

func (s *SystemService) validateConfigMap(cm *corev1.ConfigMap) {
	if cm.Name == "" {
		panic(errs.Business("ConfigMap metadata is missing."))
	}
	if cm.ResourceVersion == "" {
		panic(errs.Business("ConfigMap resourceVersion is missing."))
	}
	if len(cm.Data) == 0 {
		panic(errs.Validation("ConfigMap data is empty."))
	}
	for _, key := range requiredHigressConfigKeys {
		value, ok := cm.Data[key]
		if !ok {
			panic(errs.Validation("ConfigMap data must contain all required keys: " +
				joinStrings(requiredHigressConfigKeys, ", ")))
		}
		if trimSpace(value) == "" {
			panic(errs.Validation("ConfigMap data key " + key + " has an empty value."))
		}
		var parsed map[string]interface{}
		if err := yaml.Unmarshal([]byte(value), &parsed); err != nil {
			panic(errs.Validation("Invalid YAML data for key " + key + ": " + value))
		}
	}
}

func (s *SystemService) cleanUpConfigMap(cm *corev1.ConfigMap) {
	cm.Labels = nil
	cm.Annotations = nil
	cm.CreationTimestamp = metav1.Time{}
	cm.UID = ""
	cm.ManagedFields = nil
}

func joinStrings(items []string, sep string) string {
	out := ""
	for i, item := range items {
		if i > 0 {
			out += sep
		}
		out += item
	}
	return out
}

func intPtr(v int) *int {
	return &v
}
