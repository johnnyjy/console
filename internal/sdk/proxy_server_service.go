package sdk

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"console/internal/errs"
	k8s "console/internal/kubernetes"
	"console/internal/model"
)

// ProxyServerService 对应 Java 的 ProxyServerServiceImpl
type ProxyServerService struct {
	client    *k8s.Client
	converter *Converter
}

// NewProxyServerService 创建 ProxyServerService
func NewProxyServerService(client *k8s.Client, converter *Converter) *ProxyServerService {
	return &ProxyServerService{client: client, converter: converter}
}

// List 对应 list
func (s *ProxyServerService) List(query *model.CommonPageQuery) *model.PaginatedResult[model.ProxyServer] {
	mcpBridge, err := s.client.ReadMcpBridge(context.Background(), k8s.McpBridgeDefaultName)
	if err != nil {
		panic(errs.Business("Error occurs when getting McpBridge."))
	}
	var proxyServers []model.ProxyServer
	if mcpBridge != nil {
		resourceVersion := mcpBridge.ResourceVersion
		for i := range mcpBridge.Spec.Proxies {
			proxyServer := s.convert(&mcpBridge.Spec.Proxies[i])
			proxyServer.Version = strPtr(resourceVersion)
			proxyServers = append(proxyServers, *proxyServer)
		}
	}
	return model.CreateFromFullList(proxyServers, query)
}

// AddOrUpdate 对应 addOrUpdate
func (s *ProxyServerService) AddOrUpdate(proxyServer *model.ProxyServer) *model.ProxyServer {
	mcpBridge, err := s.client.ReadMcpBridge(context.Background(), k8s.McpBridgeDefaultName)
	if err != nil {
		panic(errs.Business("Error occurs when getting McpBridge."))
	}
	if mcpBridge == nil {
		mcpBridge = &k8s.McpBridge{}
		s.converter.InitV1McpBridge(mcpBridge)
		s.converter.AddV1McpBridgeProxy(mcpBridge, proxyServer)
		if _, err := s.client.CreateMcpBridge(context.Background(), mcpBridge); err != nil {
			if apierrors.IsConflict(err) {
				panic(errs.Conflict(""))
			}
			panic(errs.Business("Error occurs when adding or updating the ProxyServer with name: " +
				deref(proxyServer.Name)))
		}
	} else {
		s.converter.AddV1McpBridgeProxy(mcpBridge, proxyServer)
		if _, err := s.client.ReplaceMcpBridge(context.Background(), mcpBridge); err != nil {
			if apierrors.IsConflict(err) {
				panic(errs.Conflict(""))
			}
			panic(errs.Business("Error occurs when adding or updating the ProxyServer with name: " +
				deref(proxyServer.Name)))
		}
	}
	return proxyServer
}

// Delete 对应 delete
func (s *ProxyServerService) Delete(name string) {
	mcpBridge, err := s.client.ReadMcpBridge(context.Background(), k8s.McpBridgeDefaultName)
	if err != nil {
		panic(errs.Business("Error occurs when getting McpBridge."))
	}
	if mcpBridge == nil {
		return
	}
	for i := range mcpBridge.Spec.Registries {
		if name == mcpBridge.Spec.Registries[i].ProxyName {
			panic(errs.Business("Cannot delete the ProxyServer with name: " + name +
				", because it is used by a service registry."))
		}
	}
	proxy := s.converter.RemoveV1McpBridgeProxy(mcpBridge, name)
	if proxy == nil {
		// There is nothing to delete.
		return
	}
	if _, err := s.client.ReplaceMcpBridge(context.Background(), mcpBridge); err != nil {
		panic(errs.Business("Error occurs when deleting the ProxyServer with name: " + name))
	}
}

// Query 对应 query
func (s *ProxyServerService) Query(name string) *model.ProxyServer {
	mcpBridge, err := s.client.ReadMcpBridge(context.Background(), k8s.McpBridgeDefaultName)
	if err != nil {
		panic(errs.Business("Error occurs when getting McpBridge."))
	}
	if mcpBridge == nil || len(mcpBridge.Spec.Proxies) == 0 {
		return nil
	}
	for i := range mcpBridge.Spec.Proxies {
		if name == mcpBridge.Spec.Proxies[i].Name {
			proxyServer := s.convert(&mcpBridge.Spec.Proxies[i])
			proxyServer.Version = strPtr(mcpBridge.ResourceVersion)
			return proxyServer
		}
	}
	return nil
}

// Add 对应 add
func (s *ProxyServerService) Add(proxyServer *model.ProxyServer) *model.ProxyServer {
	mcpBridge, err := s.client.ReadMcpBridge(context.Background(), k8s.McpBridgeDefaultName)
	if err != nil {
		panic(errs.Business("Error occurs when getting McpBridge."))
	}
	if mcpBridge == nil {
		mcpBridge = &k8s.McpBridge{}
		s.converter.InitV1McpBridge(mcpBridge)
		s.converter.AddV1McpBridgeProxy(mcpBridge, proxyServer)
		if _, err := s.client.CreateMcpBridge(context.Background(), mcpBridge); err != nil {
			if apierrors.IsConflict(err) {
				panic(errs.Conflict(""))
			}
			panic(errs.Business("Error occurs when adding the ProxyServer with name: " + deref(proxyServer.Name)))
		}
		return proxyServer
	}

	for i := range mcpBridge.Spec.Proxies {
		if deref(proxyServer.Name) == mcpBridge.Spec.Proxies[i].Name {
			panic(errs.Conflict(""))
		}
	}
	s.converter.AddV1McpBridgeProxy(mcpBridge, proxyServer)
	if _, err := s.client.ReplaceMcpBridge(context.Background(), mcpBridge); err != nil {
		if apierrors.IsConflict(err) {
			panic(errs.Conflict(""))
		}
		panic(errs.Business("Error occurs when adding the ProxyServer with name: " + deref(proxyServer.Name)))
	}
	return proxyServer
}

func (s *ProxyServerService) convert(proxy *k8s.ProxyConfig) *model.ProxyServer {
	return s.converter.V1ProxyConfig2ProxyServer(proxy)
}
