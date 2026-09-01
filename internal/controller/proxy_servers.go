package controller

import (
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"

	"console/internal/consts"
	"console/internal/errs"
	"console/internal/model"
)

// listProxyServers 对应 ProxyServerController.list
func (c *Controller) listProxyServers(r *ghttp.Request) {
	writePaginated(r, c.ProxyServer.List(parseCommonPageQuery(r)))
}

// addProxyServer 对应 ProxyServerController.add
func (c *Controller) addProxyServer(r *ghttp.Request) {
	var proxyServer model.ProxyServer
	parseBody(r, &proxyServer)
	if !proxyServerValid(&proxyServer) {
		panic(errs.Validation("proxyServer body is not valid."))
	}
	if strings.HasSuffix(derefStr(proxyServer.Name), consts.InternalResourceNameSuffix) {
		panic(errs.Validation("Adding an internal proxy server is not allowed."))
	}
	writeCreated(r, c.ProxyServer.Add(&proxyServer))
}

// addOrUpdateProxyServer 对应 ProxyServerController.addOrUpdate
func (c *Controller) addOrUpdateProxyServer(r *ghttp.Request) {
	name := pathParam(r, "name")
	var proxyServer model.ProxyServer
	parseBody(r, &proxyServer)
	proxyServer.Name = strPtr(name)
	if !proxyServerValid(&proxyServer) {
		panic(errs.Validation("proxyServer body is not valid."))
	}
	if strings.HasSuffix(derefStr(proxyServer.Name), consts.InternalResourceNameSuffix) {
		panic(errs.Validation("Updating an internal proxy server is not allowed."))
	}
	writeUpdated(r, c.ProxyServer.AddOrUpdate(&proxyServer))
}

// deleteProxyServer 对应 ProxyServerController.delete
func (c *Controller) deleteProxyServer(r *ghttp.Request) {
	name := pathParam(r, "name")
	if strings.HasSuffix(name, consts.InternalResourceNameSuffix) {
		panic(errs.Validation("Deleting an internal proxy server is not allowed."))
	}
	c.ProxyServer.Delete(name)
	writeNoContent(r)
}

// queryProxyServer 对应 ProxyServerController.query
func (c *Controller) queryProxyServer(r *ghttp.Request) {
	writeGet(r, c.ProxyServer.Query(pathParam(r, "name")))
}
