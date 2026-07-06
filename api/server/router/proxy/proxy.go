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
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const (
	proxyBaseURL = "/pixiu/proxy"
)

type proxyRouter struct {
	c controller.PixiuInterface
}

func NewRouter(o *options.Options) {
	s := &proxyRouter{
		c: o.Controller,
	}
	s.initRoutes(o.HttpEngine)
}

func (p *proxyRouter) initRoutes(ginEngine *gin.Engine) {
	proxyRoute := ginEngine.Group("/pixiu/")
	{
		proxyRoute.Any("/proxy/datasources/:datasourceId/*act", p.datasourceProxyHandler)
		proxyRoute.Any("/proxy/:clusterName/*act", p.proxyHandler)
	}
}

func (p *proxyRouter) proxyHandler(c *gin.Context) {
	resp := httputils.NewResponse()

	var cluster struct {
		Name string `uri:"clusterName" binding:"required"`
	}
	if err := c.ShouldBindUri(&cluster); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	name := cluster.Name
	clusterSet, err := p.c.Cluster().GetClusterSetByName(context.TODO(), name)
	if err != nil {
		httputils.SetFailed(c, resp, fmt.Errorf("failed to get cluster %q clusterSet %v", name, err))
		return
	}
	config := clusterSet.Config

	transport, err := rest.TransportFor(config)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	target, err := p.parseTarget(*c.Request.URL, config.Host, name)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	// 清除可能导致 K8s 认证冲突的头部
	// 浏览器发往 Pixiu 的请求带有 Pixiu 的 Authorization，透传给 K8s 会导致 K8s 认证失败
	c.Request.Header.Del("Authorization")
	c.Request.Header.Del("Cookie")

	// 根据 X-Pixiu-Datasource-Id 从数据源配置解析上游服务（如 ES、Loki）认证信息
	pixiuDatasourceId := strings.TrimSpace(c.Request.Header.Get(upstreamDatasourceIDHeader))
	if len(pixiuDatasourceId) != 0 {
		klog.Infof("proxying with datasource %s", pixiuDatasourceId)
		if upstreamAuth := p.resolveUpstreamAuth(c, pixiuDatasourceId); upstreamAuth != "" {
			handled, proxyErr := p.tryProxyAuthenticatedService(c, clusterSet.Client, config, name, upstreamAuth)
			if handled {
				if proxyErr != nil {
					httputils.SetFailed(c, resp, proxyErr)
				}
				return
			}
		}
	}

	klog.Infof("proxying serve directly")
	httpProxy := proxy.NewUpgradeAwareHandler(target, transport, false, false, nil)
	httpProxy.UpgradeTransport = proxy.NewUpgradeRequestRoundTripper(transport, transport)
	httpProxy.ServeHTTP(c.Writer, c.Request)
}

func (p *proxyRouter) parseTarget(target url.URL, host string, name string) (*url.URL, error) {
	kubeURL, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	// TODO: 检查 URL 是否规范
	target.Path = target.Path[len(proxyBaseURL+"/"+name):]

	target.Host = kubeURL.Host
	target.Scheme = kubeURL.Scheme
	return &target, nil
}

func (p *proxyRouter) datasourceProxyHandler(c *gin.Context) {
	resp := httputils.NewResponse()

	var req struct {
		DatasourceID int64  `uri:"datasourceId" binding:"required"`
		Act          string `uri:"act"`
	}
	if err := c.ShouldBindUri(&req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	datasource, err := p.c.Datasource().Get(context.TODO(), req.DatasourceID)
	if err != nil {
		httputils.SetFailed(c, resp, fmt.Errorf("failed to get datasource %d: %v", req.DatasourceID, err))
		return
	}
	if datasource == nil {
		httputils.SetFailed(c, resp, fmt.Errorf("datasource %d not found", req.DatasourceID))
		return
	}
	if !datasource.External {
		httputils.SetFailed(c, resp, fmt.Errorf("datasource %d is not external", req.DatasourceID))
		return
	}

	targetURL, username, password, err := resolveDatasourceUpstream(datasource)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	reverseProxy := httputil.NewSingleHostReverseProxy(targetURL)
	reverseProxy.Director = func(r *http.Request) {
		r.URL.Scheme = targetURL.Scheme
		r.URL.Host = targetURL.Host
		r.URL.Path = joinProxyPath(targetURL.Path, req.Act)
		r.URL.RawPath = r.URL.Path
		r.Host = targetURL.Host

		r.Header.Del("Authorization")
		r.Header.Del("Cookie")
		r.Header.Del(upstreamDatasourceIDHeader)

		if username != "" || password != "" {
			r.SetBasicAuth(username, password)
		}
		for _, header := range datasource.Config.Headers {
			key := strings.TrimSpace(header.Key)
			if key == "" {
				continue
			}
			r.Header.Set(key, header.Value)
		}
	}
	reverseProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, proxyErr error) {
		httputils.SetFailed(c, resp, proxyErr)
	}
	reverseProxy.ServeHTTP(c.Writer, c.Request)
}

func resolveDatasourceUpstream(datasource *types.Datasource) (*url.URL, string, string, error) {
	var rawURL string
	var username string
	var password string

	switch datasource.Type {
	case model.DatasourceTypeLog:
		if datasource.Config.Log == nil {
			return nil, "", "", fmt.Errorf("datasource %d missing log config", datasource.Id)
		}
		rawURL = strings.TrimSpace(datasource.Config.Log.URL)
		username = datasource.Config.Log.UserName
		password = datasource.Config.Log.Password
	case model.DatasourceTypeAlert:
		if datasource.Config.Alert == nil {
			return nil, "", "", fmt.Errorf("datasource %d missing alert config", datasource.Id)
		}
		rawURL = strings.TrimSpace(datasource.Config.Alert.URL)
		username = datasource.Config.Alert.UserName
		password = datasource.Config.Alert.Password
	default:
		return nil, "", "", fmt.Errorf("datasource %d has unsupported type %d", datasource.Id, datasource.Type)
	}

	if rawURL == "" {
		return nil, "", "", fmt.Errorf("datasource %d has empty upstream url", datasource.Id)
	}

	targetURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", "", fmt.Errorf("invalid datasource url %q: %v", rawURL, err)
	}
	if targetURL.Scheme == "" || targetURL.Host == "" {
		return nil, "", "", fmt.Errorf("invalid datasource url %q", rawURL)
	}

	return targetURL, username, password, nil
}

func joinProxyPath(basePath string, act string) string {
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
