/*
Copyright 2021 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/klog/v2"

	pixiuclient "github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

var (
	serviceProxyPathRe = regexp.MustCompile(`^/api/v1/namespaces/([^/]+)/services/([^/:]+):(\d+)/proxy(/.*)?$`)
	freePortMu         sync.Mutex
)

type serviceProxyTarget struct {
	namespace string
	service   string
	port      int
	path      string
}

type podProxyTarget struct {
	namespace  string
	podName    string
	remotePort int32
	path       string
}

const (
	nacosTokenDefaultTTL = 5 * time.Hour
	nacosTokenMinTTL     = time.Minute
)

var errNacosAuthDisabled = errors.New("nacos authentication is disabled")

type nacosTokenEntry struct {
	token       string
	expiresAt   time.Time
	fingerprint string
}

func parseServiceProxyPath(k8sPath string) (*serviceProxyTarget, bool) {
	m := serviceProxyPathRe.FindStringSubmatch(k8sPath)
	if m == nil {
		return nil, false
	}
	port, err := strconv.Atoi(m[3])
	if err != nil || port <= 0 {
		return nil, false
	}
	path := m[4]
	if path == "" {
		path = "/"
	}
	return &serviceProxyTarget{
		namespace: m[1],
		service:   m[2],
		port:      port,
		path:      path,
	}, true
}

// tryProxyAuthenticatedService forwards authenticated Service requests through a Pod.
func (p *proxyRouter) tryProxyAuthenticatedService(c *gin.Context, clientSet kubernetes.Interface, config *rest.Config, clusterName string, upstreamAuth string) (handled bool, err error) {
	target, ok := serviceProxyTargetFromRequest(c, clusterName)
	if !ok {
		klog.V(4).Infof("skip authenticated upstream proxy, path not service proxy: %q", c.Request.URL.EscapedPath())
		return false, nil
	}
	podTarget, err := pickOnePodForProxy(c.Request.Context(), clientSet, target)
	if err != nil {
		return true, err
	}
	return true, proxyPodRequest(c, config, clientSet, podTarget, upstreamAuth)
}

// tryProxyDatasourceService handles saved datasource credentials through a Pod.
func (p *proxyRouter) tryProxyDatasourceService(
	c *gin.Context,
	clientSet kubernetes.Interface,
	config *rest.Config,
	clusterName string,
	datasource *types.Datasource,
) (bool, error) {
	if !datasourceRequiresServiceProxy(datasource) {
		return false, nil
	}

	target, ok := serviceProxyTargetFromRequest(c, clusterName)
	if !ok {
		return false, nil
	}
	podTarget, err := pickOnePodForProxy(c.Request.Context(), clientSet, target)
	if err != nil {
		return true, err
	}
	upstreamAuth, err := p.prepareDatasourceServiceRequest(c, clientSet, config, podTarget, target.path, datasource)
	if err != nil {
		return true, err
	}
	return true, proxyPodRequest(c, config, clientSet, podTarget, upstreamAuth)
}

func proxyPodRequest(
	c *gin.Context,
	config *rest.Config,
	clientSet kubernetes.Interface,
	podTarget *podProxyTarget,
	upstreamAuth string,
) error {
	response, err := proxyViaPodPortForward(c.Request.Context(), config, clientSet, podTarget, c.Request, upstreamAuth)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	copyProxyResponse(c, response)
	return nil
}

// prepareExternalDatasourceRequest applies datasource authentication.
func (p *proxyRouter) prepareExternalDatasourceRequest(c *gin.Context, target *url.URL, datasource *types.Datasource) (string, error) {
	if !isNacosDatasource(datasource) {
		return datasourceBasicAuthorization(datasource), nil
	}

	token, err := p.nacosToken(datasource, func() (string, time.Duration, error) {
		loginTarget := *target
		loginTarget.Path = nacosLoginPath(target.Path, datasource.Config.Nacos)
		loginTarget.RawPath = ""
		loginTarget.RawQuery = ""
		request, err := newNacosLoginRequest(c.Request.Context(), datasource, loginTarget.String())
		if err != nil {
			return "", 0, err
		}
		response, err := (&http.Client{Transport: externalProxyTransport, Timeout: externalProxyRequestTimeout}).Do(request)
		if err != nil {
			return "", 0, err
		}
		defer response.Body.Close()
		return parseNacosLoginResponse(response)
	})
	if err != nil {
		return "", err
	}
	addNacosAccessToken(target, token)
	return "", nil
}

func datasourceRequiresServiceProxy(datasource *types.Datasource) bool {
	if datasource == nil {
		return false
	}
	if isNacosDatasource(datasource) {
		return datasource.Config.Log != nil && strings.TrimSpace(datasource.Config.Log.UserName) != ""
	}
	return datasourceBasicAuthorization(datasource) != ""
}

func (p *proxyRouter) prepareDatasourceServiceRequest(
	c *gin.Context,
	clientSet kubernetes.Interface,
	config *rest.Config,
	podTarget *podProxyTarget,
	requestPath string,
	datasource *types.Datasource,
) (string, error) {
	if !isNacosDatasource(datasource) {
		return datasourceBasicAuthorization(datasource), nil
	}

	token, err := p.nacosTokenForService(c.Request.Context(), clientSet, config, podTarget, requestPath, datasource)
	if err != nil {
		return "", err
	}
	addNacosAccessToken(c.Request.URL, token)
	return "", nil
}

func (p *proxyRouter) nacosTokenForService(
	ctx context.Context,
	clientSet kubernetes.Interface,
	config *rest.Config,
	podTarget *podProxyTarget,
	requestPath string,
	datasource *types.Datasource,
) (string, error) {
	return p.nacosToken(datasource, func() (string, time.Duration, error) {
		loginTarget := *podTarget
		loginTarget.path = nacosLoginPath(requestPath, datasource.Config.Nacos)
		request, err := newNacosLoginRequest(ctx, datasource, "http://nacos.local"+loginTarget.path)
		if err != nil {
			return "", 0, err
		}
		response, err := proxyViaPodPortForward(ctx, config, clientSet, &loginTarget, request, "")
		if err != nil {
			return "", 0, err
		}
		defer response.Body.Close()
		return parseNacosLoginResponse(response)
	})
}

func (p *proxyRouter) nacosToken(datasource *types.Datasource, login func() (string, time.Duration, error)) (string, error) {
	if datasource == nil || datasource.Config.Log == nil || strings.TrimSpace(datasource.Config.Log.UserName) == "" {
		return "", nil
	}
	fingerprint := nacosCredentialFingerprint(datasource)
	now := time.Now()
	p.nacosTokenMu.Lock()
	cached, ok := p.nacosTokens[datasource.Id]
	p.nacosTokenMu.Unlock()
	if ok && cached.fingerprint == fingerprint && cached.expiresAt.After(now) {
		return cached.token, nil
	}

	token, ttl, err := login()
	if errors.Is(err, errNacosAuthDisabled) {
		token, ttl, err = "", 10*time.Minute, nil
	}
	if err != nil {
		return "", err
	}
	if ttl <= 0 {
		ttl = nacosTokenDefaultTTL
	}
	cacheTTL := ttl - time.Minute
	if cacheTTL < nacosTokenMinTTL {
		cacheTTL = nacosTokenMinTTL
	}
	p.nacosTokenMu.Lock()
	p.nacosTokens[datasource.Id] = nacosTokenEntry{
		token:       token,
		expiresAt:   now.Add(cacheTTL),
		fingerprint: fingerprint,
	}
	p.nacosTokenMu.Unlock()
	return token, nil
}

func nacosCredentialFingerprint(datasource *types.Datasource) string {
	if datasource == nil || datasource.Config.Log == nil {
		return ""
	}
	version := ""
	if datasource.Config.Nacos != nil {
		version = datasource.Config.Nacos.Version
	}
	return strings.Join([]string{datasource.Config.Log.URL, datasource.Config.Log.UserName, datasource.Config.Log.Password, version}, "\x00")
}

func newNacosLoginRequest(ctx context.Context, datasource *types.Datasource, target string) (*http.Request, error) {
	if datasource == nil || datasource.Config.Log == nil {
		return nil, fmt.Errorf("nacos datasource is missing login configuration")
	}
	body := url.Values{
		"username": {datasource.Config.Log.UserName},
		"password": {datasource.Config.Log.Password},
	}.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	for _, header := range datasource.Config.Headers {
		if key, value := strings.TrimSpace(header.Key), strings.TrimSpace(header.Value); key != "" && value != "" {
			request.Header.Set(key, value)
		}
	}
	return request, nil
}

func nacosLoginPath(requestPath string, config *types.NacosSourceConfig) string {
	if strings.HasPrefix(requestPath, "/nacos/v3/") {
		return "/nacos/v3/auth/user/login"
	}
	if strings.HasPrefix(requestPath, "/v3/") {
		return "/v3/auth/user/login"
	}
	if config != nil && config.Version == "v3" {
		return "/v3/auth/user/login"
	}
	return "/nacos/v1/auth/login"
}

func parseNacosLoginResponse(response *http.Response) (string, time.Duration, error) {
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", 0, err
	}
	if response.StatusCode == http.StatusNotFound {
		return "", 0, errNacosAuthDisabled
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", 0, fmt.Errorf("nacos login failed: %s", nacosResponseMessage(body))
	}

	var payload struct {
		Code        json.RawMessage `json:"code"`
		Message     string          `json:"message"`
		Data        json.RawMessage `json:"data"`
		AccessToken string          `json:"accessToken"`
		TokenTTL    json.RawMessage `json:"tokenTtl"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", 0, fmt.Errorf("invalid nacos login response: %w", err)
	}
	if nacosResponseFailed(payload.Code) {
		return "", 0, fmt.Errorf("nacos login failed: %s", nacosResponseMessage(body))
	}

	token := payload.AccessToken
	ttlRaw := payload.TokenTTL
	if len(payload.Data) > 0 && string(payload.Data) != "null" {
		var data struct {
			AccessToken string          `json:"accessToken"`
			TokenTTL    json.RawMessage `json:"tokenTtl"`
		}
		if err := json.Unmarshal(payload.Data, &data); err == nil {
			if token == "" {
				token = data.AccessToken
			}
			if len(ttlRaw) == 0 {
				ttlRaw = data.TokenTTL
			}
		}
	}
	if token == "" {
		return "", 0, fmt.Errorf("nacos login response did not include accessToken")
	}
	return token, parseNacosTokenTTL(ttlRaw), nil
}

func nacosResponseFailed(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var code int
	if err := json.Unmarshal(raw, &code); err == nil {
		return code != 0 && code != http.StatusOK
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text != "" && text != "0" && text != "200"
	}
	return false
}

func parseNacosTokenTTL(raw json.RawMessage) time.Duration {
	if len(raw) == 0 || string(raw) == "null" {
		return nacosTokenDefaultTTL
	}
	var seconds int64
	if err := json.Unmarshal(raw, &seconds); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if seconds, err := strconv.ParseInt(text, 10, 64); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return nacosTokenDefaultTTL
}

func nacosResponseMessage(body []byte) string {
	var payload struct {
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		if len(payload.Data) > 0 {
			var detail string
			if json.Unmarshal(payload.Data, &detail) == nil && strings.TrimSpace(detail) != "" {
				return detail
			}
		}
		if strings.TrimSpace(payload.Message) != "" {
			return payload.Message
		}
	}
	if text := strings.TrimSpace(string(body)); text != "" {
		return text
	}
	return "unknown error"
}

func addNacosAccessToken(target *url.URL, token string) {
	if target == nil || token == "" {
		return
	}
	query := target.Query()
	query.Set("accessToken", token)
	target.RawQuery = query.Encode()
}

func serviceProxyTargetFromRequest(c *gin.Context, clusterName string) (*serviceProxyTarget, bool) {
	escapedPath := c.Request.URL.EscapedPath()
	prefix := proxyBaseURL + "/" + clusterName
	if strings.HasPrefix(escapedPath, prefix) {
		escapedPath = escapedPath[len(prefix):]
	}
	if escapedPath == "" {
		escapedPath = "/"
	}
	return parseServiceProxyPath(escapedPath)
}

func copyProxyResponse(c *gin.Context, response *http.Response) {
	for key, values := range response.Header {
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}
	c.Status(response.StatusCode)
	_, _ = io.Copy(c.Writer, response.Body)
}

func isNacosDatasource(datasource *types.Datasource) bool {
	return datasource != nil && datasource.SubType == model.DatasourceSubTypeNacos
}

func pickOnePodForProxy(ctx context.Context, clientSet kubernetes.Interface, target *serviceProxyTarget) (*podProxyTarget, error) {
	svc, err := clientSet.CoreV1().Services(target.namespace).Get(ctx, target.service, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get service %s/%s: %w", target.namespace, target.service, err)
	}

	_, targetPort, err := resolveServicePorts(svc, target.port)
	if err != nil {
		return nil, err
	}

	eps, err := clientSet.CoreV1().Endpoints(target.namespace).Get(ctx, target.service, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get endpoints %s/%s: %w", target.namespace, target.service, err)
	}

	for _, subset := range eps.Subsets {
		for _, addr := range subset.Addresses {
			if addr.TargetRef == nil || addr.TargetRef.Kind != "Pod" || addr.TargetRef.Name == "" {
				continue
			}
			for _, port := range subset.Ports {
				if !portMatchesTarget(port, targetPort) {
					continue
				}
				klog.V(2).Infof(
					"selected pod %s/%s:%d for service %s/%s",
					target.namespace, addr.TargetRef.Name, port.Port, target.namespace, target.service,
				)
				return &podProxyTarget{
					namespace:  target.namespace,
					podName:    addr.TargetRef.Name,
					remotePort: port.Port,
					path:       target.path,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("no ready pod found for service %s/%s", target.namespace, target.service)
}

func proxyViaPodPortForward(
	ctx context.Context,
	config *rest.Config,
	clientSet kubernetes.Interface,
	target *podProxyTarget,
	req *http.Request,
	upstreamAuth string,
) (*http.Response, error) {
	localPort, err := reserveLocalPort()
	if err != nil {
		return nil, err
	}

	stopCh := make(chan struct{})
	readyCh := make(chan struct{})
	defer close(stopCh)

	roundTripper, upgrader, err := pixiuclient.RoundTripperFor(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create spdy round tripper: %w", err)
	}

	pfURL := clientSet.CoreV1().RESTClient().
		Post().
		Namespace(target.namespace).
		Resource("pods").
		Name(target.podName).
		SubResource("portforward").
		URL()

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, pfURL)
	ports := []string{fmt.Sprintf("%d:%d", localPort, target.remotePort)}
	forwarder, err := portforward.New(dialer, ports, stopCh, readyCh, io.Discard, io.Discard)
	if err != nil {
		return nil, fmt.Errorf("failed to create port forwarder: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- forwarder.ForwardPorts()
	}()

	select {
	case <-readyCh:
	case err := <-errCh:
		return nil, fmt.Errorf("port-forward to pod %s/%s failed: %w", target.namespace, target.podName, err)
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	url := fmt.Sprintf("http://127.0.0.1:%d%s", localPort, target.path)
	if req.URL.RawQuery != "" {
		url += "?" + req.URL.RawQuery
	}

	proxyReq, err := cloneUpstreamRequest(ctx, req, url, upstreamAuth)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		return nil, fmt.Errorf(
			"upstream request through pod %s/%s port-forward failed: %w",
			target.namespace, target.podName, err,
		)
	}
	return resp, nil
}

func resolveServicePorts(svc *corev1.Service, requestedPort int) (servicePort int32, targetPort intstr.IntOrString, err error) {
	for _, port := range svc.Spec.Ports {
		if int(port.Port) == requestedPort {
			return port.Port, port.TargetPort, nil
		}
	}
	return 0, intstr.IntOrString{}, fmt.Errorf("service port %d not found on %s/%s", requestedPort, svc.Namespace, svc.Name)
}

func portMatchesTarget(port corev1.EndpointPort, targetPort intstr.IntOrString) bool {
	if targetPort.Type == intstr.Int {
		return port.Port == targetPort.IntVal
	}
	return port.Name == targetPort.StrVal
}

func cloneUpstreamRequest(ctx context.Context, orig *http.Request, url string, upstreamAuth string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, orig.Method, url, orig.Body)
	if err != nil {
		return nil, err
	}

	for key, values := range orig.Header {
		lowerKey := strings.ToLower(key)
		if lowerKey == "authorization" ||
			lowerKey == strings.ToLower(upstreamDatasourceIDHeader) ||
			lowerKey == strings.ToLower(externalProxyAuthorizationHeaderKey) {
			continue
		}
		if lowerKey == "host" || lowerKey == "cookie" {
			continue
		}
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	if upstreamAuth != "" {
		req.Header.Set("Authorization", upstreamAuth)
	}
	if orig.ContentLength > 0 {
		req.ContentLength = orig.ContentLength
	}
	return req, nil
}

func reserveLocalPort() (int, error) {
	freePortMu.Lock()
	defer freePortMu.Unlock()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	_, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return 0, err
	}
	return port, nil
}
