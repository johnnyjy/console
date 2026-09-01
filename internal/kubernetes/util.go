package kubernetes

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"console/internal/consts"
)

// NormalizeDomainName 对应 KubernetesUtil.normalizeDomainName
func NormalizeDomainName(name string) string {
	if name != "" && strings.HasPrefix(name, consts.SeparatorAsterisk) {
		name = consts.Wildcard + name[len(consts.SeparatorAsterisk):]
	}
	return name
}

// JoinLabelSelectors 对应 KubernetesUtil.joinLabelSelectors
func JoinLabelSelectors(selectors ...string) string {
	var parts []string
	for _, s := range selectors {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, consts.SeparatorComma)
}

// BuildDomainLabelSelector 对应 KubernetesUtil.buildDomainLabelSelector
func BuildDomainLabelSelector(domainName string) string {
	return BuildLabelSelector(consts.LabelDomainKeyPrefix+NormalizeDomainName(domainName),
		consts.LabelDomainValueDummy)
}

// BuildLabelSelector 对应 KubernetesUtil.buildLabelSelector
func BuildLabelSelector(name, value string) string {
	if strings.Contains(value, consts.SeparatorComma) {
		return name + " in (" + value + ")"
	}
	return name + "=" + value
}

// BuildLabelSelectors 对应 KubernetesUtil.buildLabelSelectors
func BuildLabelSelectors(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	var parts []string
	for k, v := range labels {
		parts = append(parts, BuildLabelSelector(k, v))
	}
	return strings.Join(parts, consts.SeparatorComma)
}

// GetLabel 对应 KubernetesUtil.getLabel(V1ObjectMeta, key)
func GetLabel(meta *metav1.ObjectMeta, key string) string {
	if meta == nil || meta.Labels == nil {
		return ""
	}
	return meta.Labels[key]
}

// SetLabel 对应 KubernetesUtil.setLabel(V1ObjectMeta, key, value)
func SetLabel(meta *metav1.ObjectMeta, key, value string) {
	if meta.Labels == nil {
		meta.Labels = make(map[string]string)
	}
	meta.Labels[key] = value
}

// GetAnnotation 对应 KubernetesUtil.getAnnotation(V1ObjectMeta, key)
func GetAnnotation(meta *metav1.ObjectMeta, key string) string {
	if meta == nil || meta.Annotations == nil {
		return ""
	}
	return meta.Annotations[key]
}

// SetAnnotation 对应 KubernetesUtil.setAnnotation(V1ObjectMeta, key, value)
func SetAnnotation(meta *metav1.ObjectMeta, key, value string) {
	if meta.Annotations == nil {
		meta.Annotations = make(map[string]string)
	}
	meta.Annotations[key] = value
}

// IsInternalResource 对应 KubernetesUtil.isInternalResource
func IsInternalResource(name string) bool {
	return name != "" && strings.HasSuffix(name, consts.InternalResourceNameSuffix)
}

// IsInternalService 对应 KubernetesUtil.isInternalService
func IsInternalService(serviceName string) bool {
	if serviceName == "" {
		return false
	}
	suffix := consts.InternalResourceNameSuffix + "." + McpBridgeRegistryTypeDNS
	return strings.HasSuffix(serviceName, suffix)
}

// ImageUrl 对应 ImageUrl
type ImageUrl struct {
	Repository string
	Tag        string
}

func (u ImageUrl) ToUrlString() string {
	if u.Tag != "" {
		return u.Repository + ":" + u.Tag
	}
	return u.Repository
}

// ParseImageUrl 对应 ImageUrl.parse
func ParseImageUrl(url string) ImageUrl {
	colonIndex := strings.LastIndex(url, consts.SeparatorColon)
	if colonIndex == -1 {
		return ImageUrl{Repository: url}
	}
	protocolIndex := strings.Index(url, consts.ProtocolKeyword)
	if protocolIndex != -1 && !strings.HasPrefix(url, consts.OciProtocol) {
		// 不是 OCI 镜像 URL，可能是 http:// 或 file://
		return ImageUrl{Repository: url}
	}
	if colonIndex <= protocolIndex {
		return ImageUrl{Repository: url}
	}
	return ImageUrl{Repository: url[:colonIndex], Tag: url[colonIndex+1:]}
}
