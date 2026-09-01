package sdk

import (
	"context"
	"sort"
	"strings"

	"console/internal/consts"
	"console/internal/errs"
	k8s "console/internal/kubernetes"
	"console/internal/model"
)

const showMcpServicePorts = true

// ServiceService 对应 Java 的 ServiceServiceImpl
type ServiceService struct {
	client *k8s.Client
}

// NewServiceService 创建 ServiceService
func NewServiceService(client *k8s.Client) *ServiceService {
	return &ServiceService{client: client}
}

// List 对应 list
func (s *ServiceService) List(query *model.CommonPageQuery) *model.PaginatedResult[model.Service] {
	registryzServices, err := s.client.GatewayServiceList(context.Background())
	if err != nil {
		panic(errs.Business("Error occurs when listing services."))
	}
	if len(registryzServices) == 0 {
		return model.CreateFromFullList([]model.Service{}, query)
	}

	serviceEndpoint, err := s.client.GatewayServiceEndpoint(context.Background())
	if err != nil {
		panic(errs.Business("Error occurs when listing services."))
	}

	mcpBridgeDomain := s.getMcpBridgeDnsDomain()

	var services []model.Service
	for i := range registryzServices {
		rs := &registryzServices[i]
		namespace := ""
		if rs.Attributes != nil {
			namespace = rs.Attributes.Namespace
		}
		if s.client.IsNamespaceProtected(namespace) {
			continue
		}
		name := rs.Hostname
		endpoints := getServiceEndpoints(serviceEndpoint, namespace, name)
		if len(endpoints) == 0 {
			endpoints = s.completeMcpDnsEndpoints(rs, mcpBridgeDomain)
		}
		if len(rs.Ports) == 0 || (!showMcpServicePorts && strings.EqualFold(consts.McpNamespace, namespace)) {
			services = append(services, model.Service{
				Name:      strPtr(name),
				Namespace: strPtr(namespace),
				Endpoints: endpoints,
			})
		} else {
			seen := map[int]bool{}
			for _, port := range rs.Ports {
				if seen[port.Port] {
					continue
				}
				seen[port.Port] = true
				p := port.Port
				proto := port.Protocol
				services = append(services, model.Service{
					Name:      strPtr(name),
					Port:      &p,
					Namespace: strPtr(namespace),
					Endpoints: endpoints,
					Protocol:  strPtr(proto),
				})
			}
		}
	}

	sort.SliceStable(services, func(i, j int) bool {
		ni, nj := deref(services[i].Namespace), deref(services[j].Namespace)
		if ni != nj {
			return ni < nj
		}
		ai, aj := deref(services[i].Name), deref(services[j].Name)
		if ai != aj {
			return ai < aj
		}
		pi, pj := intValue(services[i].Port), intValue(services[j].Port)
		return pi < pj
	})

	return model.CreateFromFullList(services, query)
}

func (s *ServiceService) getMcpBridgeDnsDomain() map[string]string {
	bridges, err := s.client.ListMcpBridge(context.Background())
	if err != nil {
		panic(errs.Business("Error occurs when listing services."))
	}
	result := map[string]string{}
	for i := range bridges {
		spec := &bridges[i].Spec
		for _, registry := range spec.Registries {
			if strings.HasSuffix(strings.ToLower(registry.Type), k8s.McpBridgeRegistryTypeDNS) {
				key := registry.Name + consts.SeparatorDot + registry.Type
				result[key] = registry.Domain
			}
		}
	}
	return result
}

func (s *ServiceService) completeMcpDnsEndpoints(rs *k8s.RegistryzService, mcpBridgeDomain map[string]string) []string {
	attrs := rs.Attributes
	if attrs == nil {
		return nil
	}
	namespace := attrs.Namespace
	if strings.EqualFold(consts.McpNamespace, namespace) &&
		strings.HasSuffix(attrs.Name, consts.SeparatorDot+k8s.McpBridgeRegistryTypeDNS) {
		if domain := mcpBridgeDomain[attrs.Name]; domain != "" {
			return []string{domain}
		}
	}
	return nil
}

func getServiceEndpoints(serviceEndpoint map[string]map[string]k8s.IstioEndpointShard,
	serviceNamespace, serviceName string) []string {
	if serviceEndpoint == nil {
		return nil
	}
	namespace2Endpoints := serviceEndpoint[serviceName]
	if namespace2Endpoints == nil {
		return nil
	}
	endpointShard := namespace2Endpoints[serviceNamespace]
	if endpointShard.Shards == nil {
		return nil
	}
	var endpoints []string
	seen := map[string]bool{}
	for _, istioEndpoints := range endpointShard.Shards {
		for _, ep := range istioEndpoints {
			for _, addr := range ep.Addresses {
				if !seen[addr] {
					seen[addr] = true
					endpoints = append(endpoints, addr)
				}
			}
		}
	}
	return endpoints
}

func intValue(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
