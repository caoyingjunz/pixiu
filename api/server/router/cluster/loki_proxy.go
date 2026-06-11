/*
Copyright 2024 The Pixiu Authors.

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

package cluster

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/client-go/rest"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
)

const (
	defaultLokiNamespace = "loki"
	defaultLokiService   = "loki"
	defaultLokiPort      = 3100
	defaultLokiScheme    = "http"
	defaultLokiOrgID     = "fake"
)

type lokiClusterMeta struct {
	Cluster string `uri:"cluster" binding:"required"`
}

type lokiProxyOptions struct {
	Namespace string `form:"namespace"`
	Service   string `form:"service"`
	Port      int    `form:"port"`
	Scheme    string `form:"scheme"`
}

func (o *lokiProxyOptions) applyDefaults() {
	if strings.TrimSpace(o.Namespace) == "" {
		o.Namespace = defaultLokiNamespace
	}
	if strings.TrimSpace(o.Service) == "" {
		o.Service = defaultLokiService
	}
	if o.Port == 0 {
		o.Port = defaultLokiPort
	}
	if strings.TrimSpace(o.Scheme) == "" {
		o.Scheme = defaultLokiScheme
	}
}

func (cr *clusterRouter) proxyLoki(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		clusterMeta lokiClusterMeta
		opts        lokiProxyOptions
	)
	if err := c.ShouldBindUri(&clusterMeta); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := c.ShouldBindQuery(&opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	opts.applyDefaults()

	config, err := cr.c.Cluster().GetKubeConfigByName(context.TODO(), clusterMeta.Cluster)
	if err != nil {
		httputils.SetFailed(c, resp, fmt.Errorf("failed to get cluster %q kubeconfig", clusterMeta.Cluster))
		return
	}

	transport, err := rest.TransportFor(config)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	target, err := buildLokiProxyTarget(*c.Request.URL, config.Host, c.Param("act"), opts)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	ensureDefaultLokiOrgID(c)

	httpProxy := proxy.NewUpgradeAwareHandler(target, transport, false, false, nil)
	httpProxy.UpgradeTransport = proxy.NewUpgradeRequestRoundTripper(transport, transport)
	httpProxy.ServeHTTP(c.Writer, c.Request)
}

func buildLokiProxyTarget(target url.URL, host string, act string, opts lokiProxyOptions) (*url.URL, error) {
	kubeURL, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	scheme := strings.ToLower(strings.TrimSpace(opts.Scheme))
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("unsupported loki proxy scheme %q", opts.Scheme)
	}

	target.Host = kubeURL.Host
	target.Scheme = kubeURL.Scheme
	target.Path = buildLokiServiceProxyPath(act, opts.Namespace, opts.Service, opts.Port, scheme)
	target.RawPath = ""
	return &target, nil
}

func buildLokiServiceProxyPath(act, namespace, service string, port int, scheme string) string {
	base := fmt.Sprintf(
		"/api/v1/namespaces/%s/services/%s:%s:%d/proxy",
		url.PathEscape(namespace),
		scheme,
		url.PathEscape(service),
		port,
	)
	if act == "" || act == "/" {
		return base
	}
	if strings.HasPrefix(act, "/") {
		return base + act
	}
	return base + "/" + act
}

func ensureDefaultLokiOrgID(c *gin.Context) {
	if strings.TrimSpace(c.GetHeader("X-Scope-OrgID")) != "" {
		return
	}
	c.Request.Header.Set("X-Scope-OrgID", defaultLokiOrgID)
}
