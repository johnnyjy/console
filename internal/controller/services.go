package controller

import (
	"github.com/gogf/gf/v2/net/ghttp"
)

// listServices 对应 ServicesController.list
func (c *Controller) listServices(r *ghttp.Request) {
	writePaginated(r, c.Service.List(parseCommonPageQuery(r)))
}
