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

	p.initKubeGatewayRoutes(ginEngine)
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
	user, err := httputils.GetUserFromContext(c)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	// 返回实际凭证集群：被授权人用主集群名访问时回落到其子集群 scoped kubeconfig，禁止 admin 凭证
	credCluster, authErr := p.c.Cluster().AuthorizeClusterAccessByName(c, user, name)
	if authErr != nil {
		httputils.SetFailed(c, resp, authErr)
		return
	}
	credName := credCluster.Name
	clusterSet, err := p.c.Cluster().GetClusterSetByName(context.TODO(), credName)
	if err != nil {
		httputils.SetFailed(c, resp, fmt.Errorf("failed to get cluster %q clusterSet %v", credName, err))
		return
	}

	// 根据 X-Pixiu-Datasource-Id 从数据源配置解析上游服务（如 ES、Loki）认证信息
	pixiuDatasourceId := strings.TrimSpace(c.Request.Header.Get(upstreamDatasourceIDHeader))
	if len(pixiuDatasourceId) != 0 {
		klog.Infof("proxying with datasource %s", pixiuDatasourceId)
		if upstreamAuth := p.resolveUpstreamAuth(c, pixiuDatasourceId); upstreamAuth != "" {
			handled, proxyErr := p.tryProxyAuthenticatedService(c, clusterSet.Client, clusterSet.Config, credName, upstreamAuth)
			if handled {
				if proxyErr != nil {
					httputils.SetFailed(c, resp, proxyErr)
				}
				return
			}
		}
	}

	target, err := p.parseProxyTarget(*c.Request.URL, name)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err = p.forwardToCluster(c, credName, target); err != nil {
		httputils.SetFailed(c, resp, err)
	}
}

func (p *proxyRouter) parseProxyTarget(target url.URL, name string) (*url.URL, error) {
	target.Path = target.Path[len(proxyBaseURL+"/"+name):]
	return &target, nil
}
