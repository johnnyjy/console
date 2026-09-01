package service

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"console/internal/consts"
	"console/internal/errs"
	"console/internal/kubernetes"
)

const (
	aiProxyBasePath          = "/aiproxy"
	aiProxySecretUrlKey      = "aiProxyServiceUrl"
	aiProxySecretTokenKey    = "aiProxyServiceToken"
	aiProxySecretReloadDelay = 60 * time.Second
)

var (
	aiProxyInvalidRequestHeaders = map[string]bool{
		"connection":      true,
		"content-length":  true,
		"accept-encoding": true,
		"host":            true,
		"cookie":          true,
	}
	aiProxyInvalidResponseHeaders = map[string]bool{
		"connection":        true,
		"content-length":    true,
		"content-encoding":  true,
		"server":            true,
		"transfer-encoding": true,
	}
)

type aiProxyServiceInfo struct {
	url   string
	token string
}

func (i *aiProxyServiceInfo) isInvalid() bool {
	return i == nil || strings.TrimSpace(i.url) == ""
}

// AiProxyService 对应 Java 的 AiProxyController
type AiProxyService struct {
	client *kubernetes.Client

	serviceUrl   string
	serviceToken string
	secretName   string

	httpClient *http.Client

	mu          sync.RWMutex
	serviceInfo *aiProxyServiceInfo
}

// NewAiProxyService 创建 AiProxyService
func NewAiProxyService(client *kubernetes.Client) *AiProxyService {
	s := &AiProxyService{
		client:       client,
		serviceUrl:   kubernetes.EnvGet(consts.AiProxyServiceUrlKey, ""),
		serviceToken: kubernetes.EnvGet(consts.AiProxyServiceTokenKey, ""),
		secretName:   kubernetes.EnvGet(consts.SecretNameKey, consts.SecretNameDefault),
	}
	connTimeout := time.Duration(kubernetes.EnvInt(consts.AiProxyConnTimeoutKey, consts.AiProxyConnTimeoutDefault)) * time.Millisecond
	socketTimeout := time.Duration(kubernetes.EnvInt(consts.AiProxySocketTimeoutKey, consts.AiProxySocketTimeoutDefault)) * time.Millisecond
	transport := &http.Transport{
		DialContext:           (&net.Dialer{Timeout: connTimeout}).DialContext,
		ResponseHeaderTimeout: socketTimeout,
	}
	s.httpClient = &http.Client{Transport: transport}

	if strings.TrimSpace(s.serviceUrl) != "" {
		s.serviceInfo = &aiProxyServiceInfo{url: s.serviceUrl, token: s.serviceToken}
	} else {
		s.reloadServiceInfoFromK8s()
		go func() {
			ticker := time.NewTicker(aiProxySecretReloadDelay)
			defer ticker.Stop()
			for range ticker.C {
				s.reloadServiceInfoFromK8s()
			}
		}()
	}
	return s
}

func (s *AiProxyService) reloadServiceInfoFromK8s() {
	secret, err := s.client.ReadSecret(context.Background(), s.secretName)
	if err != nil || secret == nil || len(secret.Data) == 0 {
		return
	}
	urlData := secret.Data[aiProxySecretUrlKey]
	tokenData := secret.Data[aiProxySecretTokenKey]
	if len(urlData) == 0 || len(tokenData) == 0 {
		return
	}
	s.mu.Lock()
	s.serviceInfo = &aiProxyServiceInfo{url: string(urlData), token: string(tokenData)}
	s.mu.Unlock()
}

// Proxy 对应 proxy
func (s *AiProxyService) Proxy(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	info := s.serviceInfo
	s.mu.RUnlock()

	if info.isInvalid() {
		panic(errs.Internal("No valid service info is available for proxying."))
	}

	target := s.buildTargetUrl(info.url, r)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		panic(errs.Business("Error occurs when reading AI proxy request body."))
	}
	_ = r.Body.Close()

	proxyRequest, err := http.NewRequestWithContext(r.Context(), r.Method, target, bytes.NewReader(body))
	if err != nil {
		panic(errs.Business("Error occurs when building AI proxy request."))
	}

	method := strings.ToUpper(r.Method)
	for name, values := range r.Header {
		if aiProxyInvalidRequestHeaders[strings.ToLower(name)] {
			continue
		}
		for _, v := range values {
			proxyRequest.Header.Add(name, v)
		}
	}
	if strings.TrimSpace(info.token) != "" {
		proxyRequest.Header.Set("Authorization", "Bearer "+info.token)
	}
	if method != http.MethodPost && method != http.MethodPut {
		proxyRequest.Body = nil
		proxyRequest.ContentLength = 0
	}

	proxyResponse, err := s.httpClient.Do(proxyRequest)
	if err != nil {
		panic(errs.Business("Error occurs when forwarding AI proxy request."))
	}
	defer proxyResponse.Body.Close()

	for name, values := range proxyResponse.Header {
		if aiProxyInvalidResponseHeaders[strings.ToLower(name)] {
			continue
		}
		for _, v := range values {
			w.Header().Add(name, v)
		}
	}
	w.WriteHeader(proxyResponse.StatusCode)
	_, _ = io.Copy(w, proxyResponse.Body)
}

func (s *AiProxyService) buildTargetUrl(baseUrl string, r *http.Request) string {
	url := baseUrl
	originalRequestUri := r.URL.Path
	idx := strings.Index(originalRequestUri, aiProxyBasePath)
	relativePath := originalRequestUri[idx+len(aiProxyBasePath):]
	if relativePath != "" {
		if strings.HasSuffix(url, "/") && strings.HasPrefix(relativePath, "/") {
			relativePath = relativePath[1:]
		}
	}
	url = url + "/" + relativePath
	if r.URL.RawQuery != "" {
		url = url + "?" + r.URL.RawQuery
	}
	return url
}
