package controller

import (
	"github.com/gogf/gf/v2/net/ghttp"

	"console/internal/errs"
	"console/internal/model"
)

// listWasmPlugins 对应 WasmPluginsController.list
func (c *Controller) listWasmPlugins(r *ghttp.Request) {
	writePaginated(r, c.WasmPlugin.List(parseWasmPluginPageQuery(r)))
}

// queryWasmPlugin 对应 WasmPluginsController.query
func (c *Controller) queryWasmPlugin(r *ghttp.Request) {
	writeGet(r, c.WasmPlugin.Query(pathParam(r, "name"), queryString(r, "lang")))
}

// addWasmPlugin 对应 WasmPluginsController.add
func (c *Controller) addWasmPlugin(r *ghttp.Request) {
	var plugin model.WasmPlugin
	parseBody(r, &plugin)
	validateWasmPlugin(&plugin)
	writeCreated(r, c.WasmPlugin.AddCustom(&plugin))
}

// updateWasmPlugin 对应 WasmPluginsController.update
func (c *Controller) updateWasmPlugin(r *ghttp.Request) {
	name := pathParam(r, "name")
	var plugin model.WasmPlugin
	parseBody(r, &plugin)
	if isEmpty(plugin.Name) {
		plugin.Name = strPtr(name)
	} else if derefStr(plugin.Name) != name {
		panic(errs.Validation("Plugin name in the URL doesn't match the one in the body."))
	}
	validateWasmPlugin(&plugin)
	if boolValue(plugin.BuiltIn) {
		writeUpdated(r, c.WasmPlugin.UpdateBuiltIn(&plugin))
	} else {
		writeUpdated(r, c.WasmPlugin.UpdateCustom(&plugin))
	}
}

// deleteWasmPlugin 对应 WasmPluginsController.delete
func (c *Controller) deleteWasmPlugin(r *ghttp.Request) {
	c.WasmPlugin.DeleteCustom(pathParam(r, "name"))
	writeNoContent(r)
}

// queryWasmPluginConfig 对应 WasmPluginsController.queryConfig
func (c *Controller) queryWasmPluginConfig(r *ghttp.Request) {
	writeGet(r, c.WasmPlugin.QueryConfig(pathParam(r, "name"), queryString(r, "lang")))
}

// queryWasmPluginReadme 对应 WasmPluginsController.queryReadme
func (c *Controller) queryWasmPluginReadme(r *ghttp.Request) {
	writeGet(r, c.WasmPlugin.QueryReadme(pathParam(r, "name"), queryString(r, "lang")))
}
