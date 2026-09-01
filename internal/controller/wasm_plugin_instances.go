package controller

import (
	"github.com/gogf/gf/v2/net/ghttp"

	"console/internal/errs"
	"console/internal/model"
)

// pluginInstanceTargetPtr 返回 scope 对应的 target 指针；global scope 的 target 为 nil。
func pluginInstanceTargetPtr(scope model.WasmPluginInstanceScope, target string) *string {
	if scope == model.ScopeGlobal {
		return nil
	}
	return strPtr(target)
}

// validateWasmPluginInstanceTarget 对应 validateDomainName/validateRouteName/validateServiceName。
func (c *Controller) validateWasmPluginInstanceTarget(scope model.WasmPluginInstanceScope, target string) {
	switch scope {
	case model.ScopeDomain:
		if c.Domain.Query(target) == nil {
			panic(errs.Validation("Unknown domain: " + target))
		}
	case model.ScopeRoute:
		if c.Route.Query(target) == nil {
			panic(errs.Validation("Unknown route: " + target))
		}
	case model.ScopeService:
		services := c.Service.List(nil)
		found := false
		for i := range services.Data {
			if derefStr(services.Data[i].Name) == target {
				found = true
				break
			}
		}
		if !found {
			panic(errs.Validation("Unknown service: " + target))
		}
	}
}

// listWasmPluginInstances 对应 listInstances。
func (c *Controller) listWasmPluginInstances(r *ghttp.Request, scope model.WasmPluginInstanceScope, target string) {
	instances := c.WasmInstance.ListByScope(scope, target)
	filtered := make([]*model.WasmPluginInstance, 0, len(instances))
	for _, instance := range instances {
		if !instance.IsInternal() {
			filtered = append(filtered, instance)
		}
	}
	writePaginated(r, model.CreateFromFullList(filtered, nil))
}

// queryWasmPluginInstance 对应 queryInstance。
func (c *Controller) queryWasmPluginInstance(r *ghttp.Request, scope model.WasmPluginInstanceScope, target string) {
	name := pathParam(r, "name")
	instance := c.WasmInstance.Query(scope, target, name, boolPtr(false))
	if instance == nil {
		instance = &model.WasmPluginInstance{
			PluginName: strPtr(name),
			Internal:   boolPtr(false),
			Enabled:    boolPtr(false),
		}
		instance.SetTarget(scope, pluginInstanceTargetPtr(scope, target))
	}
	writeGet(r, instance)
}

// addOrUpdateWasmPluginInstance 对应 addOrUpdateInstance。
func (c *Controller) addOrUpdateWasmPluginInstance(r *ghttp.Request, scope model.WasmPluginInstanceScope, target string) {
	name := pathParam(r, "name")
	var instance model.WasmPluginInstance
	parseBody(r, &instance)
	if isEmpty(instance.PluginName) {
		instance.PluginName = strPtr(name)
	} else if derefStr(instance.PluginName) != name {
		panic(errs.Validation("Plugin name in the URL doesn't match the one in the body."))
	}
	if instance.IsInternal() {
		panic(errs.Validation("Updating an internal Wasm plugin instance is not allowed."))
	}
	if c.WasmPlugin.Query(name, "") == nil {
		panic(errs.Validation("Unsupported plugin: " + name))
	}
	instance.SetTarget(scope, pluginInstanceTargetPtr(scope, target))
	writeUpdated(r, c.WasmInstance.AddOrUpdate(&instance))
}

// deleteWasmPluginInstance 对应 deleteInstance。
func (c *Controller) deleteWasmPluginInstance(r *ghttp.Request, scope model.WasmPluginInstanceScope, target string) {
	c.WasmInstance.Delete(scope, target, pathParam(r, "name"), nil)
	writeNoContent(r)
}

// ---- global ----

func (c *Controller) listGlobalWasmPluginInstances(r *ghttp.Request) {
	c.listWasmPluginInstances(r, model.ScopeGlobal, "")
}

func (c *Controller) queryGlobalWasmPluginInstance(r *ghttp.Request) {
	c.queryWasmPluginInstance(r, model.ScopeGlobal, "")
}

func (c *Controller) addOrUpdateGlobalWasmPluginInstance(r *ghttp.Request) {
	c.addOrUpdateWasmPluginInstance(r, model.ScopeGlobal, "")
}

func (c *Controller) deleteGlobalWasmPluginInstance(r *ghttp.Request) {
	c.deleteWasmPluginInstance(r, model.ScopeGlobal, "")
}

// ---- domain ----

func (c *Controller) listDomainWasmPluginInstances(r *ghttp.Request) {
	target := pathParam(r, "domainName")
	c.validateWasmPluginInstanceTarget(model.ScopeDomain, target)
	c.listWasmPluginInstances(r, model.ScopeDomain, target)
}

func (c *Controller) queryDomainWasmPluginInstance(r *ghttp.Request) {
	target := pathParam(r, "domainName")
	c.validateWasmPluginInstanceTarget(model.ScopeDomain, target)
	c.queryWasmPluginInstance(r, model.ScopeDomain, target)
}

func (c *Controller) addOrUpdateDomainWasmPluginInstance(r *ghttp.Request) {
	target := pathParam(r, "domainName")
	c.validateWasmPluginInstanceTarget(model.ScopeDomain, target)
	c.addOrUpdateWasmPluginInstance(r, model.ScopeDomain, target)
}

func (c *Controller) deleteDomainWasmPluginInstance(r *ghttp.Request) {
	target := pathParam(r, "domainName")
	c.validateWasmPluginInstanceTarget(model.ScopeDomain, target)
	c.deleteWasmPluginInstance(r, model.ScopeDomain, target)
}

// ---- route ----

func (c *Controller) listRouteWasmPluginInstances(r *ghttp.Request) {
	target := pathParam(r, "routeName")
	c.validateWasmPluginInstanceTarget(model.ScopeRoute, target)
	c.listWasmPluginInstances(r, model.ScopeRoute, target)
}

func (c *Controller) queryRouteWasmPluginInstance(r *ghttp.Request) {
	target := pathParam(r, "routeName")
	c.validateWasmPluginInstanceTarget(model.ScopeRoute, target)
	c.queryWasmPluginInstance(r, model.ScopeRoute, target)
}

func (c *Controller) addOrUpdateRouteWasmPluginInstance(r *ghttp.Request) {
	target := pathParam(r, "routeName")
	c.validateWasmPluginInstanceTarget(model.ScopeRoute, target)
	c.addOrUpdateWasmPluginInstance(r, model.ScopeRoute, target)
}

func (c *Controller) deleteRouteWasmPluginInstance(r *ghttp.Request) {
	target := pathParam(r, "routeName")
	c.validateWasmPluginInstanceTarget(model.ScopeRoute, target)
	c.deleteWasmPluginInstance(r, model.ScopeRoute, target)
}

// ---- service ----

func (c *Controller) listServiceWasmPluginInstances(r *ghttp.Request) {
	target := pathParam(r, "serviceName")
	c.validateWasmPluginInstanceTarget(model.ScopeService, target)
	c.listWasmPluginInstances(r, model.ScopeService, target)
}

func (c *Controller) queryServiceWasmPluginInstance(r *ghttp.Request) {
	target := pathParam(r, "serviceName")
	c.validateWasmPluginInstanceTarget(model.ScopeService, target)
	c.queryWasmPluginInstance(r, model.ScopeService, target)
}

func (c *Controller) addOrUpdateServiceWasmPluginInstance(r *ghttp.Request) {
	target := pathParam(r, "serviceName")
	c.validateWasmPluginInstanceTarget(model.ScopeService, target)
	c.addOrUpdateWasmPluginInstance(r, model.ScopeService, target)
}

func (c *Controller) deleteServiceWasmPluginInstance(r *ghttp.Request) {
	target := pathParam(r, "serviceName")
	c.validateWasmPluginInstanceTarget(model.ScopeService, target)
	c.deleteWasmPluginInstance(r, model.ScopeService, target)
}
