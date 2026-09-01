package resource

import "embed"

//go:embed dashboard/main.json
var DashboardMain string

//go:embed dashboard/ai.json
var DashboardAi string

//go:embed dashboard/logs.json
var DashboardLogs string

//go:embed landing/index.html
var LandingIndex string

//go:embed plugins
var PluginsFS embed.FS
