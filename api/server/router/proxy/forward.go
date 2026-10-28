/*
Copyright 2026 The Pixiu Authors.

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
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

// applyClusterHost 回填上游地址，并保留 kubeconfig server 的路径前缀。
// client-go 将 server 中的非空 path 作为所有 API 请求的统一前缀；
// 代理转发需保持同一语义，否则经网关暴露的集群（如 vcluster、Rancher）会 404。
func applyClusterHost(target, kubeURL *url.URL) {
	target.Host = kubeURL.Host
	target.Scheme = kubeURL.Scheme

	prefix := path.Clean("/" + kubeURL.Path)
	if prefix == "/" {
		return
	}
	target.Path = prefix + "/" + strings.TrimPrefix(target.Path, "/")
	target.RawPath = ""
}

// forwardToCluster 构建传输层并转发请求到目标集群。
// target 的 Host/Scheme 会从集群配置中回填。
func (p *proxyRouter) forwardToCluster(c *gin.Context, clusterName string, target *url.URL) error {
	clusterSet, err := p.c.Cluster().GetClusterSetByName(context.TODO(), clusterName)
	if err != nil {
		return fmt.Errorf("failed to get cluster %q: %w", clusterName, err)
	}

	transport, err := rest.TransportFor(clusterSet.Config)
	if err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}

	kubeURL, err := url.Parse(clusterSet.Config.Host)
	if err != nil {
		return fmt.Errorf("invalid cluster host: %w", err)
	}
	applyClusterHost(target, kubeURL)

	c.Request.Header.Del("Authorization")
	c.Request.Header.Del("Cookie")

	klog.V(2).Infof("proxying cluster=%s path=%s", clusterName, c.Request.URL.Path)
	httpProxy := proxy.NewUpgradeAwareHandler(target, transport, false, false, nil)
	httpProxy.UpgradeTransport = proxy.NewUpgradeRequestRoundTripper(transport, transport)
	httpProxy.ServeHTTP(c.Writer, c.Request)
	return nil
}
