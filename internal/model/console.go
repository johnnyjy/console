package model

// User 对应 Java 的 User
type User struct {
	Name        *string `json:"name,omitempty"`
	Password    *string `json:"password,omitempty"`
	DisplayName *string `json:"displayName,omitempty"`
	AvatarUrl   *string `json:"avatarUrl,omitempty"`
}

// SystemInfo 对应 Java 的 SystemInfo
type SystemInfo struct {
	Version      *string  `json:"version,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// DashboardInfo 对应 Java 的 DashboardInfo
type DashboardInfo struct {
	BuiltIn *bool   `json:"builtIn,omitempty"`
	Uid     *string `json:"uid,omitempty"`
	Url     *string `json:"url,omitempty"`
}

// DashboardType 对应 Java 的 DashboardType
type DashboardType string

const (
	DashboardMain DashboardType = "MAIN"
	DashboardAi   DashboardType = "AI"
	DashboardLog  DashboardType = "LOG"
)
