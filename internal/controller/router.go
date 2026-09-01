package controller

import (
	"github.com/gogf/gf/v2/net/ghttp"
)

// RegisterRoutes 注册全部 HTTP 路由，对应各 Java Controller 的 @RequestMapping。
func RegisterRoutes(s *ghttp.Server, c *Controller) {
	s.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(RecoverMiddleware, AuthMiddleware(c))

		// session（匿名）
		group.POST("/session/login", c.login)
		group.GET("/session/logout", c.logout)

		// healthz（匿名）
		group.GET("/healthz/ready", c.healthzReady)

		// landing（匿名）
		group.ALL("/landing", c.landingIndex)

		// system
		group.POST("/system/init", c.initSystem)
		group.GET("/system/info", c.getSystemInfo)
		group.GET("/system/config", c.getSystemConfigs)
		group.GET("/system/higress-config", c.getHigressConfig)
		group.PUT("/system/higress-config", c.updateHigressConfig)

		// user
		group.GET("/user/info", c.getUserInfo)
		group.POST("/user/changePassword", c.changePassword)

		// dashboard
		group.GET("/dashboard/init", c.initDashboard)
		group.GET("/dashboard/info", c.getDashboardInfo)
		group.PUT("/dashboard/info", c.setDashboardUrl)
		group.GET("/dashboard/configData", c.getDashboardConfigData)

		// grafana 转发
		group.ALL("/grafana", c.forwardGrafana)
		group.ALL("/grafana/*any", c.forwardGrafana)

		// aiproxy 转发
		group.ALL("/aiproxy", c.forwardAiProxy)
		group.ALL("/aiproxy/*any", c.forwardAiProxy)

		// routes
		group.GET("/v1/routes", c.listRoutes)
		group.POST("/v1/routes", c.addRoute)
		group.GET("/v1/routes/:name", c.queryRoute)
		group.PUT("/v1/routes/:name", c.updateRoute)
		group.DELETE("/v1/routes/:name", c.deleteRoute)

		// domains
		group.GET("/v1/domains", c.listDomains)
		group.POST("/v1/domains", c.addDomain)
		group.GET("/v1/domains/:name", c.queryDomain)
		group.PUT("/v1/domains/:name", c.updateDomain)
		group.DELETE("/v1/domains/:name", c.deleteDomain)
		group.GET("/v1/domains/:name/routes", c.queryDomainRoutes)

		// services
		group.GET("/v1/services", c.listServices)

		// service sources
		group.GET("/v1/service-sources", c.listServiceSources)
		group.POST("/v1/service-sources", c.addServiceSource)
		group.GET("/v1/service-sources/:name", c.queryServiceSource)
		group.PUT("/v1/service-sources/:name", c.addOrUpdateServiceSource)
		group.DELETE("/v1/service-sources/:name", c.deleteServiceSource)

		// tls certificates
		group.GET("/v1/tls-certificates", c.listTlsCertificates)
		group.POST("/v1/tls-certificates", c.addTlsCertificate)
		group.GET("/v1/tls-certificates/:name", c.queryTlsCertificate)
		group.PUT("/v1/tls-certificates/:name", c.updateTlsCertificate)
		group.DELETE("/v1/tls-certificates/:name", c.deleteTlsCertificate)

		// consumers
		group.GET("/v1/consumers", c.listConsumers)
		group.POST("/v1/consumers", c.addConsumer)
		group.GET("/v1/consumers/:name", c.queryConsumer)
		group.PUT("/v1/consumers/:name", c.updateConsumer)
		group.DELETE("/v1/consumers/:name", c.deleteConsumer)

		// proxy servers
		group.GET("/v1/proxy-servers", c.listProxyServers)
		group.POST("/v1/proxy-servers", c.addProxyServer)
		group.GET("/v1/proxy-servers/:name", c.queryProxyServer)
		group.PUT("/v1/proxy-servers/:name", c.addOrUpdateProxyServer)
		group.DELETE("/v1/proxy-servers/:name", c.deleteProxyServer)

		// wasm plugins
		group.GET("/v1/wasm-plugins", c.listWasmPlugins)
		group.POST("/v1/wasm-plugins", c.addWasmPlugin)
		group.GET("/v1/wasm-plugins/:name", c.queryWasmPlugin)
		group.PUT("/v1/wasm-plugins/:name", c.updateWasmPlugin)
		group.DELETE("/v1/wasm-plugins/:name", c.deleteWasmPlugin)
		group.GET("/v1/wasm-plugins/:name/config", c.queryWasmPluginConfig)
		group.GET("/v1/wasm-plugins/:name/readme", c.queryWasmPluginReadme)

		// wasm plugin instances - global
		group.GET("/v1/global/plugin-instances", c.listGlobalWasmPluginInstances)
		group.GET("/v1/global/plugin-instances/:name", c.queryGlobalWasmPluginInstance)
		group.PUT("/v1/global/plugin-instances/:name", c.addOrUpdateGlobalWasmPluginInstance)
		group.DELETE("/v1/global/plugin-instances/:name", c.deleteGlobalWasmPluginInstance)

		// wasm plugin instances - domain
		group.GET("/v1/domains/:domainName/plugin-instances", c.listDomainWasmPluginInstances)
		group.GET("/v1/domains/:domainName/plugin-instances/:name", c.queryDomainWasmPluginInstance)
		group.PUT("/v1/domains/:domainName/plugin-instances/:name", c.addOrUpdateDomainWasmPluginInstance)
		group.DELETE("/v1/domains/:domainName/plugin-instances/:name", c.deleteDomainWasmPluginInstance)

		// wasm plugin instances - route
		group.GET("/v1/routes/:routeName/plugin-instances", c.listRouteWasmPluginInstances)
		group.GET("/v1/routes/:routeName/plugin-instances/:name", c.queryRouteWasmPluginInstance)
		group.PUT("/v1/routes/:routeName/plugin-instances/:name", c.addOrUpdateRouteWasmPluginInstance)
		group.DELETE("/v1/routes/:routeName/plugin-instances/:name", c.deleteRouteWasmPluginInstance)

		// wasm plugin instances - service
		group.GET("/v1/services/:serviceName/plugin-instances", c.listServiceWasmPluginInstances)
		group.GET("/v1/services/:serviceName/plugin-instances/:name", c.queryServiceWasmPluginInstance)
		group.PUT("/v1/services/:serviceName/plugin-instances/:name", c.addOrUpdateServiceWasmPluginInstance)
		group.DELETE("/v1/services/:serviceName/plugin-instances/:name", c.deleteServiceWasmPluginInstance)

		// ai routes
		group.GET("/v1/ai/routes", c.listAiRoutes)
		group.POST("/v1/ai/routes", c.addAiRoute)
		group.GET("/v1/ai/routes/:name", c.queryAiRoute)
		group.PUT("/v1/ai/routes/:name", c.updateAiRoute)
		group.DELETE("/v1/ai/routes/:name", c.deleteAiRoute)

		// llm providers
		group.GET("/v1/ai/providers", c.listLlmProviders)
		group.POST("/v1/ai/providers", c.addLlmProvider)
		group.GET("/v1/ai/providers/:name", c.queryLlmProvider)
		group.PUT("/v1/ai/providers/:name", c.updateLlmProvider)
		group.DELETE("/v1/ai/providers/:name", c.deleteLlmProvider)
	})
}
