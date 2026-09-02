package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/gogf/gf/v2/frame/g"

	"console/internal/controller"
	"console/internal/kubernetes"
	"console/internal/sdk"
	"console/internal/service"
)

// version 编译期注入的版本号，可通过 -ldflags "-X main.version=v1.2.3" 覆盖。
var version = "unknown"

func main() {
	showVersion := false
	flag.BoolVar(&showVersion, "v", false, "显示版本号并退出")
	flag.BoolVar(&showVersion, "version", false, "显示版本号并退出")
	staticDir := flag.String("static-dir", "", "前端静态资源目录（默认与可执行文件同目录）")
	flag.Parse()

	if showVersion {
		fmt.Println(version)
		return
	}

	controller.SetStaticDir(*staticDir)

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
