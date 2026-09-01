package controller

import (
	"github.com/gogf/gf/v2/net/ghttp"

	"console/internal/errs"
	"console/internal/model/dto"
)

// login 对应 SessionController.login
func (c *Controller) login(r *ghttp.Request) {
	var req dto.LoginRequest
	parseBody(r, &req)
	if isEmpty(req.Username) || isEmpty(req.Password) {
		panic(errs.Validation("Missing user name or password."))
	}
	user := c.Session.Login(derefStr(req.Username), derefStr(req.Password))
	if user == nil {
		panic(errs.Auth("Incorrect username or password"))
	}
	c.Session.SaveSession(r.Response.Writer, user, boolValue(req.AutoLogin))
	writeCreated(r, user)
}

// logout 对应 SessionController.logout
func (c *Controller) logout(r *ghttp.Request) {
	c.Session.ClearSession(r.Response.Writer)
	writeSuccessNull(r)
}
