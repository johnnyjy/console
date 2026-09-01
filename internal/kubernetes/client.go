package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"console/internal/consts"
)

const (
	podServiceAccountTokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	controllerAccessTokenFile  = "/var/run/secrets/access-token/token"
)

var (
	mcpBridgeGVR = schema.GroupVersionResource{
		Group: McpBridgeAPIGroup, Version: McpBridgeVersion, Resource: McpBridgePlural,
	}
	wasmPluginGVR = schema.GroupVersionResource{
		Group: WasmPluginAPIGroup, Version: WasmPluginVersion, Resource: WasmPluginPlural,
	}
	envoyFilterGVR = schema.GroupVersionResource{
		Group: EnvoyFilterAPIGroup, Version: EnvoyFilterVersion, Resource: EnvoyFilterPlural,
	}
)

// Client 对应 Java 的 KubernetesClientService
type Client struct {
	cfg        *Config
	clientSet  kubernetes.Interface
	dynamic    dynamic.Interface
	httpClient *http.Client

	inClusterMode bool

	isIngressWatched    func(*networkingv1.Ingress) bool
	defaultIngressClass string
	IngressV1Supported  bool
	ClusterDomainSuffix string
}

// NewClient 对应 KubernetesClientService 构造器
func NewClient(cfg *Config) (*Client, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	c := &Client{
		cfg:                 cfg,
		httpClient:          &http.Client{},
		ClusterDomainSuffix: cfg.ClusterDomainSuffix,
	}

	restConfig, inCluster, err := buildRestConfig(cfg)
	if err != nil {
		return nil, err
	}
	c.inClusterMode = inCluster

	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	c.clientSet = clientSet
	c.dynamic = dynamicClient

	c.isIngressWatched = buildIsIngressWatchedPredicate(cfg.ControllerWatchedIngressClass)
	c.defaultIngressClass = firstNonEmpty(cfg.ControllerWatchedIngressClass, consts.ControllerIngressClassName)

	c.initializeK8sCapabilities()

	return c, nil
}

func buildRestConfig(cfg *Config) (*rest.Config, bool, error) {
	if cfg.KubeConfigPath == "" && cfg.KubeConfigContent == "" && isInCluster() {
		rc, err := rest.InClusterConfig()
		return rc, true, err
	}
	if cfg.KubeConfigContent != "" {
		rc, err := clientcmd.RESTConfigFromKubeConfig([]byte(cfg.KubeConfigContent))
		return rc, false, err
	}
	path := cfg.KubeConfigPath
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".kube", "config")
	}
	rc, err := clientcmd.BuildConfigFromFlags("", path)
	return rc, false, err
}

func isInCluster() bool {
	_, err := os.Stat(podServiceAccountTokenFile)
	return err == nil
}

func (c *Client) initializeK8sCapabilities() {
	c.IngressV1Supported = true
}

func validateConfig(cfg *Config) error {
	if isInCluster() {
		if cfg.ControllerServiceName == "" {
			return fmt.Errorf("controllerServiceName is required")
		}
	} else {
		if cfg.ControllerServiceHost == "" {
			return fmt.Errorf("controllerServiceHost is required")
		}
	}
	if cfg.ControllerNamespace == "" {
		return fmt.Errorf("controllerNamespace is required")
	}
	if cfg.ControllerServicePort <= 0 || cfg.ControllerServicePort > 65535 {
		return fmt.Errorf("controllerServicePort is invalid")
	}
	if cfg.ControllerJwtPolicy == "" {
		return fmt.Errorf("controllerJwtPolicy is required")
	}
	return nil
}

// IsNamespaceProtected 对应 isNamespaceProtected
func (c *Client) IsNamespaceProtected(namespace string) bool {
	return namespace == consts.KubeSystemNs || namespace == c.cfg.ControllerNamespace
}

// IsDefinedByConsole 对应 isDefinedByConsole
func (c *Client) IsDefinedByConsole(meta *metav1.ObjectMeta) bool {
	return meta != nil && c.cfg.ControllerNamespace == meta.Namespace &&
		consts.LabelResourceDefinerValue == GetLabel(meta, consts.LabelResourceDefinerKey)
}

// GatewayServiceList 对应 gatewayServiceList
func (c *Client) GatewayServiceList(ctx context.Context) ([]RegistryzService, error) {
	body, err := c.controllerGet(ctx, "/debug/registryz")
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, nil
	}
	var services []RegistryzService
	if err := json.Unmarshal(body, &services); err != nil {
		return nil, err
	}
	return services, nil
}

// GatewayServiceEndpoint 对应 gatewayServiceEndpoint
func (c *Client) GatewayServiceEndpoint(ctx context.Context) (map[string]map[string]IstioEndpointShard, error) {
	body, err := c.controllerGet(ctx, "/debug/endpointShardz")
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, nil
	}
	var result map[string]map[string]IstioEndpointShard
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) controllerGet(ctx context.Context, path string) ([]byte, error) {
	serviceHost := c.cfg.ControllerServiceHost
	if c.inClusterMode {
		serviceHost = c.cfg.ControllerServiceName + "." + c.cfg.ControllerNamespace
	}
	url := fmt.Sprintf("http://%s:%d%s", serviceHost, c.cfg.ControllerServicePort, path)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	token := c.cfg.ControllerAccessToken
	if token == "" && c.inClusterMode {
		token, _ = c.readTokenFromFile()
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to get controller data from %s. Code=%d", path, resp.StatusCode)
	}
	return body, nil
}

func (c *Client) readTokenFromFile() (string, error) {
	fileName := controllerAccessTokenFile
	if c.cfg.ControllerJwtPolicy == consts.KubernetesJwtPolicyFirstParty {
		fileName = podServiceAccountTokenFile
	}
	data, err := os.ReadFile(fileName)
	return string(data), err
}

// ---- Ingress ----

// ListAllIngresses 对应 listAllIngresses
func (c *Client) ListAllIngresses(ctx context.Context) ([]networkingv1.Ingress, error) {
	var ingresses []networkingv1.Ingress
	if c.cfg.ControllerWatchedNamespace == "" {
		list, err := c.clientSet.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		ingresses = append(ingresses, list.Items...)
	} else {
		for _, ns := range []string{c.cfg.ControllerNamespace, c.cfg.ControllerWatchedNamespace} {
			list, err := c.clientSet.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			ingresses = append(ingresses, list.Items...)
		}
	}
	c.retainWatchedIngress(&ingresses)
	sortIngressByName(ingresses)
	return ingresses, nil
}

// ListAllServiceList 对应 listAllServiceList
func (c *Client) ListAllServiceList(ctx context.Context) ([]corev1.Service, error) {
	list, err := c.clientSet.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := list.Items
	result = c.filterSystemServices(result)
	return result, nil
}

// ListAllEndPointsList 对应 listAllEndPointsList
func (c *Client) ListAllEndPointsList(ctx context.Context) ([]corev1.Endpoints, error) {
	list, err := c.clientSet.CoreV1().Endpoints("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := list.Items
	result = c.filterSystemEndpoints(result)
	return result, nil
}

func (c *Client) filterSystemServices(items []corev1.Service) []corev1.Service {
	filtered := items[:0]
	for _, item := range items {
		ns := item.Namespace
		if strings.HasPrefix(ns, "kube") {
			continue
		}
		if c.cfg.ControllerNamespace != "" && c.cfg.ControllerNamespace == ns {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func (c *Client) filterSystemEndpoints(items []corev1.Endpoints) []corev1.Endpoints {
	filtered := items[:0]
	for _, item := range items {
		ns := item.Namespace
		if strings.HasPrefix(ns, "kube") {
			continue
		}
		if c.cfg.ControllerNamespace != "" && c.cfg.ControllerNamespace == ns {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

// ListIngress 对应 listIngress / listIngress(Map<String,String>)
func (c *Client) ListIngress(ctx context.Context, labelMap map[string]string) ([]networkingv1.Ingress, error) {
	labelSelector := defaultLabelSelectors()
	if len(labelMap) > 0 {
		labelSelector = JoinLabelSelectors(defaultLabelSelectors(), BuildLabelSelectors(labelMap))
	}
	list, err := c.clientSet.NetworkingV1().Ingresses(c.cfg.ControllerNamespace).List(ctx,
		metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, err
	}
	ingresses := list.Items
	c.retainWatchedIngress(&ingresses)
	sortIngressByName(ingresses)
	return ingresses, nil
}

// ListIngressByDomain 对应 listIngressByDomain
func (c *Client) ListIngressByDomain(ctx context.Context, domainName string) ([]networkingv1.Ingress, error) {
	labelSelector := JoinLabelSelectors(defaultLabelSelectors(), BuildDomainLabelSelector(domainName))
	list, err := c.clientSet.NetworkingV1().Ingresses(c.cfg.ControllerNamespace).List(ctx,
		metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, err
	}
	ingresses := list.Items
	c.retainWatchedIngress(&ingresses)
	sortIngressByName(ingresses)
	return ingresses, nil
}

// ReadIngress 对应 readIngress
func (c *Client) ReadIngress(ctx context.Context, name string) (*networkingv1.Ingress, error) {
	ingress, err := c.clientSet.NetworkingV1().Ingresses(c.cfg.ControllerNamespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return ingress, nil
}

// CreateIngress 对应 createIngress
func (c *Client) CreateIngress(ctx context.Context, ingress *networkingv1.Ingress) (*networkingv1.Ingress, error) {
	c.renderDefaultMetadata(&ingress.ObjectMeta)
	c.fillDefaultIngressClass(ingress)
	return c.clientSet.NetworkingV1().Ingresses(c.cfg.ControllerNamespace).Create(ctx, ingress, metav1.CreateOptions{})
}

// ReplaceIngress 对应 replaceIngress
func (c *Client) ReplaceIngress(ctx context.Context, ingress *networkingv1.Ingress) (*networkingv1.Ingress, error) {
	if ingress.Name == "" {
		return nil, fmt.Errorf("Ingress doesn't have a valid metadata.")
	}
	c.renderDefaultMetadata(&ingress.ObjectMeta)
	c.fillDefaultIngressClass(ingress)
	return c.clientSet.NetworkingV1().Ingresses(c.cfg.ControllerNamespace).Update(ctx, ingress, metav1.UpdateOptions{})
}

// DeleteIngress 对应 deleteIngress
func (c *Client) DeleteIngress(ctx context.Context, name string) error {
	err := c.clientSet.NetworkingV1().Ingresses(c.cfg.ControllerNamespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// ---- ConfigMap ----

func (c *Client) ListConfigMap(ctx context.Context, labelMap map[string]string) ([]corev1.ConfigMap, error) {
	labelSelector := JoinLabelSelectors(defaultLabelSelectors(), BuildLabelSelectors(labelMap))
	list, err := c.clientSet.CoreV1().ConfigMaps(c.cfg.ControllerNamespace).List(ctx,
		metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, err
	}
	result := list.Items
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func (c *Client) CreateConfigMap(ctx context.Context, cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	c.renderDefaultMetadata(&cm.ObjectMeta)
	return c.clientSet.CoreV1().ConfigMaps(c.cfg.ControllerNamespace).Create(ctx, cm, metav1.CreateOptions{})
}

func (c *Client) ReadConfigMap(ctx context.Context, name string) (*corev1.ConfigMap, error) {
	cm, err := c.clientSet.CoreV1().ConfigMaps(c.cfg.ControllerNamespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return cm, nil
}

func (c *Client) DeleteConfigMap(ctx context.Context, name string) error {
	err := c.clientSet.CoreV1().ConfigMaps(c.cfg.ControllerNamespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *Client) ReplaceConfigMap(ctx context.Context, cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	if cm.Name == "" {
		return nil, fmt.Errorf("ConfigMap doesn't have a valid metadata.")
	}
	c.renderDefaultMetadata(&cm.ObjectMeta)
	return c.clientSet.CoreV1().ConfigMaps(c.cfg.ControllerNamespace).Update(ctx, cm, metav1.UpdateOptions{})
}

// ---- Secret ----

func (c *Client) ListSecret(ctx context.Context, secretType string) ([]corev1.Secret, error) {
	opts := metav1.ListOptions{}
	if secretType != "" {
		opts.FieldSelector = consts.TypeField + "=" + secretType
	}
	list, err := c.clientSet.CoreV1().Secrets(c.cfg.ControllerNamespace).List(ctx, opts)
	if err != nil {
		return nil, err
	}
	result := list.Items
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func (c *Client) ReadSecret(ctx context.Context, name string) (*corev1.Secret, error) {
	secret, err := c.clientSet.CoreV1().Secrets(c.cfg.ControllerNamespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return secret, nil
}

func (c *Client) CreateSecret(ctx context.Context, secret *corev1.Secret) (*corev1.Secret, error) {
	c.renderDefaultMetadata(&secret.ObjectMeta)
	return c.clientSet.CoreV1().Secrets(c.cfg.ControllerNamespace).Create(ctx, secret, metav1.CreateOptions{})
}

func (c *Client) ReplaceSecret(ctx context.Context, secret *corev1.Secret) (*corev1.Secret, error) {
	if secret.Name == "" {
		return nil, fmt.Errorf("Secret doesn't have a valid metadata.")
	}
	c.renderDefaultMetadata(&secret.ObjectMeta)
	return c.clientSet.CoreV1().Secrets(c.cfg.ControllerNamespace).Update(ctx, secret, metav1.UpdateOptions{})
}

func (c *Client) DeleteSecret(ctx context.Context, name string) error {
	err := c.clientSet.CoreV1().Secrets(c.cfg.ControllerNamespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// ---- McpBridge CRD ----

func (c *Client) ListMcpBridge(ctx context.Context) ([]McpBridge, error) {
	list, err := c.dynamic.Resource(mcpBridgeGVR).Namespace(c.cfg.ControllerNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var result []McpBridge
	for i := range list.Items {
		var m McpBridge
		if fromUnstructured(&list.Items[i], &m) == nil {
			result = append(result, m)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func (c *Client) CreateMcpBridge(ctx context.Context, mcpBridge *McpBridge) (*McpBridge, error) {
	c.renderDefaultMetadata(&mcpBridge.ObjectMeta)
	u, err := toUnstructured(mcpBridge)
	if err != nil {
		return nil, err
	}
	created, err := c.dynamic.Resource(mcpBridgeGVR).Namespace(c.cfg.ControllerNamespace).Create(ctx, u, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	var result McpBridge
	if err := fromUnstructured(created, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ReplaceMcpBridge(ctx context.Context, mcpBridge *McpBridge) (*McpBridge, error) {
	if mcpBridge.Name == "" {
		return nil, fmt.Errorf("mcpBridge doesn't have a valid metadata.")
	}
	mcpBridge.Namespace = c.cfg.ControllerNamespace
	u, err := toUnstructured(mcpBridge)
	if err != nil {
		return nil, err
	}
	updated, err := c.dynamic.Resource(mcpBridgeGVR).Namespace(c.cfg.ControllerNamespace).Update(ctx, u, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	var result McpBridge
	if err := fromUnstructured(updated, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteMcpBridge(ctx context.Context, name string) error {
	return c.dynamic.Resource(mcpBridgeGVR).Namespace(c.cfg.ControllerNamespace).Delete(ctx, name, metav1.DeleteOptions{})
}

func (c *Client) ReadMcpBridge(ctx context.Context, name string) (*McpBridge, error) {
	u, err := c.dynamic.Resource(mcpBridgeGVR).Namespace(c.cfg.ControllerNamespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	var result McpBridge
	if err := fromUnstructured(u, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ---- WasmPlugin CRD ----

func (c *Client) ListWasmPlugin(ctx context.Context, name, version string, builtIn *bool) ([]WasmPlugin, error) {
	labelItems := []string{defaultLabelSelectors()}
	if name != "" {
		labelItems = append(labelItems, BuildLabelSelector(consts.LabelWasmPluginNameKey, name))
	}
	if version != "" {
		labelItems = append(labelItems, BuildLabelSelector(consts.LabelWasmPluginVersionKey, version))
	}
	if builtIn != nil {
		labelItems = append(labelItems, BuildLabelSelector(consts.LabelWasmPluginBuiltInKey, fmt.Sprintf("%v", *builtIn)))
	}
	labelSelector := JoinLabelSelectors(labelItems...)

	list, err := c.dynamic.Resource(wasmPluginGVR).Namespace(c.cfg.ControllerNamespace).List(ctx,
		metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, err
	}
	var result []WasmPlugin
	for i := range list.Items {
		var w WasmPlugin
		if fromUnstructured(&list.Items[i], &w) == nil {
			result = append(result, w)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func (c *Client) CreateWasmPlugin(ctx context.Context, plugin *WasmPlugin) (*WasmPlugin, error) {
	c.renderDefaultMetadata(&plugin.ObjectMeta)
	u, err := toUnstructured(plugin)
	if err != nil {
		return nil, err
	}
	created, err := c.dynamic.Resource(wasmPluginGVR).Namespace(c.cfg.ControllerNamespace).Create(ctx, u, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	var result WasmPlugin
	if err := fromUnstructured(created, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ReplaceWasmPlugin(ctx context.Context, plugin *WasmPlugin) (*WasmPlugin, error) {
	if plugin.Name == "" {
		return nil, fmt.Errorf("WasmPlugin doesn't have a valid metadata.")
	}
	c.renderDefaultMetadata(&plugin.ObjectMeta)
	u, err := toUnstructured(plugin)
	if err != nil {
		return nil, err
	}
	updated, err := c.dynamic.Resource(wasmPluginGVR).Namespace(c.cfg.ControllerNamespace).Update(ctx, u, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	var result WasmPlugin
	if err := fromUnstructured(updated, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteWasmPlugin(ctx context.Context, name string) error {
	err := c.dynamic.Resource(wasmPluginGVR).Namespace(c.cfg.ControllerNamespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *Client) ReadWasmPlugin(ctx context.Context, name string) (*WasmPlugin, error) {
	u, err := c.dynamic.Resource(wasmPluginGVR).Namespace(c.cfg.ControllerNamespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	var result WasmPlugin
	if err := fromUnstructured(u, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ---- EnvoyFilter CRD (unstructured) ----

func (c *Client) CreateEnvoyFilter(ctx context.Context, filter *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	c.renderDefaultUnstructuredMetadata(filter)
	return c.dynamic.Resource(envoyFilterGVR).Namespace(c.cfg.ControllerNamespace).Create(ctx, filter, metav1.CreateOptions{})
}

func (c *Client) ReplaceEnvoyFilter(ctx context.Context, filter *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	c.renderDefaultUnstructuredMetadata(filter)
	return c.dynamic.Resource(envoyFilterGVR).Namespace(c.cfg.ControllerNamespace).Update(ctx, filter, metav1.UpdateOptions{})
}

func (c *Client) DeleteEnvoyFilter(ctx context.Context, name string) error {
	err := c.dynamic.Resource(envoyFilterGVR).Namespace(c.cfg.ControllerNamespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *Client) ReadEnvoyFilter(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	u, err := c.dynamic.Resource(envoyFilterGVR).Namespace(c.cfg.ControllerNamespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

// ---- helpers ----

func (c *Client) renderDefaultMetadata(meta *metav1.ObjectMeta) {
	if meta == nil {
		return
	}
	SetLabel(meta, consts.LabelResourceDefinerKey, consts.LabelResourceDefinerValue)
	if IsInternalResource(meta.Name) {
		SetLabel(meta, consts.LabelInternalKey, "true")
		SetAnnotation(meta, consts.AnnotationCommentKey, consts.InternalResourceComment)
	}
}

func (c *Client) renderDefaultUnstructuredMetadata(u *unstructured.Unstructured) {
	labels := u.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[consts.LabelResourceDefinerKey] = consts.LabelResourceDefinerValue
	if IsInternalResource(u.GetName()) {
		labels[consts.LabelInternalKey] = "true"
		annotations := u.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[consts.AnnotationCommentKey] = consts.InternalResourceComment
		u.SetAnnotations(annotations)
	}
	u.SetLabels(labels)
}

func (c *Client) retainWatchedIngress(ingresses *[]networkingv1.Ingress) {
	filtered := (*ingresses)[:0]
	for _, ing := range *ingresses {
		if c.isIngressWatched(&ing) {
			filtered = append(filtered, ing)
		}
	}
	*ingresses = filtered
}

func (c *Client) fillDefaultIngressClass(ingress *networkingv1.Ingress) {
	if ingress.Spec.IngressClassName == nil {
		ingress.Spec.IngressClassName = &c.defaultIngressClass
	}
}

func defaultLabelSelectors() string {
	return BuildLabelSelector(consts.LabelResourceDefinerKey, consts.LabelResourceDefinerValue)
}

func buildIsIngressWatchedPredicate(controllerWatchedIngressClassName string) func(*networkingv1.Ingress) bool {
	if controllerWatchedIngressClassName == "" {
		return func(*networkingv1.Ingress) bool { return true }
	}
	if controllerWatchedIngressClassName == consts.NginxIngressClassName {
		return func(ingress *networkingv1.Ingress) bool {
			class := getIngressClassName(ingress)
			return class == "" || class == consts.NginxIngressClassName
		}
	}
	return func(ingress *networkingv1.Ingress) bool {
		return controllerWatchedIngressClassName == getIngressClassName(ingress)
	}
}

func getIngressClassName(ingress *networkingv1.Ingress) string {
	if ingress.Spec.IngressClassName == nil {
		return ""
	}
	return *ingress.Spec.IngressClassName
}

func sortIngressByName(ingresses []networkingv1.Ingress) {
	sort.Slice(ingresses, func(i, j int) bool { return ingresses[i].Name < ingresses[j].Name })
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func toUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{}
	if err := u.UnmarshalJSON(data); err != nil {
		return nil, err
	}
	return u, nil
}

func fromUnstructured(u *unstructured.Unstructured, obj interface{}) error {
	data, err := u.MarshalJSON()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, obj)
}
