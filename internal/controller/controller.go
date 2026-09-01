package controller

import (
	"console/internal/sdk"
	"console/internal/service"
)

// Controller 汇聚所有控制器所需的 service，对应 Java 的各 Controller。
type Controller struct {
	Config    *service.ConfigService
	Session   *service.SessionService
	System    *service.SystemService
	Dashboard *service.DashboardService
	AiProxy   *service.AiProxyService

	Domain        *sdk.DomainService
	Route         *sdk.RouteService
	Service       *sdk.ServiceService
	ServiceSource *sdk.ServiceSourceService
	Tls           *sdk.TlsCertificateService
	WasmPlugin    sdk.WasmPluginService
	WasmInstance  *sdk.WasmPluginInstanceService
	Consumer      *sdk.ConsumerService
	ProxyServer   *sdk.ProxyServerService
	AiRoute       *sdk.AiRouteService
	LlmProvider   *sdk.LlmProviderService
}

// NewController 创建 Controller。
func NewController(
	config *service.ConfigService,
	session *service.SessionService,
	system *service.SystemService,
	dashboard *service.DashboardService,
	aiProxy *service.AiProxyService,
	domain *sdk.DomainService,
	route *sdk.RouteService,
	serviceService *sdk.ServiceService,
	serviceSource *sdk.ServiceSourceService,
	tls *sdk.TlsCertificateService,
	wasmPlugin sdk.WasmPluginService,
	wasmInstance *sdk.WasmPluginInstanceService,
	consumer *sdk.ConsumerService,
	proxyServer *sdk.ProxyServerService,
	aiRoute *sdk.AiRouteService,
	llmProvider *sdk.LlmProviderService,
) *Controller {
	return &Controller{
		Config:        config,
		Session:       session,
		System:        system,
		Dashboard:     dashboard,
		AiProxy:       aiProxy,
		Domain:        domain,
		Route:         route,
		Service:       serviceService,
		ServiceSource: serviceSource,
		Tls:           tls,
		WasmPlugin:    wasmPlugin,
		WasmInstance:  wasmInstance,
		Consumer:      consumer,
		ProxyServer:   proxyServer,
		AiRoute:       aiRoute,
		LlmProvider:   llmProvider,
	}
}
