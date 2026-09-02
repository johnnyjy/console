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

	// 非后端 API（静态资源、.html 页面与 SPA 路由）一律匿名，由 serveStatic 兜底处理。
	if !isApiRequest(path) {
		return true
	}

	// 后端 API 中允许匿名访问的端点。
	method := r.Method
	if method == http.MethodPost && path == "/system/init" {
		return true
	}
	if method == http.MethodGet && (path == "/system/info" || path == "/system/config") {
		return true
	}
	return false
}

// isApiRequest 判断路径是否属于需要登录校验的后端 API。
// 仅匹配带斜杠的多段 API 前缀，避免误伤无后缀的前端路由（如 /dashboard.html）。
func isApiRequest(path string) bool {
	for _, p := range []string{"/v1/", "/user/", "/dashboard/", "/system/"} {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return path == "/grafana" || strings.HasPrefix(path, "/grafana/") ||
		path == "/aiproxy" || strings.HasPrefix(path, "/aiproxy/")
}

func currentUser(r *ghttp.Request) *model.User {
	v := r.GetCtxVar(currentUserCtxKey)
	if v == nil {
		return nil
	}
	u, _ := v.Val().(*model.User)
	return u
}
