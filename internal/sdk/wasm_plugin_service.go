package sdk

import (
	"context"
	"encoding/base64"
	"os"
	"sort"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/yaml"

	"console/internal/consts"
	"console/internal/errs"
	k8s "console/internal/kubernetes"
	"console/internal/model"
	"console/resource"
)

const (
	pluginsResourceFolder = "plugins/"
	pluginsPropertiesFile = pluginsResourceFolder + "plugins.properties"
	specFile              = "spec.yaml"
	readmeFile            = "README.md"
	readmeCnFile          = "README_CN.md"
	readmeEnFile          = "README_EN.md"
	iconFile              = "icon.png"
	iconDataPrefix        = "data:image/png;base64,"
	defaultPluginVersion  = "1.0.0"
	defaultImageRegistry  = "higress-registry.cn-hangzhou.cr.aliyuncs.com"
	defaultImageNamespace = "plugins"

	namePlaceholder    = "${name}"
	versionPlaceholder = "${version}"
	exampleRawPropName = "x-example-raw"
	defaultReadmeKey   = "_default_"
)

// wasmPluginServiceConfig 对应 WasmPluginServiceConfig
type wasmPluginServiceConfig struct {
	customImageUrlPattern string
	pluginImageRegistry   string
	pluginImageNamespace  string
	imagePullSecret       string
	imagePullPolicy       string
}

func newWasmPluginServiceConfig() *wasmPluginServiceConfig {
	return &wasmPluginServiceConfig{
		customImageUrlPattern: os.Getenv("HIGRESS_ADMIN_WASM_PLUGIN_CUSTOM_IMAGE_URL_PATTERN"),
		pluginImageRegistry:   os.Getenv("HIGRESS_ADMIN_WASM_PLUGIN_IMAGE_REGISTRY"),
		pluginImageNamespace:  os.Getenv("HIGRESS_ADMIN_WASM_PLUGIN_IMAGE_NAMESPACE"),
		imagePullSecret:       os.Getenv("HIGRESS_ADMIN_WASM_PLUGIN_IMAGE_PULL_SECRET"),
		imagePullPolicy:       os.Getenv("HIGRESS_ADMIN_WASM_PLUGIN_IMAGE_PULL_POLICY"),
	}
}

// WasmPluginService 对应 WasmPluginService 接口
type WasmPluginService interface {
	List(query *model.WasmPluginPageQuery) *model.PaginatedResult[model.WasmPlugin]
	Query(name, language string) *model.WasmPlugin
	QueryConfig(name, language string) *model.WasmPluginConfig
	QueryReadme(name, language string) string
	UpdateBuiltIn(plugin *model.WasmPlugin) *model.WasmPlugin
	AddCustom(plugin *model.WasmPlugin) *model.WasmPlugin
	UpdateCustom(plugin *model.WasmPlugin) *model.WasmPlugin
	DeleteCustom(name string)
}

// WasmPluginServiceImpl 对应 WasmPluginServiceImpl
type WasmPluginServiceImpl struct {
	client         *k8s.Client
	converter      *Converter
	config         *wasmPluginServiceConfig
	builtInPlugins []*wasmPluginCacheItem
}

// NewWasmPluginService 创建 WasmPluginService
func NewWasmPluginService(client *k8s.Client, converter *Converter) WasmPluginService {
	s := &WasmPluginServiceImpl{
		client:    client,
		converter: converter,
		config:    newWasmPluginServiceConfig(),
	}
	s.initialize()
	return s
}

func (s *WasmPluginServiceImpl) initialize() {
	props, err := loadPluginsProperties()
	if err != nil {
		panic(errs.Internal("Error occurs when loading built-in plugin list: " + err.Error()))
	}

	plugins := make([]*wasmPluginCacheItem, 0, len(props))
	for name, imageUrl := range props {
		if strings.TrimSpace(imageUrl) == "" {
			continue
		}
		item := &wasmPluginCacheItem{name: name, readmes: map[string]string{}}

		folder := pluginsResourceFolder + name + "/"
		specContent, err := resource.PluginsFS.ReadFile(folder + specFile)
		if err != nil {
			// No spec. Ignore it.
			continue
		}
		var pf wasmPluginFile
		if err := yaml.Unmarshal(specContent, &pf); err != nil {
			panic(errs.Internal("Error occurs when loading spec file of plugin " + name + ": " + err.Error()))
		}
		fillPluginConfigExample(&pf, string(specContent))
		item.plugin = &pf

		item.imageUrl = buildPluginImageUrl(imageUrl, s.config.customImageUrlPattern,
			s.config.pluginImageRegistry, s.config.pluginImageNamespace, pf.Info)
		item.imagePullSecret = s.config.imagePullSecret
		item.imagePullPolicy = s.config.imagePullPolicy

		item.defaultReadme = loadPluginReadme(name, readmeFile)
		item.setReadme(consts.LanguageZhCN, loadPluginReadme(name, readmeCnFile))
		item.setReadme(consts.LanguageEnUS, loadPluginReadme(name, readmeEnFile))

		if iconRaw, err := resource.PluginsFS.ReadFile(folder + iconFile); err == nil {
			item.iconData = iconDataPrefix + base64.StdEncoding.EncodeToString(iconRaw)
		}

		plugins = append(plugins, item)
	}

	sort.Slice(plugins, func(i, j int) bool { return plugins[i].name < plugins[j].name })
	s.builtInPlugins = plugins
}

func loadPluginsProperties() (map[string]string, error) {
	content, err := resource.PluginsFS.ReadFile(pluginsPropertiesFile)
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		if key != "" {
			result[key] = value
		}
	}
	return result, nil
}

func loadPluginReadme(pluginName, fileName string) string {
	content, err := resource.PluginsFS.ReadFile(pluginsResourceFolder + pluginName + "/" + fileName)
	if err != nil {
		return ""
	}
	return string(content)
}

func formatImageUrl(pattern string, info wasmPluginFileInfo) string {
	if pattern == "" {
		return pattern
	}
	return strings.ReplaceAll(strings.ReplaceAll(pattern, namePlaceholder, info.Name),
		versionPlaceholder, info.Version)
}

func buildPluginImageUrl(defaultUrl, customPattern, registry, namespace string, info wasmPluginFileInfo) string {
	if strings.TrimSpace(customPattern) != "" {
		return formatImageUrl(customPattern, info)
	}
	if strings.TrimSpace(registry) == "" && strings.TrimSpace(namespace) == "" {
		return defaultUrl
	}
	if !strings.HasPrefix(defaultUrl, consts.OciProtocol) {
		return defaultUrl
	}
	urlWithoutProtocol := strings.TrimPrefix(defaultUrl, consts.OciProtocol)
	targetRegistry := registry
	if strings.TrimSpace(targetRegistry) == "" {
		targetRegistry = defaultImageRegistry
	}
	targetNamespace := namespace
	if strings.TrimSpace(targetNamespace) == "" {
		targetNamespace = defaultImageNamespace
	}
	firstSlash := strings.Index(urlWithoutProtocol, "/")
	if firstSlash == -1 {
		return defaultUrl
	}
	secondSlash := strings.Index(urlWithoutProtocol[firstSlash+1:], "/")
	if secondSlash == -1 {
		return defaultUrl
	}
	secondSlash = firstSlash + 1 + secondSlash
	pluginPath := urlWithoutProtocol[secondSlash+1:]
	return consts.OciProtocol + targetRegistry + "/" + targetNamespace + "/" + pluginPath
}

func (s *WasmPluginServiceImpl) List(query *model.WasmPluginPageQuery) *model.PaginatedResult[model.WasmPlugin] {
	lang := ""
	if query != nil {
		lang = deref(query.Lang)
	}
	plugins := make([]model.WasmPlugin, 0, len(s.builtInPlugins))
	for _, item := range s.builtInPlugins {
		plugins = append(plugins, *item.buildWasmPlugin(lang))
	}

	crs, err := s.client.ListWasmPlugin(context.Background(), "", "", nil)
	if err != nil {
		panic(errs.Business("Error occurs when listing custom Wasm plugins"))
	}
	for i := range crs {
		plugin := s.converter.WasmPluginFromCr(&crs[i])
		if boolValue(plugin.BuiltIn) {
			idx := -1
			for j := range plugins {
				if deref(plugins[j].Name) == deref(plugin.Name) {
					idx = j
					break
				}
			}
			if idx >= 0 {
				plugins[idx].ImageRepository = plugin.ImageRepository
				plugins[idx].ImageVersion = plugin.ImageVersion
				plugins[idx].Phase = plugin.Phase
				plugins[idx].Priority = plugin.Priority
				plugins[idx].ImagePullPolicy = plugin.ImagePullPolicy
				plugins[idx].ImagePullSecret = plugin.ImagePullSecret
				continue
			}
		}
		plugins = append(plugins, *plugin)
	}
	return model.CreateFromFullList(plugins, &query.CommonPageQuery)
}

func (s *WasmPluginServiceImpl) Query(name, language string) *model.WasmPlugin {
	if name == "" {
		return nil
	}
	for _, item := range s.builtInPlugins {
		if item.name == name {
			return item.buildWasmPlugin(language)
		}
	}
	crs, err := s.client.ListWasmPlugin(context.Background(), name, "", boolPtr(false))
	if err != nil {
		panic(errs.Business("Error occurs when checking existed Wasm plugins with name " + name))
	}
	if len(crs) == 0 {
		return nil
	}
	topIdx := 0
	topPriority := 0
	for i := range crs {
		p := 0
		if crs[i].Spec.Priority != nil {
			p = *crs[i].Spec.Priority
		}
		if i == 0 || p > topPriority {
			topPriority = p
			topIdx = i
		}
	}
	return s.converter.WasmPluginFromCr(&crs[topIdx])
}

func (s *WasmPluginServiceImpl) QueryConfig(name, language string) *model.WasmPluginConfig {
	if name == "" {
		return nil
	}
	for _, item := range s.builtInPlugins {
		if item.name == name {
			return item.buildWasmPluginConfig(language)
		}
	}
	crs, err := s.client.ListWasmPlugin(context.Background(), name, "", boolPtr(false))
	if err != nil {
		panic(errs.Business("Error occurs when checking existed Wasm plugins with name " + name))
	}
	if len(crs) > 0 {
		// TODO: Config of a custom plugin is not supported yet. Return an empty schema instead.
		return &model.WasmPluginConfig{Schema: map[string]any{"type": "object"}}
	}
	return nil
}

func (s *WasmPluginServiceImpl) QueryReadme(name, language string) string {
	if name == "" {
		return ""
	}
	for _, item := range s.builtInPlugins {
		if item.name == name {
			content := ""
			if language != "" {
				content = item.readmes[language]
			}
			if content == "" {
				content = item.defaultReadme
			}
			return content
		}
	}
	crs, err := s.client.ListWasmPlugin(context.Background(), name, "", boolPtr(false))
	if err != nil {
		panic(errs.Business("Error occurs when checking existed Wasm plugins with name " + name))
	}
	if len(crs) > 0 {
		// TODO: Readme of a custom plugin is not supported yet.
		return ""
	}
	return ""
}

func (s *WasmPluginServiceImpl) UpdateBuiltIn(plugin *model.WasmPlugin) *model.WasmPlugin {
	name := deref(plugin.Name)

	var cachedBuiltInPlugin *wasmPluginCacheItem
	for _, item := range s.builtInPlugins {
		if item.name == name {
			cachedBuiltInPlugin = item
			break
		}
	}
	if cachedBuiltInPlugin == nil {
		panic(errs.Conflict("No built-in plugin is found with the given name: " + name))
	}

	pluginVersion := cachedBuiltInPlugin.plugin.Info.Version
	existedCrs, err := s.client.ListWasmPlugin(context.Background(), name, pluginVersion, boolPtr(true))
	if err != nil {
		panic(errs.Business("Error occurs when checking existed Wasm plugins with name " + name))
	}

	var updatedCr *k8s.WasmPlugin

	allInternal := true
	for i := range existedCrs {
		if !k8s.IsInternalResource(existedCrs[i].Name) {
			allInternal = false
			break
		}
	}
	if allInternal {
		builtInPlugin := cachedBuiltInPlugin.buildWasmPlugin("")
		builtInPlugin.ImageRepository = plugin.ImageRepository
		builtInPlugin.ImageVersion = plugin.ImageVersion
		builtInPlugin.Phase = plugin.Phase
		builtInPlugin.Priority = plugin.Priority
		builtInPlugin.ImagePullPolicy = plugin.ImagePullPolicy
		builtInPlugin.ImagePullSecret = plugin.ImagePullSecret
		cr := s.converter.WasmPluginToCr(builtInPlugin)
		// Make sure it is disabled by default.
		cr.Spec.DefaultConfigDisable = boolPtr(true)
		created, err := s.client.CreateWasmPlugin(context.Background(), cr)
		if err != nil {
			if apierrors.IsConflict(err) {
				panic(errs.Conflict(""))
			}
			panic(errs.Business("Error occurs when adding a new Wasm plugin with name: " + cr.Name))
		}
		updatedCr = created
	}

	for i := range existedCrs {
		existedCr := &existedCrs[i]
		imageUrl := k8s.ImageUrl{Repository: deref(plugin.ImageRepository), Tag: deref(plugin.ImageVersion)}
		existedCr.Spec.URL = imageUrl.ToUrlString()
		existedCr.Spec.Phase = pluginPhaseFromName(deref(plugin.Phase))
		existedCr.Spec.Priority = plugin.Priority
		existedCr.Spec.ImagePullPolicy = imagePullPolicyFromName(deref(plugin.ImagePullPolicy))
		existedCr.Spec.ImagePullSecret = deref(plugin.ImagePullSecret)
		replaced, err := s.client.ReplaceWasmPlugin(context.Background(), existedCr)
		if err != nil {
			if apierrors.IsConflict(err) {
				panic(errs.Conflict(""))
			}
			panic(errs.Business("Error occurs when updating the Wasm plugin wth name " + existedCr.Name))
		}
		updatedCr = replaced
	}

	if updatedCr == nil {
		panic(errs.Internal("No Wasm plugin CR was updated for built-in plugin: " + name))
	}
	return s.converter.WasmPluginFromCr(updatedCr)
}

func (s *WasmPluginServiceImpl) AddCustom(plugin *model.WasmPlugin) *model.WasmPlugin {
	if boolValue(plugin.BuiltIn) {
		panic(errs.Conflict("Adding a built-in plugin is not allowed."))
	}
	for _, item := range s.builtInPlugins {
		if item.name == deref(plugin.Name) {
			panic(errs.Conflict("Name conflicted with a built-in plugin."))
		}
	}
	if deref(plugin.PluginVersion) == "" {
		plugin.PluginVersion = strPtr(defaultPluginVersion)
	}
	plugin.BuiltIn = boolPtr(false)
	plugin.Category = strPtr(consts.PluginCategoryCustom)

	cr := s.converter.WasmPluginToCr(plugin)
	// Make sure it is disabled by default.
	cr.Spec.DefaultConfigDisable = boolPtr(true)
	addedCr, err := s.client.CreateWasmPlugin(context.Background(), cr)
	if err != nil {
		if apierrors.IsConflict(err) {
			panic(errs.Conflict(""))
		}
		panic(errs.Business("Error occurs when adding a new Wasm plugin."))
	}
	return s.converter.WasmPluginFromCr(addedCr)
}

func (s *WasmPluginServiceImpl) UpdateCustom(plugin *model.WasmPlugin) *model.WasmPlugin {
	name := deref(plugin.Name)

	if boolValue(plugin.BuiltIn) {
		panic(errs.Conflict("Updating a built-in plugin is not allowed."))
	}
	for _, item := range s.builtInPlugins {
		if item.name == name {
			panic(errs.Conflict("Updating a built-in plugin is not allowed."))
		}
	}
	if deref(plugin.PluginVersion) == "" {
		plugin.PluginVersion = strPtr(defaultPluginVersion)
	}
	plugin.BuiltIn = boolPtr(false)
	plugin.Category = strPtr(consts.PluginCategoryCustom)

	cr := s.converter.WasmPluginToCr(plugin)
	existedCrs, err := s.client.ListWasmPlugin(context.Background(), name, "", boolPtr(false))
	if err != nil {
		panic(errs.Business("Error occurs when checking existed Wasm plugins with name " + name))
	}
	if len(existedCrs) == 0 {
		panic(errs.NotFound("No Wasm plugin with name \"" + name + "\" is found."))
	}

	crName := cr.Name
	var existedCr *k8s.WasmPlugin
	for i := range existedCrs {
		if existedCrs[i].Name == crName {
			existedCr = &existedCrs[i]
			break
		}
	}
	if existedCr != nil {
		s.converter.MergeWasmPluginSpec(existedCr, cr)
		resourceVersion := deref(plugin.Version)
		if resourceVersion == "" {
			resourceVersion = existedCr.ResourceVersion
		}
		cr.ResourceVersion = resourceVersion

		updatedCr, err := s.client.ReplaceWasmPlugin(context.Background(), cr)
		if err != nil {
			if apierrors.IsConflict(err) {
				panic(errs.Conflict(""))
			}
			panic(errs.Business("Error occurs when updating the Wasm plugin wth name " + crName))
		}
		return s.converter.WasmPluginFromCr(updatedCr)
	}

	sort.Slice(existedCrs, func(i, j int) bool {
		pi, pj := 0, 0
		if existedCrs[i].Spec.Priority != nil {
			pi = *existedCrs[i].Spec.Priority
		}
		if existedCrs[j].Spec.Priority != nil {
			pj = *existedCrs[j].Spec.Priority
		}
		return pi < pj
	})
	for i := range existedCrs {
		s.converter.MergeWasmPluginSpec(&existedCrs[i], cr)
	}

	updatedCr, err := s.client.CreateWasmPlugin(context.Background(), cr)
	if err != nil {
		if apierrors.IsConflict(err) {
			panic(errs.Conflict(""))
		}
		panic(errs.Business("Error occurs when adding the Wasm plugin CR wth name " + crName))
	}

	for i := range existedCrs {
		ecrName := existedCrs[i].Name
		if ecrName == crName {
			continue
		}
		if err := s.client.DeleteWasmPlugin(context.Background(), ecrName); err != nil {
			panic(errs.Business("Error occurs when deleting the Wasm plugin CR wth name " + ecrName))
		}
	}

	return s.converter.WasmPluginFromCr(updatedCr)
}

func (s *WasmPluginServiceImpl) DeleteCustom(name string) {
	for _, item := range s.builtInPlugins {
		if item.name == name {
			panic(errs.Conflict("Deleting a built-in plugin is not allowed."))
		}
	}
	crs, err := s.client.ListWasmPlugin(context.Background(), name, "", boolPtr(false))
	if err != nil {
		panic(errs.Business("Error occurs when loading Wasm plugins with name " + name))
	}
	if len(crs) == 0 {
		return
	}
	for i := range crs {
		crName := crs[i].Name
		if err := s.client.DeleteWasmPlugin(context.Background(), crName); err != nil {
			panic(errs.Business("Error occurs when deleting the Wasm plugin CR wth name " + crName))
		}
	}
}

// ---- 内置插件缓存 ----

type wasmPluginCacheItem struct {
	name            string
	imageUrl        string
	imagePullPolicy string
	imagePullSecret string
	plugin          *wasmPluginFile
	iconData        string
	defaultReadme   string
	readmes         map[string]string
}

func (item *wasmPluginCacheItem) setReadme(language, content string) {
	if strings.TrimSpace(content) != "" {
		item.readmes[language] = content
	}
}

func isProductCoveredPlugin(pluginName string) bool {
	switch pluginName {
	case consts.ProductCoveredAiProxy, consts.ProductCoveredModelRouter, consts.ProductCoveredModelMapper,
		consts.ProductCoveredMcpServer, consts.ProductCoveredKeyAuth:
		return true
	}
	return false
}

func (item *wasmPluginCacheItem) buildWasmPlugin(language string) *model.WasmPlugin {
	wasmPlugin := &model.WasmPlugin{}
	wasmPlugin.Name = strPtr(item.name)
	imageUrl := k8s.ParseImageUrl(item.imageUrl)
	wasmPlugin.ImageRepository = strPtr(imageUrl.Repository)
	wasmPlugin.ImageVersion = strPtr(imageUrl.Tag)
	wasmPlugin.ImagePullSecret = strPtr(item.imagePullSecret)
	wasmPlugin.ImagePullPolicy = strPtr(item.imagePullPolicy)
	wasmPlugin.BuiltIn = boolPtr(true)
	wasmPlugin.ProductCovered = boolPtr(isProductCoveredPlugin(item.name))

	info := item.plugin.Info
	wasmPlugin.Category = strPtr(info.Category)
	wasmPlugin.PluginVersion = strPtr(info.Version)
	wasmPlugin.Icon = strPtr(info.IconUrl)

	if language == "" {
		wasmPlugin.Title = strPtr(info.Title)
		wasmPlugin.Description = strPtr(info.Description)
		wasmPlugin.ReadmeUrl = strPtr(info.ReadmeUrl)
	} else {
		wasmPlugin.Title = strPtr(stringOr(info.TitleI18n[language], info.Title))
		wasmPlugin.Description = strPtr(stringOr(info.DescriptionI18n[language], info.Description))
		wasmPlugin.ReadmeUrl = strPtr(stringOr(info.ReadmeUrlI18n[language], info.ReadmeUrl))
	}

	if item.plugin.Spec.Phase == "default" {
		wasmPlugin.Phase = strPtr(consts.PluginPhaseUnspecified)
	} else {
		wasmPlugin.Phase = strPtr(item.plugin.Spec.Phase)
	}
	wasmPlugin.Priority = item.plugin.Spec.Priority

	if item.iconData != "" {
		wasmPlugin.Icon = strPtr(item.iconData)
	}
	return wasmPlugin
}

func (item *wasmPluginCacheItem) buildWasmPluginConfig(language string) *model.WasmPluginConfig {
	if item.plugin.Spec.ConfigSchema == nil || item.plugin.Spec.ConfigSchema.OpenApiV3Schema == nil {
		return &model.WasmPluginConfig{}
	}
	schema := deepCopyMap(item.plugin.Spec.ConfigSchema.OpenApiV3Schema)
	applyI18nResources(schema, language)
	return &model.WasmPluginConfig{Schema: schema}
}

func stringOr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func deepCopyMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		switch vv := v.(type) {
		case map[string]any:
			dst[k] = deepCopyMap(vv)
		case []any:
			cp := make([]any, len(vv))
			for i, item := range vv {
				if m, ok := item.(map[string]any); ok {
					cp[i] = deepCopyMap(m)
				} else {
					cp[i] = item
				}
			}
			dst[k] = cp
		default:
			dst[k] = v
		}
	}
	return dst
}

// applyI18nResources 对应 applyI18nResources
func applyI18nResources(schema map[string]any, language string) {
	if len(schema) == 0 {
		return
	}
	var keysToDelete []string
	var fieldsToSet map[string]any
	for key, value := range schema {
		if fieldName, ok := i18nFieldName(key); ok {
			keysToDelete = append(keysToDelete, key)
			if i18nMap, ok := value.(map[string]any); ok {
				if v, exists := i18nMap[language]; exists {
					if fieldsToSet == nil {
						fieldsToSet = map[string]any{}
					}
					fieldsToSet[fieldName] = v
				}
			}
		}
	}
	for _, key := range keysToDelete {
		delete(schema, key)
	}
	for field, v := range fieldsToSet {
		schema[field] = v
	}

	for _, value := range schema {
		switch v := value.(type) {
		case map[string]any:
			applyI18nResources(v, language)
		case []any:
			for _, item := range v {
				if m, ok := item.(map[string]any); ok {
					applyI18nResources(m, language)
				}
			}
		}
	}
}

func i18nFieldName(key string) (string, bool) {
	const prefix = "x-"
	const suffix = "-i18n"
	if !strings.HasPrefix(key, prefix) || !strings.HasSuffix(key, suffix) {
		return "", false
	}
	field := key[len(prefix) : len(key)-len(suffix)]
	if field == "" {
		return "", false
	}
	return field, true
}

// ---- spec.yaml 模型 ----

type wasmPluginFile struct {
	ApiVersion string             `json:"apiVersion"`
	Info       wasmPluginFileInfo `json:"info"`
	Spec       wasmPluginFileSpec `json:"spec"`
}

type wasmPluginFileInfo struct {
	Category        string            `json:"category"`
	Name            string            `json:"name"`
	Title           string            `json:"title"`
	TitleI18n       map[string]string `json:"x-title-i18n"`
	Description     string            `json:"description"`
	DescriptionI18n map[string]string `json:"x-description-i18n"`
	IconUrl         string            `json:"iconUrl"`
	ReadmeUrl       string            `json:"readmeUrl"`
	ReadmeUrlI18n   map[string]string `json:"x-readmeUrl-i18n"`
	Version         string            `json:"version"`
}

type wasmPluginFileSpec struct {
	Phase        string                  `json:"phase"`
	Priority     *int                    `json:"priority"`
	ConfigSchema *wasmPluginConfigSchema `json:"configSchema"`
}

type wasmPluginConfigSchema struct {
	OpenApiV3Schema map[string]any `json:"openAPIV3Schema"`
}

// fillPluginConfigExample 对应 fillPluginConfigExample
func fillPluginConfigExample(plugin *wasmPluginFile, content string) {
	example := extractConfigExample(content)
	if plugin.Spec.ConfigSchema == nil || plugin.Spec.ConfigSchema.OpenApiV3Schema == nil {
		return
	}
	if example == "" {
		return
	}
	plugin.Spec.ConfigSchema.OpenApiV3Schema[exampleRawPropName] = example
}

// extractConfigExample 对应 extractConfigExample
func extractConfigExample(content string) string {
	var builder strings.Builder
	foundConfigSchema := false
	foundOpenApiV3Schema := false
	foundExample := false
	var schemaOuterIndentation string
	var exampleOuterIndentation string
	var exampleInnerIndentation string

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		indentation, unindentedContent, ok := splitYamlLine(line)
		if !ok {
			continue
		}

		if !foundOpenApiV3Schema {
			if !foundConfigSchema {
				if strings.HasPrefix(unindentedContent, "configSchema:") {
					foundConfigSchema = true
				}
				continue
			}
			if strings.HasPrefix(unindentedContent, "openAPIV3Schema:") {
				foundOpenApiV3Schema = true
				schemaOuterIndentation = indentation
			}
			continue
		}

		if !foundExample {
			if len(indentation) <= len(schemaOuterIndentation) {
				break
			}
			if exampleOuterIndentation == "" {
				exampleOuterIndentation = indentation
			}
			if indentation == exampleOuterIndentation && strings.HasPrefix(unindentedContent, "example:") {
				foundExample = true
			}
			continue
		}

		if len(indentation) <= len(exampleOuterIndentation) {
			break
		}
		if exampleInnerIndentation == "" {
			exampleInnerIndentation = indentation
		}
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(line[len(exampleInnerIndentation):])
	}
	return builder.String()
}

// splitYamlLine 返回缩进和去除缩进后的内容
func splitYamlLine(line string) (indentation, content string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", "", false
	}
	indentLen := len(line) - len(strings.TrimLeft(line, " \t"))
	return line[:indentLen], strings.TrimLeft(line, " \t"), true
}
