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
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
)

const (
	externalProxyTargetQueryKey         = "url"
	externalProxyAuthorizationHeaderKey = "X-Pixiu-Proxy-Authorization"
)

const (
	externalProxyMaxBodyBytes          int64 = 10 << 20
	externalProxyDialTimeout                 = 10 * time.Second
	externalProxyTLSHandshakeTimeout         = 10 * time.Second
	externalProxyResponseHeaderTimeout       = 30 * time.Second
	externalProxyIdleConnTimeout             = 90 * time.Second
	externalProxyRequestTimeout              = 60 * time.Second
)

var errExternalProxyResponseTooLarge = errors.New("external proxy response exceeds size limit")
var errExternalProxyRequestTooLarge = errors.New("external proxy request exceeds size limit")

var externalProxyTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout: externalProxyDialTimeout,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          100,
	IdleConnTimeout:       externalProxyIdleConnTimeout,
	TLSHandshakeTimeout:   externalProxyTLSHandshakeTimeout,
	ExpectContinueTimeout: 1 * time.Second,
	ResponseHeaderTimeout: externalProxyResponseHeaderTimeout,
}

// externalProxyHandler 通用外部 HTTP 代理：透传 Method / Query / Body / 请求头。
//
//	@Summary		通用外部请求代理
//	@Description	将请求转发至 url 查询参数指定的上游；上游认证使用 X-Pixiu-Proxy-Authorization 请求头
//	@Tags			Proxy
//	@Param			url	query	string	true	"上游基础地址，如 http://es.example.com:9200"
//	@Param			X-Pixiu-Proxy-Authorization	header	string	false	"上游 Authorization，如 Basic xxx"
//	@Param			act	path	string	true	"上游 API 路径后缀"
//	@Success		200	{object}	object
//	@Router			/pixiu/external/*act [get]
func (p *proxyRouter) externalProxyHandler(c *gin.Context) {
	resp := httputils.NewResponse()
	ctx, cancel := context.WithTimeout(c.Request.Context(), externalProxyRequestTimeout)
	defer cancel()
	c.Request = c.Request.WithContext(ctx)
	if c.Request.ContentLength > externalProxyMaxBodyBytes {
		httputils.SetFailed(c, resp, errExternalProxyRequestTooLarge)
		return
	}
	if c.Request.Body != nil {
		c.Request.Body = newMaxBytesReadCloser(c.Request.Body, externalProxyMaxBodyBytes, errExternalProxyRequestTooLarge)
	}

	var req struct {
		Act string `uri:"act"`
	}
	if err := c.ShouldBindUri(&req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	target, err := resolveExternalProxyTarget(c, req.Act)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	p.forwardExternalRequest(c, resp, target, c.Request)
}

func resolveExternalProxyTarget(c *gin.Context, act string) (*url.URL, error) {
	raw := strings.TrimSpace(c.Query(externalProxyTargetQueryKey))
	if raw == "" {
		return nil, fmt.Errorf("missing %s query parameter", externalProxyTargetQueryKey)
	}

	baseURL, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", externalProxyTargetQueryKey, err)
	}
	if baseURL.Scheme != "http" && baseURL.Scheme != "https" {
		return nil, fmt.Errorf("%s must use http or https scheme", externalProxyTargetQueryKey)
	}
	if baseURL.Host == "" {
		return nil, fmt.Errorf("%s must include host", externalProxyTargetQueryKey)
	}

	targetURL := *baseURL
	targetURL.Path = joinUpstreamProxyPath(baseURL.Path, act)
	targetURL.RawPath = targetURL.Path

	query := c.Request.URL.Query()
	query.Del(externalProxyTargetQueryKey)
	mergedQuery := baseURL.Query()
	for key, values := range query {
		mergedQuery.Del(key)
		for _, value := range values {
			mergedQuery.Add(key, value)
		}
	}
	targetURL.RawQuery = mergedQuery.Encode()
	return &targetURL, nil
}

func (p *proxyRouter) forwardExternalRequest(c *gin.Context, resp *httputils.Response, targetURL *url.URL, upstreamReq *http.Request) {
	reverseProxy := httputil.NewSingleHostReverseProxy(targetURL)
	reverseProxy.Transport = externalProxyTransport
	reverseProxy.Director = func(r *http.Request) {
		r.URL.Scheme = targetURL.Scheme
		r.URL.Host = targetURL.Host
		r.URL.Path = targetURL.Path
		r.URL.RawPath = targetURL.RawPath
		r.URL.RawQuery = targetURL.RawQuery
		r.Host = targetURL.Host
		r.Method = upstreamReq.Method

		r.Header = make(http.Header)
		for key, values := range upstreamReq.Header {
			lowerKey := strings.ToLower(strings.TrimSpace(key))
			if lowerKey == "authorization" || lowerKey == "cookie" || lowerKey == strings.ToLower(externalProxyAuthorizationHeaderKey) {
				continue
			}
			for _, value := range values {
				r.Header.Add(key, value)
			}
		}

		if proxyAuth := strings.TrimSpace(upstreamReq.Header.Get(externalProxyAuthorizationHeaderKey)); proxyAuth != "" {
			r.Header.Set("Authorization", proxyAuth)
		}
	}
	reverseProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, proxyErr error) {
		httputils.SetFailed(c, resp, proxyErr)
	}
	reverseProxy.ModifyResponse = func(r *http.Response) error {
		if r != nil && r.Body != nil {
			if r.ContentLength > externalProxyMaxBodyBytes {
				r.Body.Close()
				return errExternalProxyResponseTooLarge
			}
			r.Body = newMaxBytesReadCloser(r.Body, externalProxyMaxBodyBytes, errExternalProxyResponseTooLarge)
		}
		return nil
	}
	reverseProxy.ServeHTTP(c.Writer, upstreamReq)
}

type maxBytesReadCloser struct {
	rc        io.ReadCloser
	remaining int64
	limitErr  error
}

func newMaxBytesReadCloser(rc io.ReadCloser, limit int64, limitErr error) io.ReadCloser {
	return &maxBytesReadCloser{
		rc:        rc,
		remaining: limit,
		limitErr:  limitErr,
	}
}

func (m *maxBytesReadCloser) Read(p []byte) (int, error) {
	if m.remaining <= 0 {
		return 0, m.limitErr
	}

	maxRead := len(p)
	if int64(maxRead) > m.remaining {
		maxRead = int(m.remaining)
	}

	n, err := m.rc.Read(p[:maxRead])
	m.remaining -= int64(n)
	if err != nil {
		return n, err
	}

	if m.remaining == 0 {
		var probe [1]byte
		probeN, probeErr := m.rc.Read(probe[:])
		if probeN > 0 || (probeErr != nil && !errors.Is(probeErr, io.EOF)) {
			return n, m.limitErr
		}
		return n, io.EOF
	}

	return n, nil
}

func (m *maxBytesReadCloser) Close() error {
	return m.rc.Close()
}

func joinUpstreamProxyPath(basePath string, act string) string {
	trimmedBase := strings.TrimRight(basePath, "/")
	trimmedAct := "/" + strings.TrimLeft(act, "/")
	if trimmedBase == "" {
		return trimmedAct
	}
	if trimmedAct == "/" {
		return trimmedBase
	}
	return trimmedBase + trimmedAct
}
