package controller

import (
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"

	"console/internal/consts"
	"console/internal/errs"
	"console/internal/model"
)

// listRoutes 对应 RoutesController.list
func (c *Controller) listRoutes(r *ghttp.Request) {
	writePaginated(r, c.Route.List(parseRoutePageQuery(r)))
}

// queryRoute 对应 RoutesController.query
func (c *Controller) queryRoute(r *ghttp.Request) {
	writeGet(r, c.Route.Query(pathParam(r, "name")))
}

// addRoute 对应 RoutesController.add
func (c *Controller) addRoute(r *ghttp.Request) {
	var route model.Route
	parseBody(r, &route)
	if isBlank(route.Name) {
		panic(errs.Validation("Route name is required."))
	}
	if strings.HasSuffix(derefStr(route.Name), consts.InternalResourceNameSuffix) {
		panic(errs.Validation("Adding an internal route is not allowed."))
	}
	validateRoute(&route)
	writeCreated(r, c.Route.Add(&route))
}

// updateRoute 对应 RoutesController.update
func (c *Controller) updateRoute(r *ghttp.Request) {
	name := pathParam(r, "name")
	var route model.Route
	parseBody(r, &route)
	if isBlank(route.Name) {
		route.Name = strPtr(name)
	} else if derefStr(route.Name) != name {
		panic(errs.Validation("Route name in the URL doesn't match the one in the body."))
	}
	if strings.HasSuffix(derefStr(route.Name), consts.InternalResourceNameSuffix) {
		panic(errs.Validation("Updating an internal route is not allowed."))
	}
	validateRoute(&route)
	writeUpdated(r, c.Route.Update(&route))
}

// deleteRoute 对应 RoutesController.delete
func (c *Controller) deleteRoute(r *ghttp.Request) {
	name := pathParam(r, "name")
	if strings.HasSuffix(name, consts.InternalResourceNameSuffix) {
		panic(errs.Validation("Deleting an internal route is not allowed."))
	}
	c.Route.Delete(name)
	writeNoContent(r)
}
