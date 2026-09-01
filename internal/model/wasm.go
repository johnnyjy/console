package model

import "errors"

// WasmPluginInstanceScope 对应 Java 的 WasmPluginInstanceScope
type WasmPluginInstanceScope string

const (
	ScopeGlobal  WasmPluginInstanceScope = "global"
	ScopeDomain  WasmPluginInstanceScope = "domain"
	ScopeRoute   WasmPluginInstanceScope = "route"
	ScopeService WasmPluginInstanceScope = "service"
)

var NonGlobalScopes = []WasmPluginInstanceScope{ScopeDomain, ScopeRoute, ScopeService}

// FromScopeId 根据 id 返回 scope
func FromScopeId(id string) (WasmPluginInstanceScope, bool) {
	switch id {
	case string(ScopeGlobal):
		return ScopeGlobal, true
	case string(ScopeDomain):
		return ScopeDomain, true
	case string(ScopeRoute):
		return ScopeRoute, true
	case string(ScopeService):
		return ScopeService, true
	}
	return "", false
}

// WasmPluginInstance 对应 Java 的 WasmPluginInstance
type WasmPluginInstance struct {
	Version           *string                             `json:"version,omitempty"`
	Scope             *WasmPluginInstanceScope            `json:"scope,omitempty"`
	Target            *string                             `json:"target,omitempty"`
	Targets           map[WasmPluginInstanceScope]*string `json:"targets,omitempty"`
	PluginName        *string                             `json:"pluginName,omitempty"`
	PluginVersion     *string                             `json:"pluginVersion,omitempty"`
	Internal          *bool                               `json:"internal,omitempty"`
	Enabled           *bool                               `json:"enabled,omitempty"`
	RawConfigurations *string                             `json:"rawConfigurations,omitempty"`
	Configurations    map[string]any                      `json:"configurations,omitempty"`
}

func (w *WasmPluginInstance) GetVersion() *string  { return w.Version }
func (w *WasmPluginInstance) SetVersion(v *string) { w.Version = v }

// IsInternal 对应 Java 的 isInternal()
func (w *WasmPluginInstance) IsInternal() bool {
	return w.Internal != nil && *w.Internal
}

// SyncDeprecatedFields 对应 Java 的 syncDeprecatedFields()
func (w *WasmPluginInstance) SyncDeprecatedFields() {
	if w.Scope == nil && len(w.Targets) == 0 {
		return
	}
	if w.Scope != nil {
		w.Targets = map[WasmPluginInstanceScope]*string{*w.Scope: w.Target}
	} else if len(w.Targets) == 1 {
		for scope, target := range w.Targets {
			sc := scope
			w.Scope = &sc
			w.Target = target
		}
	}
}

// HasScopedTarget 对应 Java 的 hasScopedTarget(scope)
func (w *WasmPluginInstance) HasScopedTarget(scope WasmPluginInstanceScope) bool {
	_, ok := w.Targets[scope]
	return ok
}

// HasScopedTargetWithTarget 对应 Java 的 hasScopedTarget(scope, target)
func (w *WasmPluginInstance) HasScopedTargetWithTarget(scope WasmPluginInstanceScope, target string) bool {
	v, ok := w.Targets[scope]
	if !ok {
		return false
	}
	if v == nil {
		return target == ""
	}
	return *v == target
}

// SetGlobalTarget 对应 Java 的 setGlobalTarget()
func (w *WasmPluginInstance) SetGlobalTarget() {
	w.SetTarget(ScopeGlobal, nil)
}

// SetTarget 对应 Java 的 setTarget(scope, target)
func (w *WasmPluginInstance) SetTarget(scope WasmPluginInstanceScope, target *string) {
	w.Targets = map[WasmPluginInstanceScope]*string{scope: target}
	w.SyncDeprecatedFields()
}

// PutTarget 对应 Java 的 putTarget(scope, target)
func (w *WasmPluginInstance) PutTarget(scope WasmPluginInstanceScope, target *string) {
	if w.Targets == nil {
		w.Targets = make(map[WasmPluginInstanceScope]*string)
	}
	w.Targets[scope] = target
	w.SyncDeprecatedFields()
}

// Validate 对应 Java 的 validate()
func (w *WasmPluginInstance) Validate() error {
	if len(w.Targets) == 0 {
		return errors.New("instance.targets cannot be empty.")
	}
	if _, ok := w.Targets[ScopeGlobal]; ok {
		if len(w.Targets) > 1 {
			return errors.New("instance.targets cannot contain GLOBAL and other scopes at the same time.")
		}
		if w.Targets[ScopeGlobal] != nil {
			return errors.New("instance.target must be empty when scope is GLOBAL.")
		}
	} else {
		for _, target := range w.Targets {
			if target == nil || *target == "" {
				return errors.New("instance.target must not be null or empty when scope is not GLOBAL.")
			}
		}
	}
	return nil
}

// WasmPlugin 对应 Java 的 WasmPlugin
type WasmPlugin struct {
	Name            *string `json:"name,omitempty"`
	PluginVersion   *string `json:"pluginVersion,omitempty"`
	Version         *string `json:"version,omitempty"`
	Category        *string `json:"category,omitempty"`
	Title           *string `json:"title,omitempty"`
	Description     *string `json:"description,omitempty"`
	BuiltIn         *bool   `json:"builtIn,omitempty"`
	ProductCovered  *bool   `json:"productCovered,omitempty"`
	Icon            *string `json:"icon,omitempty"`
	ReadmeUrl       *string `json:"readmeUrl,omitempty"`
	ImageRepository *string `json:"imageRepository,omitempty"`
	ImageVersion    *string `json:"imageVersion,omitempty"`
	ImagePullPolicy *string `json:"imagePullPolicy,omitempty"`
	ImagePullSecret *string `json:"imagePullSecret,omitempty"`
	Phase           *string `json:"phase,omitempty"`
	Priority        *int    `json:"priority,omitempty"`
}

func (w *WasmPlugin) GetVersion() *string  { return w.Version }
func (w *WasmPlugin) SetVersion(v *string) { w.Version = v }

// WasmPluginPageQuery 对应 Java 的 WasmPluginPageQuery
type WasmPluginPageQuery struct {
	CommonPageQuery
	Lang *string `json:"lang,omitempty"`
}

// WasmPluginConfig 对应 Java 的 WasmPluginConfig（schema 用 any 表示）
type WasmPluginConfig struct {
	Schema any `json:"schema,omitempty"`
}

// Consumer 对应 Java 的 Consumer
type Consumer struct {
	Name        *string      `json:"name,omitempty"`
	Credentials []Credential `json:"credentials,omitempty"`
}

// Credential 对应 Java 的 Credential
type Credential struct {
	Type       *string        `json:"type,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
	// key-auth 专用字段
	Source *string  `json:"source,omitempty"`
	Key    *string  `json:"key,omitempty"`
	Values []string `json:"values,omitempty"`
}

// KeyAuthCredentialSource 对应 Java 的 KeyAuthCredentialSource
type KeyAuthCredentialSource string

const (
	KeyAuthSourceBearer KeyAuthCredentialSource = "BEARER"
	KeyAuthSourceHeader KeyAuthCredentialSource = "HEADER"
	KeyAuthSourceQuery  KeyAuthCredentialSource = "QUERY"
)

const CredentialTypeKeyAuth = "key-auth"

// AllowList 对应 Java 的 AllowList
type AllowList struct {
	Targets         map[WasmPluginInstanceScope]*string `json:"targets,omitempty"`
	AuthEnabled     *bool                               `json:"authEnabled,omitempty"`
	CredentialTypes []string                            `json:"credentialTypes,omitempty"`
	ConsumerNames   []string                            `json:"consumerNames,omitempty"`
}

// AllowListOperation 对应 Java 的 AllowListOperation
type AllowListOperation string

const (
	AllowListAdd        AllowListOperation = "ADD"
	AllowListRemove     AllowListOperation = "REMOVE"
	AllowListReplace    AllowListOperation = "REPLACE"
	AllowListToggleOnly AllowListOperation = "TOGGLE_ONLY"
)
