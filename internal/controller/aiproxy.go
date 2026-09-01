package controller

import (
	"github.com/gogf/gf/v2/net/ghttp"
)

// forwardAiProxy 对应 AiProxyController.proxy
func (c *Controller) forwardAiProxy(r *ghttp.Request) {
	c.AiProxy.Proxy(r.Response.Writer, r.Request)
}
