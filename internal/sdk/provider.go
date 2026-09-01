package sdk

import (
	k8s "console/internal/kubernetes"
)

// Provider 组装所有服务，对应 Java 的 HigressServiceProviderImpl
type Provider struct {
	Client                    *k8s.Client
	Converter                 *Converter
	DomainService             *DomainService
	RouteService              *RouteService
	ServiceService            *ServiceService
	ServiceSourceService      *ServiceSourceService
	ProxyServerService        *ProxyServerService
	TlsCertificateService     *TlsCertificateService
	WasmPluginService         WasmPluginService
	WasmPluginInstanceService *WasmPluginInstanceService
	ConsumerService           *ConsumerService
	AiRouteService            *AiRouteService
	LlmProviderService        *LlmProviderService
}

// NewProvider 创建 Provider 并完成服务依赖注入
func NewProvider(client *k8s.Client) *Provider {
	converter := NewConverter(client)
	serviceService := NewServiceService(client)
	serviceSourceService := NewServiceSourceService(client, converter)
	proxyServerService := NewProxyServerService(client, converter)
	tlsCertificateService := NewTlsCertificateService(client, converter)
	wasmPluginService := NewWasmPluginService(client, converter)
	wasmPluginInstanceService := NewWasmPluginInstanceService(wasmPluginService, client, converter)
	consumerService := NewConsumerService(wasmPluginInstanceService)
	routeService := NewRouteService(client, converter, wasmPluginInstanceService, consumerService)
	domainService := NewDomainService(client, converter, routeService, wasmPluginInstanceService)
	llmProviderService := NewLlmProviderService(serviceSourceService, wasmPluginInstanceService)
	aiRouteService := NewAiRouteService(converter, client, routeService, llmProviderService, wasmPluginInstanceService)
	llmProviderService.SetAiRouteService(aiRouteService)

	return &Provider{
		Client:                    client,
		Converter:                 converter,
		DomainService:             domainService,
		RouteService:              routeService,
		ServiceService:            serviceService,
		ServiceSourceService:      serviceSourceService,
		ProxyServerService:        proxyServerService,
		TlsCertificateService:     tlsCertificateService,
		WasmPluginService:         wasmPluginService,
		WasmPluginInstanceService: wasmPluginInstanceService,
		ConsumerService:           consumerService,
		AiRouteService:            aiRouteService,
		LlmProviderService:        llmProviderService,
	}
}
