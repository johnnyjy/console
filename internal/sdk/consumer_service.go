package sdk

import (
	"sort"
	"strings"

	"console/internal/consts"
	"console/internal/errs"
	"console/internal/model"
)

const bearerTokenPrefix = "Bearer "
const authorizationHeader = "Authorization"

// credentialHandler 对应 Java 的 CredentialHandler
type credentialHandler interface {
	getType() string
	getPluginName() string
	isConsumerInUse(consumerName string, instances []*model.WasmPluginInstance) bool
	extractConsumers(instance *model.WasmPluginInstance) []*model.Consumer
	initDefaultGlobalConfigs(instance *model.WasmPluginInstance)
	saveConsumer(instance *model.WasmPluginInstance, consumer *model.Consumer) bool
	deleteConsumer(globalInstance *model.WasmPluginInstance, consumerName string) bool
	getAllowedConsumers(instance *model.WasmPluginInstance) []string
	updateAllowList(operation model.AllowListOperation, instance *model.WasmPluginInstance, consumerNames []string)
}

var defaultCredentialTypes = []string{model.CredentialTypeKeyAuth}

var credentialHandlers = map[string]credentialHandler{
	model.CredentialTypeKeyAuth: &keyAuthCredentialHandler{},
}

// ConsumerService 对应 Java 的 ConsumerServiceImpl
type ConsumerService struct {
	wasmPluginInstanceService *WasmPluginInstanceService
}

// NewConsumerService 创建 ConsumerService
func NewConsumerService(wasmPluginInstanceService *WasmPluginInstanceService) *ConsumerService {
	return &ConsumerService{wasmPluginInstanceService: wasmPluginInstanceService}
}

// AddOrUpdate 对应 addOrUpdate
func (s *ConsumerService) AddOrUpdate(consumer *model.Consumer) *model.Consumer {
	instancesToUpdate := make([]*model.WasmPluginInstance, 0, len(credentialHandlers))
	for _, handler := range credentialHandlers {
		instance := s.getGlobalPluginInstance(handler)
		if instance == nil {
			instance = s.createGlobalPluginInstance(handler)
		}
		instance.Enabled = boolPtr(true)
		if handler.saveConsumer(instance, consumer) {
			instancesToUpdate = append(instancesToUpdate, instance)
		}
	}
	s.wasmPluginInstanceService.AddOrUpdateAll(instancesToUpdate)
	return s.Query(deref(consumer.Name))
}

// List 对应 list
func (s *ConsumerService) List(query *model.CommonPageQuery) *model.PaginatedResult[model.Consumer] {
	consumers := s.getConsumers()
	names := make([]string, 0, len(consumers))
	for name := range consumers {
		names = append(names, name)
	}
	sort.Strings(names)
	list := make([]model.Consumer, 0, len(names))
	for _, name := range names {
		list = append(list, *consumers[name])
	}
	return model.CreateFromFullList(list, query)
}

// Query 对应 query
func (s *ConsumerService) Query(consumerName string) *model.Consumer {
	if consumerName == "" {
		panic(errs.Business("consumerName cannot be empty."))
	}
	return s.getConsumers()[consumerName]
}

// Delete 对应 delete
func (s *ConsumerService) Delete(consumerName string) {
	if consumerName == "" {
		panic(errs.Business("consumerName cannot be empty."))
	}
	instancesCache := map[string][]*model.WasmPluginInstance{}
	for _, handler := range credentialHandlers {
		instances := s.getAllPluginInstances(handler)
		if handler.isConsumerInUse(consumerName, instances) {
			panic(errs.Business("Consumer " + consumerName + " is still in use"))
		}
		instancesCache[handler.getType()] = instances
	}
	for _, handler := range credentialHandlers {
		instances := instancesCache[handler.getType()]
		var globalInstance *model.WasmPluginInstance
		for _, i := range instances {
			if i.HasScopedTarget(model.ScopeGlobal) {
				globalInstance = i
				break
			}
		}
		if globalInstance == nil {
			continue
		}
		if handler.deleteConsumer(globalInstance, consumerName) {
			s.wasmPluginInstanceService.AddOrUpdate(globalInstance)
		}
	}
}

// ListAllowLists 对应 listAllowLists
func (s *ConsumerService) ListAllowLists() []model.AllowList {
	var allowLists []model.AllowList
	for _, handler := range credentialHandlers {
		instances := s.getAllPluginInstances(handler)
		if len(instances) == 0 {
			continue
		}
		for _, instance := range instances {
			if instance.HasScopedTarget(model.ScopeGlobal) {
				continue
			}
			idx := -1
			for i := range allowLists {
				if targetsEqual(instance.Targets, allowLists[i].Targets) {
					idx = i
					break
				}
			}
			if idx < 0 {
				allowLists = append(allowLists, model.AllowList{
					Targets:         cloneTargets(instance.Targets),
					AuthEnabled:     instance.Enabled,
					CredentialTypes: []string{},
					ConsumerNames:   []string{},
				})
				idx = len(allowLists) - 1
			}
			allowList := &allowLists[idx]
			consumerNames := handler.getAllowedConsumers(instance)
			if !containsString(allowList.CredentialTypes, handler.getType()) {
				allowList.CredentialTypes = append(allowList.CredentialTypes, handler.getType())
			}
			for _, consumerName := range consumerNames {
				if !containsString(allowList.ConsumerNames, consumerName) {
					allowList.ConsumerNames = append(allowList.ConsumerNames, consumerName)
				}
			}
		}
	}
	return allowLists
}

// GetAllowList 对应 getAllowList
func (s *ConsumerService) GetAllowList(targets map[model.WasmPluginInstanceScope]*string) *model.AllowList {
	if len(targets) == 0 {
		panic(errs.Business("targets cannot be null or empty."))
	}
	if _, ok := targets[model.ScopeGlobal]; ok {
		panic(errs.Business("targets cannot contain GLOBAL scope."))
	}

	var credentialTypes []string
	var allConsumerNames []string
	seenConsumerNames := map[string]bool{}
	authEnabled := false
	for _, handler := range credentialHandlers {
		instance := s.getPluginInstance(handler, targets)
		if instance == nil {
			continue
		}
		consumerNames := handler.getAllowedConsumers(instance)
		if boolValue(instance.Enabled) {
			authEnabled = true
		}
		credentialTypes = append(credentialTypes, handler.getType())
		for _, name := range consumerNames {
			if !seenConsumerNames[name] {
				seenConsumerNames[name] = true
				allConsumerNames = append(allConsumerNames, name)
			}
		}
	}
	if allConsumerNames == nil {
		return nil
	}
	return &model.AllowList{
		Targets:         targets,
		AuthEnabled:     &authEnabled,
		CredentialTypes: credentialTypes,
		ConsumerNames:   allConsumerNames,
	}
}

// UpdateAllowList 对应 updateAllowList
func (s *ConsumerService) UpdateAllowList(operation model.AllowListOperation, allowList *model.AllowList) {
	if operation == "" {
		panic(errs.Business("operation cannot be null."))
	}
	if allowList == nil {
		panic(errs.Business("allowList cannot be null."))
	}

	targets := allowList.Targets
	consumerNames := allowList.ConsumerNames

	if len(targets) == 0 {
		panic(errs.Business("targets cannot be null or empty."))
	}
	if _, ok := targets[model.ScopeGlobal]; ok {
		panic(errs.Business("targets cannot contain GLOBAL scope."))
	}

	credentialTypes := allowList.CredentialTypes
	if len(credentialTypes) == 0 {
		credentialTypes = defaultCredentialTypes
	} else {
		seen := map[string]bool{}
		distinct := make([]string, 0, len(credentialTypes))
		for _, t := range credentialTypes {
			if !seen[t] {
				seen[t] = true
				distinct = append(distinct, t)
			}
		}
		credentialTypes = distinct
	}

	switch operation {
	case model.AllowListAdd, model.AllowListRemove:
		if len(consumerNames) == 0 && allowList.AuthEnabled == nil {
			return
		}
	case model.AllowListToggleOnly:
		if allowList.AuthEnabled == nil {
			return
		}
	case model.AllowListReplace:
	default:
		panic(errs.Business("Unsupported operation: " + string(operation)))
	}

	for _, credentialType := range credentialTypes {
		handler, ok := credentialHandlers[credentialType]
		if !ok {
			panic(errs.Business("Unsupported credential type: " + credentialType))
		}

		var instancesToSave []*model.WasmPluginInstance

		instances := s.getAllPluginInstances(handler)

		var targetInstance *model.WasmPluginInstance
		for _, i := range instances {
			if targetsEqual(targets, i.Targets) {
				targetInstance = i
				break
			}
		}
		if targetInstance == nil {
			targetInstance = s.wasmPluginInstanceService.CreateEmptyInstance(handler.getPluginName())
			targetInstance.Internal = boolPtr(true)
			// Default to disabled.
			targetInstance.Enabled = boolPtr(false)
			targetInstance.Targets = targets
		}
		if allowList.AuthEnabled != nil {
			targetInstance.Enabled = allowList.AuthEnabled
		}
		handler.updateAllowList(operation, targetInstance, consumerNames)
		instancesToSave = append(instancesToSave, targetInstance)

		var globalInstance *model.WasmPluginInstance
		for _, i := range instances {
			if i.HasScopedTarget(model.ScopeGlobal) {
				globalInstance = i
				break
			}
		}
		if globalInstance == nil && boolValue(targetInstance.Enabled) {
			globalInstance = s.createGlobalPluginInstance(handler)
			instancesToSave = append(instancesToSave, globalInstance)
		}

		s.wasmPluginInstanceService.AddOrUpdateAll(instancesToSave)
	}
}

func (s *ConsumerService) getConsumers() map[string]*model.Consumer {
	consumers := map[string]*model.Consumer{}
	for _, handler := range credentialHandlers {
		instance := s.getGlobalPluginInstance(handler)
		if instance == nil {
			continue
		}
		extracted := handler.extractConsumers(instance)
		for _, consumer := range extracted {
			existed, ok := consumers[deref(consumer.Name)]
			if !ok {
				consumers[deref(consumer.Name)] = consumer
				continue
			}
			existed.Credentials = append(existed.Credentials, consumer.Credentials...)
		}
	}
	return consumers
}

func (s *ConsumerService) getAllPluginInstances(handler credentialHandler) []*model.WasmPluginInstance {
	instances := s.wasmPluginInstanceService.List(handler.getPluginName(), boolPtr(true))
	if instances == nil {
		return []*model.WasmPluginInstance{}
	}
	return instances
}

func (s *ConsumerService) getGlobalPluginInstance(handler credentialHandler) *model.WasmPluginInstance {
	return s.wasmPluginInstanceService.Query(model.ScopeGlobal, "", handler.getPluginName(), boolPtr(true))
}

func (s *ConsumerService) getPluginInstance(handler credentialHandler,
	targets map[model.WasmPluginInstanceScope]*string) *model.WasmPluginInstance {
	return s.wasmPluginInstanceService.QueryByTargets(targets, handler.getPluginName(), boolPtr(true))
}

func (s *ConsumerService) createGlobalPluginInstance(handler credentialHandler) *model.WasmPluginInstance {
	instance := s.wasmPluginInstanceService.CreateEmptyInstance(handler.getPluginName())
	instance.Internal = boolPtr(true)
	instance.SetGlobalTarget()
	handler.initDefaultGlobalConfigs(instance)
	return instance
}

func targetsEqual(a, b map[model.WasmPluginInstanceScope]*string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}
		if av == nil || bv == nil {
			if av != nil || bv != nil {
				return false
			}
			continue
		}
		if *av != *bv {
			return false
		}
	}
	return true
}

// ---- key-auth credential handler ----

type keyAuthCredential struct {
	source string
	key    string
	values []string
}

type keyAuthCredentialHandler struct{}

func (h *keyAuthCredentialHandler) getType() string {
	return model.CredentialTypeKeyAuth
}

func (h *keyAuthCredentialHandler) getPluginName() string {
	return consts.PluginKeyAuth
}

func (h *keyAuthCredentialHandler) isConsumerInUse(consumerName string,
	instances []*model.WasmPluginInstance) bool {
	if len(instances) == 0 {
		return false
	}
	for _, instance := range instances {
		configurations := instance.Configurations
		if len(configurations) == 0 {
			continue
		}
		if allowList, ok := configurations[consts.KeyAuthAllow].([]any); ok {
			for _, item := range allowList {
				if s, ok := item.(string); ok && s == consumerName {
					return true
				}
			}
		}
	}
	return false
}

func (h *keyAuthCredentialHandler) extractConsumers(instance *model.WasmPluginInstance) []*model.Consumer {
	if len(instance.Configurations) == 0 {
		return []*model.Consumer{}
	}
	consumersObj, ok := instance.Configurations[consts.KeyAuthConsumers]
	if !ok {
		return []*model.Consumer{}
	}
	consumerList, ok := consumersObj.([]any)
	if !ok {
		return []*model.Consumer{}
	}
	consumers := make([]*model.Consumer, 0, len(consumerList))
	for _, consumerObj := range consumerList {
		consumerMap, ok := consumerObj.(map[string]any)
		if !ok {
			continue
		}
		consumer := h.extractConsumer(consumerMap)
		if consumer != nil {
			consumers = append(consumers, consumer)
		}
	}
	return consumers
}

func (h *keyAuthCredentialHandler) initDefaultGlobalConfigs(instance *model.WasmPluginInstance) {
	configurations := instance.Configurations
	if len(configurations) == 0 {
		configurations = map[string]any{}
		instance.Configurations = configurations
	}
	if _, ok := configurations[consts.KeyAuthGlobalAuth]; !ok {
		configurations[consts.KeyAuthGlobalAuth] = false
	}
	if _, ok := configurations[consts.KeyAuthAllow]; !ok {
		configurations[consts.KeyAuthAllow] = []any{}
	}
	// TODO: Remove this after plugin upgrade.
	configurations[consts.KeyAuthKeys] = []any{"x-higress-dummy-key"}
	if _, ok := configurations[consts.KeyAuthConsumers]; !ok {
		configurations[consts.KeyAuthConsumers] = []any{}
	}
}

func (h *keyAuthCredentialHandler) saveConsumer(instance *model.WasmPluginInstance,
	consumer *model.Consumer) bool {
	if len(consumer.Credentials) == 0 {
		return false
	}

	keyAuthCred := findKeyAuthCredential(consumer.Credentials)
	if keyAuthCred == nil {
		return h.deleteConsumer(instance, deref(consumer.Name))
	}

	configurations := instance.Configurations
	if len(configurations) == 0 {
		h.initDefaultGlobalConfigs(instance)
		configurations = instance.Configurations
	}

	consumers := []any{}
	if consumersObj, ok := configurations[consts.KeyAuthConsumers]; ok {
		if list, ok := consumersObj.([]any); ok {
			consumers = append(consumers, list...)
		}
	}

	var consumerConfig map[string]any
	for _, consumerObj := range consumers {
		consumerMap, ok := consumerObj.(map[string]any)
		if !ok {
			continue
		}
		existedConsumer := h.extractConsumer(consumerMap)
		if existedConsumer == nil {
			continue
		}
		if deref(consumer.Name) == deref(existedConsumer.Name) {
			consumerConfig = consumerMap
		} else if h.hasSameCredential(existedConsumer, keyAuthCred) {
			panic(errs.Business("Key auth credential already in use by consumer: " +
				deref(existedConsumer.Name)))
		}
	}

	if consumerConfig == nil {
		consumerConfig = map[string]any{}
		consumerConfig[consts.KeyAuthConsumerName] = deref(consumer.Name)
		consumers = append(consumers, consumerConfig)
	} else {
		keyAuthCred = h.mergeExistedConfig(keyAuthCred, consumerConfig)
	}

	h.validateKeyAuthCredential(keyAuthCred, false)

	source, ok := parseKeyAuthSource(keyAuthCred.source)
	if !ok {
		panic(errs.Business("Invalid key auth credential source: " + keyAuthCred.source))
	}
	key := keyAuthCred.key
	credentials := keyAuthCred.values
	switch source {
	case model.KeyAuthSourceBearer, model.KeyAuthSourceHeader:
		consumerConfig[consts.KeyAuthInHeader] = true
		consumerConfig[consts.KeyAuthInQuery] = false
		if source == model.KeyAuthSourceBearer {
			key = authorizationHeader
			credentials = make([]string, 0, len(credentials))
			for _, c := range credentials {
				credentials = append(credentials, bearerTokenPrefix+c)
			}
		}
	case model.KeyAuthSourceQuery:
		consumerConfig[consts.KeyAuthInHeader] = false
		consumerConfig[consts.KeyAuthInQuery] = true
	default:
		panic(errs.Business("Unsupported key auth credential source: " + keyAuthCred.source))
	}
	consumerConfig[consts.KeyAuthKeys] = []any{key}
	consumerConfig[consts.KeyAuthConsumerCreds] = anyStringList(credentials)
	delete(consumerConfig, consts.KeyAuthConsumerCred)

	configurations[consts.KeyAuthConsumers] = consumers
	configurations[consts.KeyAuthGlobalAuth] = false
	return true
}

func (h *keyAuthCredentialHandler) deleteConsumer(globalInstance *model.WasmPluginInstance,
	consumerName string) bool {
	globalConfigurations := globalInstance.Configurations
	if len(globalConfigurations) == 0 {
		return false
	}
	consumersObj, ok := globalConfigurations[consts.KeyAuthConsumers]
	if !ok {
		return false
	}
	consumers, ok := consumersObj.([]any)
	if !ok {
		return false
	}
	deleted := false
	newConsumers := make([]any, 0, len(consumers))
	for _, consumerObj := range consumers {
		consumerMap, ok := consumerObj.(map[string]any)
		if !ok {
			newConsumers = append(newConsumers, consumerObj)
			continue
		}
		if consumerName == stringValue(consumerMap[consts.KeyAuthConsumerName]) {
			deleted = true
			continue
		}
		newConsumers = append(newConsumers, consumerObj)
	}
	if deleted {
		globalConfigurations[consts.KeyAuthConsumers] = newConsumers
	}
	return deleted
}

func (h *keyAuthCredentialHandler) getAllowedConsumers(instance *model.WasmPluginInstance) []string {
	configurations := instance.Configurations
	if len(configurations) == 0 {
		return []string{}
	}
	allowObj, ok := configurations[consts.KeyAuthAllow]
	if !ok {
		return []string{}
	}
	allowList, ok := allowObj.([]any)
	if !ok {
		return []string{}
	}
	result := make([]string, 0, len(allowList))
	for _, item := range allowList {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

func (h *keyAuthCredentialHandler) updateAllowList(operation model.AllowListOperation,
	instance *model.WasmPluginInstance, consumerNames []string) {
	configurations := instance.Configurations
	if len(configurations) == 0 {
		configurations = map[string]any{}
		instance.Configurations = configurations
	}

	newAllowList := h.getAllowedConsumers(instance)
	switch operation {
	case model.AllowListAdd:
		for _, consumerName := range consumerNames {
			if !containsString(newAllowList, consumerName) {
				newAllowList = append(newAllowList, consumerName)
			}
		}
	case model.AllowListRemove:
		for _, consumerName := range consumerNames {
			newAllowList = removeStringFrom(newAllowList, consumerName)
		}
	case model.AllowListReplace:
		newAllowList = consumerNames
	case model.AllowListToggleOnly:
		if len(newAllowList) == 0 {
			newAllowList = []string{}
		}
	default:
		panic(errs.Business("Unsupported operation: " + string(operation)))
	}
	configurations[consts.KeyAuthAllow] = anyStringList(newAllowList)
}

func (h *keyAuthCredentialHandler) extractConsumer(consumerMap map[string]any) *model.Consumer {
	if len(consumerMap) == 0 {
		return nil
	}
	name := stringValue(consumerMap[consts.KeyAuthConsumerName])
	if strings.TrimSpace(name) == "" {
		return nil
	}
	credential := h.parseCredential(consumerMap)
	if credential == nil {
		return nil
	}
	consumer := &model.Consumer{Name: strPtr(name)}
	consumer.Credentials = []model.Credential{keyAuthCredentialToModel(credential)}
	return consumer
}

func (h *keyAuthCredentialHandler) hasSameCredential(existedConsumer *model.Consumer,
	credential *keyAuthCredential) bool {
	if credential == nil || existedConsumer == nil {
		return false
	}
	existedCredential := findKeyAuthCredential(existedConsumer.Credentials)
	if existedCredential == nil {
		return false
	}
	if !strings.EqualFold(credential.source, existedCredential.source) {
		return false
	}
	if credential.key != existedCredential.key {
		return false
	}
	if len(credential.values) == 0 || len(existedCredential.values) == 0 {
		return false
	}
	for _, value := range credential.values {
		if containsString(existedCredential.values, value) {
			return true
		}
	}
	return false
}

func (h *keyAuthCredentialHandler) mergeExistedConfig(keyAuthCred *keyAuthCredential,
	consumerConfig map[string]any) *keyAuthCredential {
	existedCredential := h.parseCredential(consumerConfig)
	if existedCredential == nil {
		return keyAuthCred
	}
	merged := &keyAuthCredential{
		source: firstNonBlank(keyAuthCred.source, existedCredential.source),
		key:    firstNonBlank(keyAuthCred.key, existedCredential.key),
	}
	if len(keyAuthCred.values) > 0 {
		merged.values = keyAuthCred.values
	} else {
		merged.values = existedCredential.values
	}
	return merged
}

func (h *keyAuthCredentialHandler) validateKeyAuthCredential(credential *keyAuthCredential, forUpdate bool) {
	if strings.TrimSpace(credential.source) == "" {
		panic(errs.Validation("source cannot be blank."))
	}
	source, ok := parseKeyAuthSource(credential.source)
	if !ok {
		panic(errs.Validation("unknown source value: " + credential.source))
	}
	if sourceRequiresKey(source) && strings.TrimSpace(credential.key) == "" {
		panic(errs.Validation("key cannot be blank."))
	}
	if !forUpdate && len(credential.values) == 0 {
		panic(errs.Validation("value cannot be blank."))
	}
}

func (h *keyAuthCredentialHandler) parseCredential(consumerMap map[string]any) *keyAuthCredential {
	keyObj, ok := consumerMap[consts.KeyAuthKeys]
	if !ok {
		return nil
	}
	keyList, ok := keyObj.([]any)
	if !ok {
		return nil
	}
	if len(keyList) == 0 {
		return nil
	}
	key := ""
	for _, keyItemObj := range keyList {
		keyItem, ok := keyItemObj.(string)
		if !ok {
			continue
		}
		if strings.TrimSpace(keyItem) != "" {
			key = keyItem
		}
	}
	if key == "" {
		return nil
	}

	inHeader := boolValueOfAny(consumerMap[consts.KeyAuthInHeader])
	inQuery := boolValueOfAny(consumerMap[consts.KeyAuthInQuery])

	var credentials []string
	if credentialsObj, ok := consumerMap[consts.KeyAuthConsumerCreds]; ok {
		if credentialsList, ok := credentialsObj.([]any); ok {
			for _, credentialObj := range credentialsList {
				if credential, ok := credentialObj.(string); ok {
					credentials = append(credentials, credential)
				}
			}
		}
	}
	{
		// TODO: To be removed later.
		credential := stringValue(consumerMap[consts.KeyAuthConsumerCred])
		if strings.TrimSpace(credential) != "" && !containsString(credentials, credential) {
			credentials = append(credentials, credential)
		}
	}

	var source model.KeyAuthCredentialSource
	if inHeader {
		if key == authorizationHeader {
			allBearer := len(credentials) > 0
			for _, c := range credentials {
				if !strings.HasPrefix(c, bearerTokenPrefix) {
					allBearer = false
					break
				}
			}
			if allBearer {
				source = model.KeyAuthSourceBearer
				key = ""
				for i, c := range credentials {
					credentials[i] = strings.TrimSpace(strings.TrimPrefix(c, bearerTokenPrefix))
				}
			} else {
				source = model.KeyAuthSourceHeader
			}
		} else {
			source = model.KeyAuthSourceHeader
		}
	} else if inQuery {
		source = model.KeyAuthSourceQuery
	} else {
		return nil
	}
	return &keyAuthCredential{source: string(source), key: key, values: credentials}
}

func findKeyAuthCredential(credentials []model.Credential) *keyAuthCredential {
	for i := range credentials {
		if deref(credentials[i].Type) == model.CredentialTypeKeyAuth {
			return &keyAuthCredential{
				source: deref(credentials[i].Source),
				key:    deref(credentials[i].Key),
				values: credentials[i].Values,
			}
		}
	}
	return nil
}

func keyAuthCredentialToModel(c *keyAuthCredential) model.Credential {
	return model.Credential{
		Type:   strPtr(model.CredentialTypeKeyAuth),
		Source: strPtr(c.source),
		Key:    optStrPtr(c.key),
		Values: c.values,
	}
}

func parseKeyAuthSource(s string) (model.KeyAuthCredentialSource, bool) {
	switch s {
	case string(model.KeyAuthSourceBearer):
		return model.KeyAuthSourceBearer, true
	case string(model.KeyAuthSourceHeader):
		return model.KeyAuthSourceHeader, true
	case string(model.KeyAuthSourceQuery):
		return model.KeyAuthSourceQuery, true
	}
	return "", false
}

func sourceRequiresKey(s model.KeyAuthCredentialSource) bool {
	return s != model.KeyAuthSourceBearer
}

func anyStringList(list []string) []any {
	result := make([]any, len(list))
	for i, s := range list {
		result[i] = s
	}
	return result
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

func boolValueOfAny(v any) bool {
	b, _ := v.(bool)
	return b
}

func firstNonBlank(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
