package controller

import (
	"net/http"
	"reflect"

	"github.com/gogf/gf/v2/net/ghttp"

	"console/internal/errs"
	"console/internal/model"
)

// writeJSON 写 JSON 响应并设置状态码。
func writeJSON(r *ghttp.Request, status int, data any) {
	r.Response.WriteHeader(status)
	r.Response.WriteJson(data)
}

// writeRaw 直接序列化 data（不包装），指定状态码。
func writeRaw(r *ghttp.Request, status int, data any) {
	writeJSON(r, status, data)
}

// writeNoContent 写 204 无 body。
func writeNoContent(r *ghttp.Request) {
	r.Response.WriteHeader(http.StatusNoContent)
}

// writeSuccess 写 200 包装成功响应 Response.success(data)。
func writeSuccess(r *ghttp.Request, data any) {
	writeJSON(r, http.StatusOK, map[string]any{
		"success": true,
		"message": nil,
		"data":    data,
	})
}

// writeSuccessNull 写 200 包装成功响应 Response.success(null)。
func writeSuccessNull(r *ghttp.Request) {
	writeSuccess(r, nil)
}

// writeGet 对应 ControllerUtil.buildResponseEntity 的 GET 分支：
// result == null -> 404 无 body；否则 200 Response.success(result)。
func writeGet(r *ghttp.Request, data any) {
	if isNilValue(data) {
		r.Response.WriteHeader(http.StatusNotFound)
		return
	}
	writeSuccess(r, data)
}

// writeCreated 对应 ControllerUtil.buildResponseEntity 的 POST 分支：201 body=result。
func writeCreated(r *ghttp.Request, data any) {
	writeRaw(r, http.StatusCreated, data)
}

// writeUpdated 对应 ControllerUtil.buildResponseEntity 的 default（PUT/PATCH）分支：200 body=result。
func writeUpdated(r *ghttp.Request, data any) {
	writeRaw(r, http.StatusOK, data)
}

// writePaginated 对应 PaginatedResponse.success(result)。
func writePaginated[T any](r *ghttp.Request, result *model.PaginatedResult[T]) {
	writeJSON(r, http.StatusOK, map[string]any{
		"success":  true,
		"message":  nil,
		"data":     result.Data,
		"total":    result.Total,
		"pageNum":  result.PageNum,
		"pageSize": result.PageSize,
	})
}

// writeFailure 对应 Response.failure(t)，message 为全限定类名 + ": " + message。
func writeFailure(r *ghttp.Request, e *errs.Error) {
	writeJSON(r, e.Status, map[string]any{
		"success": false,
		"message": e.Type + ": " + e.Message,
		"data":    nil,
	})
}

func isNilValue(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.Interface:
		return rv.IsNil()
	}
	return false
}
