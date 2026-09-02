package sdk

import (
	"context"
	"strconv"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/yaml"

	"console/internal/consts"
	"console/internal/errs"
	k8s "console/internal/kubernetes"
	"console/internal/model"
)

const (
	maxUpdateAttempts     = 3
	updateRetryDelayNanos = 10 * time.Millisecond
)

// WasmPluginInstanceService 对应 Java 的 WasmPluginInstanceServiceImpl
type WasmPluginInstanceService struct {
	wasmPluginService WasmPluginService
	client            *k8s.Client
	converter         *Converter
}

// NewWasmPluginInstanceService 创建 WasmPluginInstanceService
func NewWasmPluginInstanceService(wasmPluginService WasmPluginService, client *k8s.Client,
	converter *Converter) *WasmPluginInstanceService {
	return &WasmPluginInstanceService{
		wasmPluginService: wasmPluginService,
		client:            client,
		converter:         converter,
	}
}

// CreateEmptyInstance 对应 createEmptyInstance
func (s *WasmPluginInstanceService) CreateEmptyInstance(pluginName string) *model.WasmPluginInstance {
	plugin := s.wasmPluginService.Query(pluginName, "")
	if plugin == nil {
		panic(errs.Business("Plugin " + pluginName + " not found"))
	}
	instance := &model.WasmPluginInstance{}
	instance.PluginName = plugin.Name
	instance.PluginVersion = plugin.PluginVersion
	return instance
}

// List 对应 list(pluginName, internal)
func (s *WasmPluginInstanceService) List(pluginName string, internal *bool) []*model.WasmPluginInstance {
	plugins, err := s.client.ListWasmPlugin(context.Background(), pluginName, "", nil)
	if err != nil {
		panic(errs.Business("Error occurs when listing WasmPlugin."))
	}
	if len(plugins) == 0 {
		return []*model.WasmPluginInstance{}
	}
	var instances []*model.WasmPluginInstance
	for i := range plugins {
		if internal != nil && *internal != k8s.IsInternalResource(plugins[i].Name) {
			continue
		}
		instances = append(instances, s.converter.GetWasmPluginInstancesFromCr(&plugins[i])...)
	}
	return instances
}

// ListByScope 对应 list(scope, target)
func (s *WasmPluginInstanceService) ListByScope(scope model.WasmPluginInstanceScope, target string) []*model.WasmPluginInstance {
	plugins, err := s.client.ListWasmPlugin(context.Background(), "", "", nil)
	if err != nil {
		panic(errs.Business("Error occurs when listing WasmPlugin."))
	}
	if len(plugins) == 0 {
		return []*model.WasmPluginInstance{}
	}
	var instances []*model.WasmPluginInstance
	for i := range plugins {
		instance := s.converter.GetWasmPluginInstanceFromCrByScope(&plugins[i], scope, target)
		if instance != nil {
			instances = append(instances, instance)
		}
	}
	return instances
}

// Query 对应 query(scope, target, pluginName, internal)
func (s *WasmPluginInstanceService) Query(scope model.WasmPluginInstanceScope, target, pluginName string,
	internal *bool) *model.WasmPluginInstance {
	return s.QueryByTargets(map[model.WasmPluginInstanceScope]*string{scope: optStrPtr(target)}, pluginName, internal)
}

// QueryByTargets 对应 query(targets, pluginName, internal)
func (s *WasmPluginInstanceService) QueryByTargets(targets map[model.WasmPluginInstanceScope]*string,
	pluginName string, internal *bool) *model.WasmPluginInstance {
	plugins, err := s.client.ListWasmPlugin(context.Background(), pluginName, "", nil)
	if err != nil {
		panic(errs.Business("Error occurs when querying WasmPlugin."))
	}
	var instance *model.WasmPluginInstance
	for i := range plugins {
		if internal != nil && *internal != k8s.IsInternalResource(plugins[i].Name) {
			continue
		}
		instance = s.converter.GetWasmPluginInstanceFromCr(&plugins[i], targets)
		if instance != nil {
			break
		}
	}
	return instance
}

// AddOrUpdate 对应 addOrUpdate
func (s *WasmPluginInstanceService) AddOrUpdate(instance *model.WasmPluginInstance) *model.WasmPluginInstance {
	updatedInstances := s.AddOrUpdateAll([]*model.WasmPluginInstance{instance})
	if len(updatedInstances) == 0 {
		panic(errs.Business("No instances were updated for plugin: " + deref(instance.PluginName)))
	}
	if len(updatedInstances) > 1 {
		panic(errs.Business("Expected only one instance to be updated, but got: " +
			strconv.Itoa(len(updatedInstances))))
	}
	return updatedInstances[0]
}

// AddOrUpdateAll 对应 addOrUpdateAll
func (s *WasmPluginInstanceService) AddOrUpdateAll(instances []*model.WasmPluginInstance) []*model.WasmPluginInstance {
	type groupKey struct {
		pluginName    string
		pluginVersion string
		internal      bool
	}
	pluginsToUpdate := map[string]*model.WasmPlugin{}
	groupedInstances := map[groupKey][]*model.WasmPluginInstance{}
	var order []groupKey

	for _, instance := range instances {
		instance.SyncDeprecatedFields()
		if err := instance.Validate(); err != nil {
			panic(errs.Business(err.Error()))
		}

		pluginName := deref(instance.PluginName)
		plugin, ok := pluginsToUpdate[pluginName]
		if !ok {
			plugin = s.wasmPluginService.Query(pluginName, "")
			pluginsToUpdate[pluginName] = plugin
		}
		if plugin == nil {
			panic(errs.Business("Unknown plugin: " + deref(instance.PluginName)))
		}

		version := deref(instance.PluginVersion)
		if version == "" {
			version = deref(plugin.PluginVersion)
		}
		key := groupKey{pluginName: pluginName, pluginVersion: version, internal: instance.IsInternal()}
		if _, exists := groupedInstances[key]; !exists {
			order = append(order, key)
		}
		groupedInstances[key] = append(groupedInstances[key], instance)
	}

	beforeToAfterMap := map[*model.WasmPluginInstance]*model.WasmPluginInstance{}

	for _, key := range order {
		instancesToUpdate := groupedInstances[key]
		if len(instancesToUpdate) == 0 {
			continue
		}
		name := key.pluginName
		version := key.pluginVersion
		internal := key.internal
		plugin := pluginsToUpdate[name]

		for _, instance := range instancesToUpdate {
			if instance.Configurations == nil && deref(instance.RawConfigurations) != "" {
				var configurations map[string]any
				if err := yaml.Unmarshal([]byte(deref(instance.RawConfigurations)), &configurations); err != nil {
					panic(errs.Validation("Error occurs when parsing raw configurations: " +
						deref(instance.RawConfigurations)))
				}
				instance.Configurations = configurations
			}
			configurations := instance.Configurations
			pluginConfig := s.wasmPluginService.QueryConfig(name, "")
			if pluginConfig != nil {
				configurations = validateAndCleanUpConfigurations(pluginConfig, configurations)
			}
			instance.Configurations = configurations
		}

		result := s.addOrUpdateGroupWithRetry(name, version, internal, plugin, instancesToUpdate)

		for _, instance := range instancesToUpdate {
			beforeToAfterMap[instance] = s.converter.GetWasmPluginInstanceFromCr(result, instance.Targets)
		}
	}

	results := make([]*model.WasmPluginInstance, 0, len(instances))
	for _, instance := range instances {
		results = append(results, beforeToAfterMap[instance])
	}
	return results
}

func (s *WasmPluginInstanceService) addOrUpdateGroupWithRetry(name, version string, internal bool,
	plugin *model.WasmPlugin, instancesToUpdate []*model.WasmPluginInstance) *k8s.WasmPlugin {
	for attempt := 1; attempt <= maxUpdateAttempts; attempt++ {
		existedCr := s.getExistingCr(name, version, internal)
		result := s.buildCrForUpdate(version, internal, plugin, existedCr)
		for _, instance := range instancesToUpdate {
			s.converter.SetWasmPluginInstanceToCr(result, instance)
		}

		var (
			updated *k8s.WasmPlugin
			err     error
		)
		if existedCr == nil {
			updated, err = s.client.CreateWasmPlugin(context.Background(), result)
		} else {
			updated, err = s.client.ReplaceWasmPlugin(context.Background(), result)
		}
		if err == nil {
			return updated
		}
		if !apierrors.IsConflict(err) {
			panic(errs.Business("Error occurs when adding or updating the WasmPlugin CR with name: " +
				deref(plugin.Name) + ": " + err.Error()))
		}
		if attempt == maxUpdateAttempts {
			panic(errs.Conflict("Failed to add or update WasmPlugin " + name + " (internal=" +
				strconv.FormatBool(internal) + ") after " + strconv.Itoa(maxUpdateAttempts) +
				" attempts due to concurrent updates."))
		}
		time.Sleep(updateRetryDelayNanos * time.Duration(attempt))
	}
	panic(errs.Internal("Unreachable WasmPlugin update retry state."))
}

func (s *WasmPluginInstanceService) getExistingCr(name, version string, internal bool) *k8s.WasmPlugin {
	var (
		existedCrs []k8s.WasmPlugin
		err        error
	)
	if internal {
		existedCrs, err = s.client.ListWasmPlugin(context.Background(), name, "", nil)
	} else {
		existedCrs, err = s.client.ListWasmPlugin(context.Background(), name, version, nil)
	}
	if err != nil {
		panic(errs.Business("Error occurs when getting WasmPlugin."))
	}
	if len(existedCrs) == 0 {
		return nil
	}
	for i := range existedCrs {
		if internal == k8s.IsInternalResource(existedCrs[i].Name) {
			return &existedCrs[i]
		}
	}
	return nil
}

func (s *WasmPluginInstanceService) buildCrForUpdate(version string, internal bool, plugin *model.WasmPlugin,
	existedCr *k8s.WasmPlugin) *k8s.WasmPlugin {
	if existedCr == nil {
		if version == deref(plugin.PluginVersion) {
			if internal {
				return s.converter.WasmPluginToCrInternal(plugin)
			}
			return s.converter.WasmPluginToCr(plugin)
		}
		panic(errs.Business("Add operation is only allowed for the current plugin version."))
	}

	existedVersion := k8s.GetLabel(&existedCr.ObjectMeta, consts.LabelWasmPluginVersionKey)
	if !internal || !boolValue(plugin.BuiltIn) || deref(plugin.PluginVersion) == existedVersion {
		return existedCr
	}

	currentCr := s.converter.WasmPluginToCrInternal(plugin)
	currentCr.ResourceVersion = existedCr.ResourceVersion
	s.converter.MergeWasmPluginSpec(existedCr, currentCr)
	return currentCr
}

// Delete 对应 delete(scope, target, pluginName, internal)
func (s *WasmPluginInstanceService) Delete(scope model.WasmPluginInstanceScope, target, pluginName string,
	internal *bool) {
	s.DeleteByTargets(map[model.WasmPluginInstanceScope]*string{scope: optStrPtr(target)}, pluginName, internal)
}

// DeleteByTargets 对应 delete(targets, pluginName, internal)
func (s *WasmPluginInstanceService) DeleteByTargets(targets map[model.WasmPluginInstanceScope]*string,
	pluginName string, internal *bool) {
	if len(targets) == 0 {
		return
	}
	existedCrs, err := s.client.ListWasmPlugin(context.Background(), pluginName, "", nil)
	if err != nil {
		panic(errs.Business("Error occurs when getting WasmPlugin."))
	}
	if internal != nil {
		filtered := existedCrs[:0]
		for i := range existedCrs {
			if *internal == k8s.IsInternalResource(existedCrs[i].Name) {
				filtered = append(filtered, existedCrs[i])
			}
		}
		existedCrs = filtered
	}
	s.deletePluginInstances(existedCrs, targets)
}

// DeleteAll 对应 deleteAll(scope, target)
func (s *WasmPluginInstanceService) DeleteAll(scope model.WasmPluginInstanceScope, target string) {
	s.DeleteAllByTargets(map[model.WasmPluginInstanceScope]*string{scope: optStrPtr(target)})
}

// DeleteAllByTargets 对应 deleteAll(targets)
func (s *WasmPluginInstanceService) DeleteAllByTargets(targets map[model.WasmPluginInstanceScope]*string) {
	if len(targets) == 0 {
		return
	}
	existedCrs, err := s.client.ListWasmPlugin(context.Background(), "", "", nil)
	if err != nil {
		panic(errs.Business("Error occurs when getting WasmPlugin."))
	}
	s.deletePluginInstances(existedCrs, targets)
}

func (s *WasmPluginInstanceService) deletePluginInstances(crs []k8s.WasmPlugin,
	targets map[model.WasmPluginInstanceScope]*string) {
	if len(crs) == 0 {
		return
	}
	for i := range crs {
		cr := &crs[i]
		needUpdate := s.converter.RemoveWasmPluginInstanceFromCr(cr, targets)
		if needUpdate {
			if _, err := s.client.ReplaceWasmPlugin(context.Background(), cr); err != nil {
				panic(errs.Business("Error occurs when trying to updating WasmPlugin with name " + cr.Name))
			}
		}
	}
}

// validateAndCleanUpConfigurations 对应 WasmPluginConfig.validateAndCleanUp
func validateAndCleanUpConfigurations(pluginConfig *model.WasmPluginConfig,
	configurations map[string]any) map[string]any {
	// TODO: Implement validation and clean-up logic.
	return configurations
}
