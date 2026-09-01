package controller

import (
	"net/http"

	"github.com/gogf/gf/v2/net/ghttp"
)

// healthzReady 对应 HealthzController.ready
func (c *Controller) healthzReady(r *ghttp.Request) {
	r.Response.WriteHeader(http.StatusOK)
	r.Response.Write("ok")
}
