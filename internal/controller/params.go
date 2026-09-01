package controller

import (
	"encoding/json"
	"strconv"

	"github.com/gogf/gf/v2/net/ghttp"

	"console/internal/errs"
	"console/internal/model"
)

func boolPtr(v bool) *bool { return &v }

// pathParam 获取路径参数。
func pathParam(r *ghttp.Request, key string) string {
	return r.Get(key).String()
}

// queryString 返回 query 参数；不存在时返回空串。
func queryString(r *ghttp.Request, key string) string {
	return r.GetQuery(key).String()
}

// queryInt 解析 query 整数参数；不存在或非法返回 nil。
func queryInt(r *ghttp.Request, key string) *int {
	s := r.GetQuery(key).String()
	if s == "" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &n
}

// queryBool 解析 query 布尔参数；不存在或非法返回 nil。
func queryBool(r *ghttp.Request, key string) *bool {
	s := r.GetQuery(key).String()
	if s == "" {
		return nil
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return nil
	}
	return &b
}

// parseBody 从请求体解析 JSON。
func parseBody(r *ghttp.Request, dst any) {
	if err := json.Unmarshal(r.GetBody(), dst); err != nil {
		panic(errs.Validation("Invalid request body."))
	}
}

// parseCommonPageQuery 解析通用分页参数。
func parseCommonPageQuery(r *ghttp.Request) *model.CommonPageQuery {
	return &model.CommonPageQuery{
		PageNum:  queryInt(r, "pageNum"),
		PageSize: queryInt(r, "pageSize"),
	}
}

// parseRoutePageQuery 解析路由分页参数（含 domainName/all）。
func parseRoutePageQuery(r *ghttp.Request) *model.RoutePageQuery {
	return &model.RoutePageQuery{
		CommonPageQuery: model.CommonPageQuery{
			PageNum:  queryInt(r, "pageNum"),
			PageSize: queryInt(r, "pageSize"),
		},
		DomainName: strPtrIf(queryString(r, "domainName") != "", queryString(r, "domainName")),
		All:        queryBool(r, "all"),
	}
}

// parseWasmPluginPageQuery 解析 wasm 插件分页参数（含 lang）。
func parseWasmPluginPageQuery(r *ghttp.Request) *model.WasmPluginPageQuery {
	return &model.WasmPluginPageQuery{
		CommonPageQuery: model.CommonPageQuery{
			PageNum:  queryInt(r, "pageNum"),
			PageSize: queryInt(r, "pageSize"),
		},
		Lang: strPtrIf(queryString(r, "lang") != "", queryString(r, "lang")),
	}
}

// strPtrIf 返回 s 的指针；若 ok 为 false 返回 nil。
func strPtrIf(ok bool, s string) *string {
	if !ok {
		return nil
	}
	return &s
}
