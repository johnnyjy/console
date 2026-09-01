package sdk

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"console/internal/errs"
	k8s "console/internal/kubernetes"
	"console/internal/model"
)

const secretNameAttempts = 5

// ServiceSourceService 对应 Java 的 ServiceSourceServiceImpl
type ServiceSourceService struct {
	client    *k8s.Client
	converter *Converter
}

// NewServiceSourceService 创建 ServiceSourceService
func NewServiceSourceService(client *k8s.Client, converter *Converter) *ServiceSourceService {
	return &ServiceSourceService{client: client, converter: converter}
}

// List 对应 list
func (s *ServiceSourceService) List(query *model.CommonPageQuery) *model.PaginatedResult[model.ServiceSource] {
	mcpBridge, err := s.client.ReadMcpBridge(context.Background(), k8s.McpBridgeDefaultName)
	if err != nil {
		panic(errs.Business("Error occurs when getting McpBridge."))
	}
	var serviceSources []model.ServiceSource
	if mcpBridge != nil {
		resourceVersion := mcpBridge.ResourceVersion
		for i := range mcpBridge.Spec.Registries {
			source := s.convert(&mcpBridge.Spec.Registries[i])
			source.Version = strPtr(resourceVersion)
			serviceSources = append(serviceSources, *source)
		}
	}
	return model.CreateFromFullList(serviceSources, query)
}

// AddOrUpdate 对应 addOrUpdate
func (s *ServiceSourceService) AddOrUpdate(serviceSource *model.ServiceSource) *model.ServiceSource {
	mcpBridge, err := s.client.ReadMcpBridge(context.Background(), k8s.McpBridgeDefaultName)
	if err != nil {
		panic(errs.Business("Error occurs when getting McpBridge."))
	}
	if mcpBridge == nil {
		mcpBridge = &k8s.McpBridge{}
		s.converter.InitV1McpBridge(mcpBridge)
		registry := s.converter.AddV1McpBridgeRegistry(mcpBridge, serviceSource)
		s.syncAuthSecret(serviceSource, registry)
		if _, err := s.client.CreateMcpBridge(context.Background(), mcpBridge); err != nil {
			if apierrors.IsConflict(err) {
				panic(errs.Conflict(""))
			}
			panic(errs.Business("Error occurs when adding or updating the ServiceSource with name: " +
				deref(serviceSource.Name)))
		}
	} else {
		registry := s.converter.AddV1McpBridgeRegistry(mcpBridge, serviceSource)
		s.syncAuthSecret(serviceSource, registry)
		if _, err := s.client.ReplaceMcpBridge(context.Background(), mcpBridge); err != nil {
			if apierrors.IsConflict(err) {
				panic(errs.Conflict(""))
			}
			panic(errs.Business("Error occurs when adding or updating the ServiceSource with name: " +
				deref(serviceSource.Name)))
		}
	}
	return serviceSource
}

// Add 对应 add
func (s *ServiceSourceService) Add(serviceSource *model.ServiceSource) *model.ServiceSource {
	mcpBridge, err := s.client.ReadMcpBridge(context.Background(), k8s.McpBridgeDefaultName)
	if err != nil {
		panic(errs.Business("Error occurs when getting McpBridge."))
	}
	if mcpBridge == nil {
		mcpBridge = &k8s.McpBridge{}
		s.converter.InitV1McpBridge(mcpBridge)
		registry := s.converter.AddV1McpBridgeRegistry(mcpBridge, serviceSource)
		s.syncAuthSecret(serviceSource, registry)
		if _, err := s.client.CreateMcpBridge(context.Background(), mcpBridge); err != nil {
			if apierrors.IsConflict(err) {
				panic(errs.Conflict(""))
			}
			panic(errs.Business("Error occurs when adding the ServiceSource with name: " + deref(serviceSource.Name)))
		}
		return serviceSource
	}

	for i := range mcpBridge.Spec.Registries {
		if deref(serviceSource.Name) == mcpBridge.Spec.Registries[i].Name {
			panic(errs.Conflict(""))
		}
	}
	registry := s.converter.AddV1McpBridgeRegistry(mcpBridge, serviceSource)
	s.syncAuthSecret(serviceSource, registry)
	if _, err := s.client.ReplaceMcpBridge(context.Background(), mcpBridge); err != nil {
		if apierrors.IsConflict(err) {
			panic(errs.Conflict(""))
		}
		panic(errs.Business("Error occurs when adding the ServiceSource with name: " + deref(serviceSource.Name)))
	}
	return serviceSource
}

// Delete 对应 delete
func (s *ServiceSourceService) Delete(name string) {
	mcpBridge, err := s.client.ReadMcpBridge(context.Background(), k8s.McpBridgeDefaultName)
	if err != nil {
		panic(errs.Business("Error occurs when getting McpBridge."))
	}
	if mcpBridge == nil {
		return
	}
	registry := s.converter.RemoveV1McpBridgeRegistry(mcpBridge, name)
	if registry == nil {
		return
	}
	if registry.AuthSecretName != "" {
		if err := s.client.DeleteSecret(context.Background(), registry.AuthSecretName); err != nil {
			panic(errs.Business("Error occurs when deleting the secret associated with ServiceSource named " +
				registry.Name))
		}
	}
	if _, err := s.client.ReplaceMcpBridge(context.Background(), mcpBridge); err != nil {
		panic(errs.Business("Error occurs when deleting the ServiceSource with name: " + name))
	}
}

// Query 对应 query
func (s *ServiceSourceService) Query(name string) *model.ServiceSource {
	mcpBridge, err := s.client.ReadMcpBridge(context.Background(), k8s.McpBridgeDefaultName)
	if err != nil {
		panic(errs.Business("Error occurs when getting McpBridge."))
	}
	if mcpBridge == nil || len(mcpBridge.Spec.Registries) == 0 {
		return nil
	}
	for i := range mcpBridge.Spec.Registries {
		if name == mcpBridge.Spec.Registries[i].Name {
			source := s.convert(&mcpBridge.Spec.Registries[i])
			source.Version = strPtr(mcpBridge.ResourceVersion)
			return source
		}
	}
	return nil
}

func (s *ServiceSourceService) syncAuthSecret(serviceSource *model.ServiceSource, registry *k8s.RegistryConfig) {
	authN := serviceSource.AuthN
	authEnabledCurrent := registry.AuthSecretName != ""
	authEnabledTarget := authN != nil && authN.Enabled != nil && *authN.Enabled

	if !authEnabledCurrent && !authEnabledTarget {
		return
	}
	if !authEnabledTarget {
		if err := s.client.DeleteSecret(context.Background(), registry.AuthSecretName); err != nil {
			panic(errs.Business("Error occurs when deleting the secret associated with ServiceSource named " +
				deref(serviceSource.Name)))
		}
		registry.AuthSecretName = ""
		return
	}
	if !authEnabledCurrent || len(authN.Properties) > 0 {
		if !validateAuthN(authN, registry.Type) {
			panic(errs.Validation("The authN properties are not valid."))
		}
	}
	if len(authN.Properties) == 0 {
		return
	}

	secretData := map[string][]byte{}
	for k, v := range authN.Properties {
		secretData[k] = []byte(v)
	}

	if registry.AuthSecretName != "" {
		secret, err := s.client.ReadSecret(context.Background(), registry.AuthSecretName)
		if err != nil {
			panic(errs.Business("Error occurs when getting the secret associated with ServiceSource named " +
				deref(serviceSource.Name)))
		}
		secretLost := false
		if secret == nil {
			secret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: registry.AuthSecretName}}
			secretLost = true
		}
		secret.Data = secretData
		if secretLost {
			if _, err := s.client.CreateSecret(context.Background(), secret); err != nil {
				panic(errs.Business("Error occurs when updating the secret associated with ServiceSource named " +
					deref(serviceSource.Name)))
			}
		} else {
			if _, err := s.client.ReplaceSecret(context.Background(), secret); err != nil {
				panic(errs.Business("Error occurs when updating the secret associated with ServiceSource named " +
					deref(serviceSource.Name)))
			}
		}
	} else {
		done := false
		for i := 0; !done && i < secretNameAttempts; i++ {
			name := s.converter.GenerateAuthSecretName(deref(serviceSource.Name))
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name}, Data: secretData}
			if _, err := s.client.CreateSecret(context.Background(), secret); err != nil {
				if apierrors.IsConflict(err) {
					continue
				}
				panic(errs.Business("Error occurs when creating the secret associated with ServiceSource named " +
					deref(serviceSource.Name)))
			}
			registry.AuthSecretName = name
			done = true
		}
		if !done {
			panic(errs.Business("Failed to find an unused name for the secret associated with ServiceSource named " +
				deref(serviceSource.Name)))
		}
	}
}

func (s *ServiceSourceService) convert(registry *k8s.RegistryConfig) *model.ServiceSource {
	source := s.converter.V1RegistryConfig2ServiceSource(registry)
	authN := &model.ServiceSourceAuthN{Enabled: boolPtr(false), Properties: map[string]string{}}
	if registry.AuthSecretName != "" {
		secret, err := s.client.ReadSecret(context.Background(), registry.AuthSecretName)
		if err != nil {
			panic(errs.Business("Error occurs when getting McpBridge."))
		}
		if secret != nil && len(secret.Data) > 0 {
			authN.Enabled = boolPtr(true)
			properties := map[string]string{}
			for k, v := range secret.Data {
				properties[k] = string(v)
			}
			authN.Properties = properties
		}
	}
	source.AuthN = authN
	return source
}

func validateAuthN(authN *model.ServiceSourceAuthN, registryType string) bool {
	if authN == nil || authN.Enabled == nil || !*authN.Enabled {
		return true
	}
	if len(authN.Properties) == 0 {
		return false
	}
	switch registryType {
	case k8s.McpBridgeRegistryTypeNacos, k8s.McpBridgeRegistryTypeNacos2, k8s.McpBridgeRegistryTypeNacos3:
		if strings.TrimSpace(authN.Properties[k8s.McpBridgeRegistryTypeNacosUsername]) == "" {
			return false
		}
		if strings.TrimSpace(authN.Properties[k8s.McpBridgeRegistryTypeNacosPassword]) == "" {
			return false
		}
		return true
	case k8s.McpBridgeRegistryTypeConsul:
		return strings.TrimSpace(authN.Properties[k8s.McpBridgeRegistryTypeConsulToken]) != ""
	default:
		return true
	}
}
