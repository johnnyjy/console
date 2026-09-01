package controller

import (
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"

	"console/internal/errs"
	"console/internal/model"
)

// toDashboardType 对应 DashboardController.toDashboardType
func toDashboardType(typ string) model.DashboardType {
	if typ == "" {
		return model.DashboardMain
	}
	switch strings.ToUpper(typ) {
	case "MAIN":
		return model.DashboardMain
	case "AI":
		return model.DashboardAi
	case "LOG":
		return model.DashboardLog
	}
	panic(errs.Validation("Unknown dashboard type: " + typ))
}

// initDashboard 对应 DashboardController.init
func (c *Controller) initDashboard(r *ghttp.Request) {
	force := queryBool(r, "force")
	c.Dashboard.InitializeDashboard(force != nil && *force)
	writeSuccess(r, c.Dashboard.GetDashboardInfo(model.DashboardMain))
}

// getDashboardInfo 对应 DashboardController.info
func (c *Controller) getDashboardInfo(r *ghttp.Request) {
	typ := toDashboardType(queryString(r, "type"))
	writeSuccess(r, c.Dashboard.GetDashboardInfo(typ))
}

// setDashboardUrl 对应 DashboardController.setUrl
func (c *Controller) setDashboardUrl(r *ghttp.Request) {
	typ := toDashboardType(queryString(r, "type"))
	var dashboardInfo model.DashboardInfo
	parseBody(r, &dashboardInfo)
	if isEmpty(dashboardInfo.Url) {
		panic(errs.Validation("Missing required parameter: url"))
	}
	c.Dashboard.SetDashboardUrl(typ, derefStr(dashboardInfo.Url))
	writeSuccess(r, c.Dashboard.GetDashboardInfo(typ))
}

// getDashboardConfigData 对应 DashboardController.getConfigData
func (c *Controller) getDashboardConfigData(r *ghttp.Request) {
	typ := toDashboardType(queryString(r, "type"))
	dataSourceUid := queryString(r, "dataSourceUid")
	if dataSourceUid == "" {
		panic(errs.Validation("Missing required parameter: dataSourceUid"))
	}
	writeSuccess(r, c.Dashboard.BuildConfigData(typ, dataSourceUid))
}
