package controller

import (
	"github.com/gogf/gf/v2/net/ghttp"

	"console/internal/errs"
	"console/internal/model"
)

// listLlmProviders 对应 LlmProvidersController.list
func (c *Controller) listLlmProviders(r *ghttp.Request) {
	writePaginated(r, c.LlmProvider.List(parseCommonPageQuery(r)))
}

// addLlmProvider 对应 LlmProvidersController.add
func (c *Controller) addLlmProvider(r *ghttp.Request) {
	var provider model.LlmProvider
	parseBody(r, &provider)
	validateLlmProvider(&provider)
	writeCreated(r, c.LlmProvider.AddOrUpdate(&provider))
}

// queryLlmProvider 对应 LlmProvidersController.query
func (c *Controller) queryLlmProvider(r *ghttp.Request) {
	writeGet(r, c.LlmProvider.Query(pathParam(r, "name")))
}

// updateLlmProvider 对应 LlmProvidersController.put（Java 的 isNotEmpty 逻辑颠倒，这里采用 isEmpty 语义）。
func (c *Controller) updateLlmProvider(r *ghttp.Request) {
	name := pathParam(r, "name")
	var provider model.LlmProvider
	parseBody(r, &provider)
	if isEmpty(provider.Name) {
		provider.Name = strPtr(name)
	} else if derefStr(provider.Name) != name {
		panic(errs.Validation("Provider name in the URL doesn't match the one in the body."))
	}
	validateLlmProvider(&provider)
	writeUpdated(r, c.LlmProvider.AddOrUpdate(&provider))
}

// deleteLlmProvider 对应 LlmProvidersController.delete
func (c *Controller) deleteLlmProvider(r *ghttp.Request) {
	c.LlmProvider.Delete(pathParam(r, "name"))
	writeNoContent(r)
}
