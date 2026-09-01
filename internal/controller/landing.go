package controller

import (
	"github.com/gogf/gf/v2/net/ghttp"

	"console/resource"
)

// landingIndex 对应 LandingController.index
func (c *Controller) landingIndex(r *ghttp.Request) {
	r.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
	r.Response.Write(resource.LandingIndex)
}
