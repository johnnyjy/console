package kubernetes

import (
	"os"
	"strconv"
	"strings"

	"console/internal/consts"
)

// Config 对应 Java 的 HigressServiceConfig
type Config struct {
	KubeConfigPath                string
	KubeConfigContent             string
	ControllerNamespace           string
	ControllerWatchedNamespace    string
	ControllerWatchedIngressClass string
	ControllerServiceName         string
	ControllerServiceHost         string
	ControllerServicePort         int
	ControllerJwtPolicy           string
	ControllerAccessToken         string
	ServiceListSupportRegistry    bool
	ClusterDomainSuffix           string
}

// envGet 读取环境变量。Java 端通过 Spring 的 relaxed binding 将
// `higress-console.xxx` 映射为大写、点/横线转下划线的环境变量名。
func envGet(key, def string) string {
	envKey := strings.ToUpper(strings.NewReplacer(".", "_", "-", "_").Replace(key))
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return def
}

// EnvGet 供其他包读取环境变量（Spring relaxed binding 风格）。
func EnvGet(key, def string) string { return envGet(key, def) }

// EnvInt 供其他包读取整数环境变量。
func EnvInt(key string, def int) int { return envInt(key, def) }

// NewConfig 从环境变量构建配置，默认值与 Java HigressServiceConfig.Builder 一致。
func NewConfig() *Config {
	return &Config{
		KubeConfigPath:                envGet(consts.KubeConfigKey, ""),
		ControllerServiceName:         envGet(consts.ControllerServiceNameKey, consts.ControllerServiceNameDefault),
		ControllerNamespace:           envGet(consts.NsKey, consts.NsDefault),
		ControllerWatchedNamespace:    envGet(consts.ControllerWatchedNsKey, ""),
		ControllerWatchedIngressClass: envGet(consts.ControllerIngressClassKey, ""),
		ControllerServiceHost:         envGet(consts.ControllerServiceHostKey, consts.ControllerServiceHostDefault),
		ControllerServicePort:         envInt(consts.ControllerServicePortKey, consts.ControllerServicePortDefault),
		ControllerJwtPolicy:           envGet(consts.ControllerJwtPolicyKey, consts.ControllerJwtPolicyDefault),
		ControllerAccessToken:         envGet(consts.ControllerAccessTokenKey, ""),
		ServiceListSupportRegistry:    consts.ServiceListSupportRegistryDef,
		ClusterDomainSuffix:           envGet(consts.ClusterDomainSuffixEnv, consts.ClusterDomainSuffixDefault),
	}
}

func envInt(key string, def int) int {
	if v := envGet(key, ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
