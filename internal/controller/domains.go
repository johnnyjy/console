package controller

import (
	"github.com/gogf/gf/v2/net/ghttp"

	"console/internal/errs"
	"console/internal/model"
)

// listDomains 对应 DomainsController.list
func (c *Controller) listDomains(r *ghttp.Request) {
	writePaginated(r, c.Domain.List(parseCommonPageQuery(r)))
}

// addDomain 对应 DomainsController.add
func (c *Controller) addDomain(r *ghttp.Request) {
	var domain model.Domain
	parseBody(r, &domain)
	writeCreated(r, c.Domain.Add(&domain))
}

// queryDomain 对应 DomainsController.query
func (c *Controller) queryDomain(r *ghttp.Request) {
	writeGet(r, c.Domain.Query(pathParam(r, "name")))
}

// updateDomain 对应 DomainsController.put
func (c *Controller) updateDomain(r *ghttp.Request) {
	name := pathParam(r, "name")
	var domain model.Domain
	parseBody(r, &domain)
	if isBlank(domain.Name) {
		domain.Name = strPtr(name)
	} else if derefStr(domain.Name) != name {
		panic(errs.Validation("Domain name in the URL doesn't match the one in the body."))
	}
	writeUpdated(r, c.Domain.Put(&domain))
}

// deleteDomain 对应 DomainsController.delete
func (c *Controller) deleteDomain(r *ghttp.Request) {
	c.Domain.Delete(pathParam(r, "name"))
	writeNoContent(r)
}

// queryDomainRoutes 对应 DomainsController.queryRoutes
func (c *Controller) queryDomainRoutes(r *ghttp.Request) {
	name := pathParam(r, "name")
	query := &model.RoutePageQuery{
		CommonPageQuery: model.CommonPageQuery{
			PageNum:  queryInt(r, "pageNum"),
			PageSize: queryInt(r, "pageSize"),
		},
		DomainName: strPtr(name),
	}
	writePaginated(r, c.Route.List(query))
}
