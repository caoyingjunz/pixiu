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
	"fmt"
	"net/url"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/client-go/rest"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/pixiu"
)

const (
	proxyBaseURL = "/proxy/pixiu"
)

type proxyRouter struct{}

type Cloud struct {
	Name string `uri:"cloud_name" binding:"required"`
}

func NewRouter(ginEngine *gin.Engine) {
	s := &proxyRouter{}
	s.initRoutes(ginEngine)
}

func (p *proxyRouter) initRoutes(ginEngine *gin.Engine) {
	auditRoute := ginEngine.Group("/proxy")
	{
		auditRoute.Any("/pixiu/:cloud_name/*act", p.proxyHandler)
	}
}

func (p *proxyRouter) proxyHandler(c *gin.Context) {
	resp := httputils.NewResponse()

	var cloud Cloud
	if err := c.ShouldBindUri(&cloud); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	name := cloud.Name

	config, exists := pixiu.CoreV1.Cloud().GetKubeConfig(c, name)
	if !exists {
		httputils.SetFailed(c, resp, fmt.Errorf("cluster %q not register", name))
		return
	}
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

	httpProxy := proxy.NewUpgradeAwareHandler(target, transport, false, false, nil)
	httpProxy.UpgradeTransport = proxy.NewUpgradeRequestRoundTripper(transport, transport)
	httpProxy.ServeHTTP(c.Writer, c.Request)
}

func (p *proxyRouter) parseTarget(target url.URL, host string, cloud string) (*url.URL, error) {
	kubeURL, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	// TODO: 检查 URL 是否规范
	target.Path = target.Path[len(proxyBaseURL+"/"+cloud):]

	target.Host = kubeURL.Host
	target.Scheme = kubeURL.Scheme
	return &target, nil
}
