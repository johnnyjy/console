package controller

import (
	"github.com/gogf/gf/v2/net/ghttp"
)

// forwardGrafana 对应 GrafanaController.forward
func (c *Controller) forwardGrafana(r *ghttp.Request) {
	c.Dashboard.ForwardDashboardRequest(r.Response.Writer, r.Request)
}
