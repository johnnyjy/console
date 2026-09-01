package controller

import (
	"github.com/gogf/gf/v2/net/ghttp"

	"console/internal/errs"
	"console/internal/model/dto"
)

// getUserInfo 对应 UserController.getUserInfo
func (c *Controller) getUserInfo(r *ghttp.Request) {
	writeGet(r, currentUser(r))
}

// changePassword 对应 UserController.logout
func (c *Controller) changePassword(r *ghttp.Request) {
	var req dto.ChangePasswordRequest
	parseBody(r, &req)
	if isEmpty(req.OldPassword) {
		panic(errs.Validation("Missing old password."))
	}
	if isEmpty(req.NewPassword) {
		panic(errs.Validation("Missing new password."))
	}
	user := currentUser(r)
	c.Session.ChangePassword(derefStr(user.Name), derefStr(req.OldPassword), derefStr(req.NewPassword))
	c.Session.ClearSession(r.Response.Writer)
	writeSuccessNull(r)
}
