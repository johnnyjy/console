package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"

	"console/internal/errs"
	"console/internal/model"
)

const currentUserCtxKey = "console.currentUser"

// RecoverMiddleware 捕获 panic 并转换为 Response.failure，对应 ApiStandardizationAspect 的异常处理。
func RecoverMiddleware(r *ghttp.Request) {
	defer func() {
		if recovered := recover(); recovered != nil {
			switch e := recovered.(type) {
			case *errs.Error:
				writeFailure(r, e)
			case error:
				writeFailure(r, errs.Business(e.Error()))
			default:
				writeFailure(r, errs.Internal(fmt.Sprintf("%v", recovered)))
			}
		}
	}()
	r.Middleware.Next()
}

// AuthMiddleware 对应 ApiStandardizationAspect 的登录校验。
func AuthMiddleware(c *Controller) ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		if !isAnonymousRequest(r) {
			user := c.Session.ValidateSession(r.Request)
			if user == nil {
				panic(errs.Auth("Login required."))
			}
			r.SetCtxVar(currentUserCtxKey, user)
		}
		r.Middleware.Next()
	}
}

// isAnonymousRequest 对应 isLoginRequired 的反向逻辑（类级与方法级 @AllowAnonymous）。
func isAnonymousRequest(r *ghttp.Request) bool {
	path := r.URL.Path
	if pathInPrefix(path, "/session") || pathInPrefix(path, "/healthz") || pathInPrefix(path, "/landing") {
		return true
	}
	method := r.Method
	if method == http.MethodPost && path == "/system/init" {
		return true
	}
	if method == http.MethodGet && (path == "/system/info" || path == "/system/config") {
		return true
	}
	return false
}

func pathInPrefix(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func currentUser(r *ghttp.Request) *model.User {
	v := r.GetCtxVar(currentUserCtxKey)
	if v == nil {
		return nil
	}
	u, _ := v.Val().(*model.User)
	return u
}
