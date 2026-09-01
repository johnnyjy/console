package controller

import (
	"github.com/gogf/gf/v2/net/ghttp"

	"console/internal/errs"
	"console/internal/model"
)

// listAiRoutes 对应 AiRoutesController.list
func (c *Controller) listAiRoutes(r *ghttp.Request) {
	writePaginated(r, c.AiRoute.List(parseCommonPageQuery(r)))
}

// addAiRoute 对应 AiRoutesController.add
func (c *Controller) addAiRoute(r *ghttp.Request) {
	var route model.AiRoute
	parseBody(r, &route)
	validateAiRoute(&route)
	writeCreated(r, c.AiRoute.Add(&route))
}

// queryAiRoute 对应 AiRoutesController.query
func (c *Controller) queryAiRoute(r *ghttp.Request) {
	writeGet(r, c.AiRoute.Query(pathParam(r, "name")))
}

// updateAiRoute 对应 AiRoutesController.put（Java 的 isNotEmpty 逻辑颠倒，这里采用 isEmpty 语义）。
func (c *Controller) updateAiRoute(r *ghttp.Request) {
	name := pathParam(r, "name")
	var route model.AiRoute
	parseBody(r, &route)
	if isEmpty(route.Name) {
		route.Name = strPtr(name)
	} else if derefStr(route.Name) != name {
		panic(errs.Validation("Route name in the URL doesn't match the one in the body."))
	}
	validateAiRoute(&route)
	writeUpdated(r, c.AiRoute.Update(&route))
}

// deleteAiRoute 对应 AiRoutesController.delete
func (c *Controller) deleteAiRoute(r *ghttp.Request) {
	c.AiRoute.Delete(pathParam(r, "name"))
	writeNoContent(r)
}
