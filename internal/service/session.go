package service

import (
	"context"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"console/internal/consts"
	"console/internal/errs"
	"console/internal/kubernetes"
	"console/internal/model"
	"console/internal/util"
)

const (
	sessionUsernameKey    = "adminUsername"
	sessionDisplayNameKey = "adminDisplayName"
	sessionPasswordKey    = "adminPassword"
	sessionEncryptKeyKey  = "key"
	sessionEncryptKeyLen  = 32
	sessionEncryptIvKey   = "iv"
	sessionEncryptIvLen   = 16
	tokenPartSeparator    = "\x01"
)

// SessionService 对应 Java 的 SessionServiceImpl
type SessionService struct {
	client       *kubernetes.Client
	config       *ConfigService
	cookieName   string
	cookieMaxAge int
	secretName   string
	configTtl    int64

	mu               sync.Mutex
	adminConfigCache *adminConfig
}

type adminConfig struct {
	username            string
	displayName         string
	password            string
	encryptKey          string
	encryptIv           string
	lastUpdateTimestamp int64
}

func (c *adminConfig) isValid() bool {
	return c.username != "" && c.password != "" && c.encryptKey != "" && c.encryptIv != ""
}

func (c *adminConfig) isExpired(ttl int64) bool {
	return time.Now().UnixMilli()-c.lastUpdateTimestamp >= ttl
}

func (c *adminConfig) toUser() *model.User {
	return &model.User{Name: strPtr(c.username), DisplayName: strPtr(c.displayName)}
}

func NewSessionService(client *kubernetes.Client, config *ConfigService) *SessionService {
	return &SessionService{
		client:       client,
		config:       config,
		cookieName:   consts.AdminCookieNameDefault,
		cookieMaxAge: consts.AdminCookieMaxAgeDefault,
		secretName:   consts.SecretNameDefault,
		configTtl:    consts.AdminConfigTtlDefault,
	}
}

func (s *SessionService) CookieName() string { return s.cookieName }
func (s *SessionService) CookieMaxAge() int  { return s.cookieMaxAge }

func (s *SessionService) IsAdminInitialized() bool {
	return s.tryGetAdminConfig() != nil
}

func (s *SessionService) InitializeAdmin(user *model.User) {
	if s.IsAdminInitialized() {
		panic(errs.Internal("Admin user is already initialized."))
	}

	secret, err := s.client.ReadSecret(context.Background(), s.secretName)
	if err != nil {
		panic(errs.Business("Unable to load secret from K8s."))
	}

	data := map[string][]byte{
		sessionUsernameKey:    []byte(deref(user.Name)),
		sessionPasswordKey:    []byte(deref(user.Password)),
		sessionDisplayNameKey: []byte(deref(user.DisplayName)),
		sessionEncryptKeyKey:  []byte(util.RandomGraph(sessionEncryptKeyLen)),
		sessionEncryptIvKey:   []byte(util.RandomGraph(sessionEncryptIvLen)),
	}

	if secret == nil {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: s.secretName},
			Data:       data,
		}
		if _, err := s.client.CreateSecret(context.Background(), secret); err != nil {
			panic(errs.Business("Error occurs when trying to add admin secret."))
		}
	} else {
		newData := map[string][]byte{}
		for k, v := range secret.Data {
			newData[k] = v
		}
		for k, v := range data {
			newData[k] = v
		}
		secret.Data = newData
		if _, err := s.client.ReplaceSecret(context.Background(), secret); err != nil {
			panic(errs.Business("Error occurs when trying to update admin secret."))
		}
	}

	s.mu.Lock()
	s.adminConfigCache = nil
	s.mu.Unlock()
}

func (s *SessionService) Login(username, password string) *model.User {
	config := s.getAdminConfig()
	if config.username != username || config.password != password {
		return nil
	}
	return config.toUser()
}

func (s *SessionService) ChangePassword(username, oldPassword, newPassword string) {
	disabled := s.config.GetBooleanDefault(consts.AdminPasswordChangeDisabled, false)
	if disabled {
		panic(errs.Internal("Password change is disabled."))
	}

	adminConfig := s.getAdminConfig()
	if adminConfig.username != username {
		panic(errs.Validation("Unknown username: " + username))
	}
	if adminConfig.password != oldPassword {
		panic(errs.Validation("Incorrect old password."))
	}

	secret, err := s.client.ReadSecret(context.Background(), s.secretName)
	if err != nil {
		panic(errs.Business("Unable to load secret from K8s."))
	}
	if secret == nil || len(secret.Data) == 0 {
		panic(errs.Internal("Admin secret is missing."))
	}

	newData := map[string][]byte{}
	for k, v := range secret.Data {
		newData[k] = v
	}
	newData[sessionPasswordKey] = []byte(newPassword)
	secret.Data = newData

	if _, err := s.client.ReplaceSecret(context.Background(), secret); err != nil {
		panic(errs.Business("Error occurs when trying to update admin secret."))
	}

	s.mu.Lock()
	s.adminConfigCache = nil
	s.mu.Unlock()
}

// SaveSession 对应 saveSession
func (s *SessionService) SaveSession(w http.ResponseWriter, user *model.User, persistent bool) {
	cookie := s.buildEmptyCookie()
	cookie.Value = s.GenerateToken(user)
	if persistent {
		cookie.MaxAge = s.cookieMaxAge
	}
	http.SetCookie(w, cookie)
}

// ClearSession 对应 clearSession
func (s *SessionService) ClearSession(w http.ResponseWriter) {
	cookie := s.buildEmptyCookie()
	cookie.MaxAge = -1
	http.SetCookie(w, cookie)
}

// ValidateSession 对应 validateSession
func (s *SessionService) ValidateSession(r *http.Request) *model.User {
	if user := s.tryExtractUserFromCookie(r); user != nil {
		return user
	}
	return s.ValidateAuthHeader(r.Header.Get("Authorization"))
}

func (s *SessionService) tryExtractUserFromCookie(r *http.Request) *model.User {
	cookie, err := r.Cookie(s.cookieName)
	if err != nil || cookie == nil || cookie.Value == "" {
		return nil
	}
	return s.ValidateToken(cookie.Value)
}

func (s *SessionService) buildEmptyCookie() *http.Cookie {
	return &http.Cookie{Name: s.cookieName, Path: "/", HttpOnly: true}
}

// GenerateToken 对应 generateToken
func (s *SessionService) GenerateToken(user *model.User) string {
	config := s.getAdminConfig()
	if config.username != deref(user.Name) {
		return ""
	}
	rawToken := strings.Join([]string{config.username, config.password, strconv.FormatInt(time.Now().UnixMilli(), 10)},
		tokenPartSeparator)
	token, err := util.AesEncrypt(config.encryptKey, config.encryptIv, rawToken)
	if err != nil {
		panic(errs.Business("Error occurs when generating token for user " + deref(user.Name)))
	}
	return token
}

// ValidateToken 对应 tryExtractUserFromCookie 中的 token 校验逻辑
func (s *SessionService) ValidateToken(token string) *model.User {
	config := s.tryGetAdminConfig()
	if config == nil {
		return nil
	}
	rawToken, err := util.AesDecrypt(config.encryptKey, config.encryptIv, token)
	if err != nil {
		return nil
	}
	segments := strings.Split(rawToken, tokenPartSeparator)
	if len(segments) < 3 {
		return nil
	}
	return s.validateCredential(segments[0], segments[1])
}

// ValidateAuthHeader 对应 tryExtractUserFromAuthHeader
func (s *SessionService) ValidateAuthHeader(header string) *model.User {
	if header == "" {
		return nil
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || parts[0] != "Basic" {
		return nil
	}
	decoded, err := base64Decode(parts[1])
	if err != nil {
		return nil
	}
	credentials := strings.SplitN(decoded, ":", 2)
	if len(credentials) != 2 {
		return nil
	}
	return s.validateCredential(credentials[0], credentials[1])
}

func (s *SessionService) validateCredential(username, password string) *model.User {
	config := s.tryGetAdminConfig()
	if config == nil {
		return nil
	}
	if config.username != username || config.password != password {
		return nil
	}
	return config.toUser()
}

func (s *SessionService) getAdminConfig() *adminConfig {
	config := s.tryGetAdminConfig()
	if config == nil {
		panic(errs.Internal("No valid admin config is available."))
	}
	return config
}

func (s *SessionService) tryGetAdminConfig() *adminConfig {
	s.mu.Lock()
	cached := s.adminConfigCache
	s.mu.Unlock()

	if cached == nil || cached.isExpired(s.configTtl) {
		loaded := s.loadAdminConfig()
		if loaded != nil && loaded.isValid() {
			loaded.lastUpdateTimestamp = time.Now().UnixMilli()
			s.mu.Lock()
			s.adminConfigCache = loaded
			s.mu.Unlock()
		}
		return loaded
	}
	return cached
}

func (s *SessionService) loadAdminConfig() *adminConfig {
	secret, err := s.client.ReadSecret(context.Background(), s.secretName)
	if err != nil || secret == nil || len(secret.Data) == 0 {
		return nil
	}
	config := &adminConfig{
		username:    stringFromData(secret.Data, sessionUsernameKey),
		displayName: stringFromData(secret.Data, sessionDisplayNameKey),
		password:    stringFromData(secret.Data, sessionPasswordKey),
		encryptKey:  stringFromData(secret.Data, sessionEncryptKeyKey),
		encryptIv:   stringFromData(secret.Data, sessionEncryptIvKey),
	}
	if config.isValid() {
		return config
	}
	return nil
}

func stringFromData(data map[string][]byte, key string) string {
	if v, ok := data[key]; ok {
		return string(v)
	}
	return ""
}

func base64Decode(s string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
