package controller

import (
	"net/http"

	"github.com/gogf/gf/v2/net/ghttp"

	"console/internal/consts"
	"console/internal/errs"
	"console/internal/model/dto"
)

// initSystem 对应 SystemController.initialize
func (c *Controller) initSystem(r *ghttp.Request) {
	var req dto.SystemInitRequest
	parseBody(r, &req)
	adminUser := req.AdminUser
	if adminUser == nil {
		panic(errs.Validation("Missing adminUser."))
	}
	if isBlank(adminUser.Name) || isBlank(adminUser.DisplayName) || isBlank(adminUser.Password) {
		panic(errs.Validation("Incomplete adminUser object."))
	}
	c.System.InitSystem(adminUser, req.Configs)
	writeSuccessNull(r)
}

// getSystemInfo 对应 SystemController.info
func (c *Controller) getSystemInfo(r *ghttp.Request) {
	writeRaw(r, http.StatusOK, c.System.GetSystemInfo())
}

// getSystemConfigs 对应 SystemController.getConfigs
func (c *Controller) getSystemConfigs(r *ghttp.Request) {
	keys := c.Config.GetConfigKeys()
	configs := make(map[string]any, len(keys))
	for _, key := range keys {
		configs[key] = c.getConfigValue(key)
	}
	writeSuccess(r, configs)
}

// getConfigValue 对应 UserConfigKey.getConfigValueType 的类型分发。
func (c *Controller) getConfigValue(key string) any {
	switch key {
	case consts.ChatEnabled, consts.AdminPasswordChangeDisabled, consts.DashboardBuiltin,
		consts.DefaultRouteInitialized, consts.SystemInitialized:
		return c.Config.GetBoolean(key)
	default:
		return c.Config.GetString(key)
	}
}

// getHigressConfig 对应 SystemController.getHigressConfig
func (c *Controller) getHigressConfig(r *ghttp.Request) {
	writeSuccess(r, c.System.GetHigressConfig())
}

// updateHigressConfig 对应 SystemController.updateHigressConfig
func (c *Controller) updateHigressConfig(r *ghttp.Request) {
	var req dto.UpdateHigressConfigRequest
	parseBody(r, &req)
	if isEmpty(req.Config) {
		panic(errs.Validation("Missing required parameter: config"))
	}
	writeSuccess(r, c.System.SetHigressConfig(derefStr(req.Config)))
}
