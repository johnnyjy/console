package controller

import (
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"

	"console/internal/consts"
	"console/internal/errs"
	"console/internal/model"
)

// stripServiceSourceSensitiveInfo 对应 ServiceSourceController.stripSensitiveInfo
func stripServiceSourceSensitiveInfo(s *model.ServiceSource) {
	if s == nil || s.AuthN == nil {
		return
	}
	s.AuthN.Properties = nil
}

// listServiceSources 对应 ServiceSourceController.list
func (c *Controller) listServiceSources(r *ghttp.Request) {
	result := c.ServiceSource.List(parseCommonPageQuery(r))
	for i := range result.Data {
		stripServiceSourceSensitiveInfo(&result.Data[i])
	}
	writePaginated(r, result)
}

// addServiceSource 对应 ServiceSourceController.add
func (c *Controller) addServiceSource(r *ghttp.Request) {
	var serviceSource model.ServiceSource
	parseBody(r, &serviceSource)
	validateServiceSource(&serviceSource)
	if strings.HasSuffix(derefStr(serviceSource.Name), consts.InternalResourceNameSuffix) {
		panic(errs.Validation("Adding an internal service source is not allowed."))
	}
	result := c.ServiceSource.Add(&serviceSource)
	stripServiceSourceSensitiveInfo(result)
	writeCreated(r, result)
}

// addOrUpdateServiceSource 对应 ServiceSourceController.addOrUpdate
func (c *Controller) addOrUpdateServiceSource(r *ghttp.Request) {
	name := pathParam(r, "name")
	var serviceSource model.ServiceSource
	parseBody(r, &serviceSource)
	serviceSource.Name = strPtr(name)
	validateServiceSource(&serviceSource)
	if strings.HasSuffix(derefStr(serviceSource.Name), consts.InternalResourceNameSuffix) {
		panic(errs.Validation("Updating an internal service source is not allowed."))
	}
	result := c.ServiceSource.AddOrUpdate(&serviceSource)
	stripServiceSourceSensitiveInfo(result)
	writeUpdated(r, result)
}

// deleteServiceSource 对应 ServiceSourceController.delete
func (c *Controller) deleteServiceSource(r *ghttp.Request) {
	name := pathParam(r, "name")
	if strings.HasSuffix(name, consts.InternalResourceNameSuffix) {
		panic(errs.Validation("Deleting an internal service source is not allowed."))
	}
	c.ServiceSource.Delete(name)
	writeNoContent(r)
}

// queryServiceSource 对应 ServiceSourceController.query
func (c *Controller) queryServiceSource(r *ghttp.Request) {
	result := c.ServiceSource.Query(pathParam(r, "name"))
	stripServiceSourceSensitiveInfo(result)
	writeGet(r, result)
}
