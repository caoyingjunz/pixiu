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
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
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
		// 指定代理到 kubernetes 集群
		proxyRoute.Any("/proxy/:clusterName/*act", p.proxyHandler)
		// 通用的外部请求代理
		proxyRoute.Any("/external/*act", p.externalProxyHandler)
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
