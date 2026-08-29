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

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

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
	target.Host = kubeURL.Host
	target.Scheme = kubeURL.Scheme

	c.Request.Header.Del("Authorization")
	c.Request.Header.Del("Cookie")

	klog.V(2).Infof("proxying cluster=%s path=%s", clusterName, c.Request.URL.Path)
	httpProxy := proxy.NewUpgradeAwareHandler(target, transport, false, false, nil)
	httpProxy.UpgradeTransport = proxy.NewUpgradeRequestRoundTripper(transport, transport)
	httpProxy.ServeHTTP(&stripWWWAuthenticateWriter{ResponseWriter: c.Writer}, c.Request)
	return nil
}

// stripWWWAuthenticateWriter 包装 gin.ResponseWriter，在 WriteHeader 前删除上游透传的
// WWW-Authenticate 响应头，避免浏览器对 pixiu 代理地址弹出原生认证框。
//
// 选择包装 ResponseWriter 而非包装 Transport 的原因：UpgradeAwareHandler 非升级路径经
// httputil.ReverseProxy 先 copyHeader 到 w.Header() 再调用 w.WriteHeader，此时删除头即可
// 生效，能覆盖所有写入该响应的来源。注意升级协议（WebSocket 等）走 hijack 直写裸连接会
// 绕过本包装，但该路径是 101 升级成功响应、不会携带 401 质询头，实际透传携带
// WWW-Authenticate 的响应均能被本包装拦截。
type stripWWWAuthenticateWriter struct {
	gin.ResponseWriter
}

func (w *stripWWWAuthenticateWriter) WriteHeader(code int) {
	w.Header().Del("WWW-Authenticate")
	w.ResponseWriter.WriteHeader(code)
}
