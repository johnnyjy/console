package main

import (
	"os"
	"strconv"

	"github.com/gogf/gf/v2/frame/g"

	"console/internal/controller"
	"console/internal/kubernetes"
	"console/internal/sdk"
	"console/internal/service"
)

func main() {
	cfg := kubernetes.NewConfig()
	client, err := kubernetes.NewClient(cfg)
	if err != nil {
		panic(err)
	}

	provider := sdk.NewProvider(client)

	configService := service.NewConfigService(client)
	sessionService := service.NewSessionService(client, configService)
	dashboardService := service.NewDashboardService(configService)
	systemService := service.NewSystemService(
		client,
		configService,
		sessionService,
		dashboardService,
		provider.TlsCertificateService,
		provider.DomainService,
		provider.RouteService,
	)
	aiProxyService := service.NewAiProxyService(client)

	c := controller.NewController(
		configService,
		sessionService,
		systemService,
		dashboardService,
		aiProxyService,
		provider.DomainService,
		provider.RouteService,
		provider.ServiceService,
		provider.ServiceSourceService,
		provider.TlsCertificateService,
		provider.WasmPluginService,
		provider.WasmPluginInstanceService,
		provider.ConsumerService,
		provider.ProxyServerService,
		provider.AiRouteService,
		provider.LlmProviderService,
	)

	s := g.Server()
	s.SetPort(serverPort())
	controller.RegisterRoutes(s, c)
	s.Run()
}

// serverPort 返回监听端口，默认 8080（与 Java Spring Boot 一致），
// 可通过 SERVER_PORT 环境变量覆盖。
func serverPort() int {
	if v := os.Getenv("SERVER_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 8080
}
