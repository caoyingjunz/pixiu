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
	httpProxy.ServeHTTP(c.Writer, c.Request)
	return nil
}
