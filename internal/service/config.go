package service

import (
	"context"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"console/internal/consts"
	"console/internal/errs"
	"console/internal/kubernetes"
)

// ConfigService 对应 Java 的 ConfigServiceImpl
type ConfigService struct {
	client        *kubernetes.Client
	configMapName string
}

func NewConfigService(client *kubernetes.Client) *ConfigService {
	return &ConfigService{
		client:        client,
		configMapName: consts.ConfigMapNameDefault,
	}
}

func (s *ConfigService) GetString(key string) string {
	if key == "" {
		return ""
	}
	cm := s.getConfigMap()
	if cm == nil || len(cm.Data) == 0 {
		return ""
	}
	if v, ok := cm.Data[key]; ok {
		return trimSpace(v)
	}
	return ""
}

func (s *ConfigService) GetStringDefault(key, def string) string {
	if v := s.GetString(key); v != "" {
		return v
	}
	return def
}

func (s *ConfigService) GetBoolean(key string) *bool {
	v := s.GetString(key)
	if v == "" {
		return nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return nil
	}
	return &b
}

func (s *ConfigService) GetBooleanDefault(key string, def bool) bool {
	if v := s.GetBoolean(key); v != nil {
		return *v
	}
	return def
}

func (s *ConfigService) GetInteger(key string) *int {
	v := s.GetString(key)
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return nil
	}
	return &n
}

func (s *ConfigService) GetIntegerDefault(key string, def int) int {
	if v := s.GetInteger(key); v != nil {
		return *v
	}
	return def
}

func (s *ConfigService) GetLong(key string) *int64 {
	v := s.GetString(key)
	if v == "" {
		return nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}

func (s *ConfigService) GetLongDefault(key string, def int64) int64 {
	if v := s.GetLong(key); v != nil {
		return *v
	}
	return def
}

func (s *ConfigService) SetConfig(key, value string) {
	s.SetConfigs(map[string]interface{}{key: value})
}

func (s *ConfigService) SetConfigBool(key string, value bool) {
	s.SetConfig(key, strconv.FormatBool(value))
}

func (s *ConfigService) SetConfigInt(key string, value int) {
	s.SetConfig(key, strconv.Itoa(value))
}

func (s *ConfigService) SetConfigLong(key string, value int64) {
	s.SetConfig(key, strconv.FormatInt(value, 10))
}

func (s *ConfigService) SetConfigObject(key string, value interface{}) {
	if value == nil {
		s.RemoveConfig(key)
		return
	}
	s.SetConfig(key, toStr(value))
}

func (s *ConfigService) SetConfigs(configs map[string]interface{}) {
	if len(configs) == 0 {
		return
	}
	cm := s.getConfigMap()
	if cm == nil {
		cm = s.initConfigMap()
	}
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	for k, v := range configs {
		if v == nil {
			delete(cm.Data, k)
		} else {
			cm.Data[k] = toStr(v)
		}
	}
	if _, err := s.client.ReplaceConfigMap(context.Background(), cm); err != nil {
		panic(errs.Business("Error occurs when updating ConfigMap."))
	}
}

func (s *ConfigService) RemoveConfig(key string) {
	if key == "" {
		return
	}
	cm := s.getConfigMap()
	if cm == nil || len(cm.Data) == 0 {
		return
	}
	delete(cm.Data, key)
	if _, err := s.client.ReplaceConfigMap(context.Background(), cm); err != nil {
		panic(errs.Business("Error occurs when updating ConfigMap."))
	}
}

func (s *ConfigService) GetConfigKeys() []string {
	cm := s.getConfigMap()
	if cm == nil || len(cm.Data) == 0 {
		return []string{}
	}
	keys := make([]string, 0, len(cm.Data))
	for k := range cm.Data {
		keys = append(keys, k)
	}
	return keys
}

func (s *ConfigService) getConfigMap() *corev1.ConfigMap {
	cm, err := s.client.ReadConfigMap(context.Background(), s.configMapName)
	if err != nil {
		panic(errs.Business("Error occurs when reading ConfigMap " + s.configMapName))
	}
	return cm
}

func (s *ConfigService) initConfigMap() *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: s.configMapName},
		Data:       map[string]string{},
	}
	created, err := s.client.CreateConfigMap(context.Background(), cm)
	if err != nil {
		panic(errs.Business("Error occurs when creating ConfigMap " + s.configMapName))
	}
	return created
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

func toStr(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return ""
	}
}
