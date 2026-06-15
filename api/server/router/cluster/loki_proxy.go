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
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const defaultLokiServiceScheme = "http"

type lokiClusterMeta struct {
	Cluster string `uri:"cluster" binding:"required"`
}

type lokiEndpoint struct {
	Namespace string
	Service   string
	Port      int
	Scheme    string
}

func (cr *clusterRouter) proxyLoki(c *gin.Context) {
	resp := httputils.NewResponse()

	var clusterMeta lokiClusterMeta
	if err := c.ShouldBindUri(&clusterMeta); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	datasource, err := cr.c.LogDatasource().GetDefaultProxyConfigByClusterName(c, clusterMeta.Cluster)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	targetURL, err := url.Parse(datasource.URL)
	if err != nil {
		httputils.SetFailed(c, resp, fmt.Errorf("invalid datasource url %q", datasource.URL))
		return
	}

	config, err := cr.c.Cluster().GetKubeConfigByName(context.TODO(), clusterMeta.Cluster)
	if err != nil {
		httputils.SetFailed(c, resp, fmt.Errorf("failed to get cluster %q kubeconfig", clusterMeta.Cluster))
		return
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	if endpoint, ok, err := resolveLokiServiceEndpoint(c, clientSet, targetURL); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	} else if ok {
		if err = cr.proxyThroughKubeService(c, config, endpoint, datasource); err != nil {
			httputils.SetFailed(c, resp, err)
		}
		return
	}

	cr.proxyDirect(c, targetURL, datasource)
}

func (cr *clusterRouter) proxyThroughKubeService(c *gin.Context, config *rest.Config, endpoint lokiEndpoint, datasource *types.LogDatasourceProxyConfig) error {
	transport, err := rest.TransportFor(config)
	if err != nil {
		return err
	}

	target, err := buildLokiKubeProxyTarget(*c.Request.URL, config.Host, c.Param("act"), endpoint)
	if err != nil {
		return err
	}
	applyDatasourceHeaders(c.Request, datasource)

	httpProxy := proxy.NewUpgradeAwareHandler(target, transport, false, false, nil)
	httpProxy.UpgradeTransport = proxy.NewUpgradeRequestRoundTripper(transport, transport)
	httpProxy.ServeHTTP(c.Writer, c.Request)
	return nil
}

func (cr *clusterRouter) proxyDirect(c *gin.Context, targetURL *url.URL, datasource *types.LogDatasourceProxyConfig) {
	target := *targetURL
	target.Path = joinURLPath(targetURL.Path, normalizeLokiAPIPath(c.Param("act")))

	proxyHandler := httputil.NewSingleHostReverseProxy(targetURL)
	proxyHandler.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = target.Path
		req.Host = target.Host
		applyDatasourceHeaders(req, datasource)
	}
	proxyHandler.ServeHTTP(c.Writer, c.Request)
}

func buildLokiKubeProxyTarget(target url.URL, host string, act string, endpoint lokiEndpoint) (*url.URL, error) {
	kubeURL, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	target.Host = kubeURL.Host
	target.Scheme = kubeURL.Scheme
	target.Path = buildLokiServiceProxyPath(act, endpoint)
	target.RawPath = ""
	return &target, nil
}

func buildLokiServiceProxyPath(act string, endpoint lokiEndpoint) string {
	base := fmt.Sprintf(
		"/api/v1/namespaces/%s/services/%s:%s:%d/proxy",
		url.PathEscape(endpoint.Namespace),
		endpoint.Scheme,
		url.PathEscape(endpoint.Service),
		endpoint.Port,
	)
	return base + normalizeLokiAPIPath(act)
}

func normalizeLokiAPIPath(act string) string {
	trimmed := strings.TrimSpace(act)
	if trimmed == "" || trimmed == "/" {
		return "/loki/api/v1/query_range"
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	return "/loki" + trimmed
}

func resolveLokiServiceEndpoint(ctx context.Context, clientSet kubernetes.Interface, targetURL *url.URL) (lokiEndpoint, bool, error) {
	if targetURL == nil {
		return lokiEndpoint{}, false, nil
	}

	if endpoint, ok, err := resolveEndpointByDNS(ctx, clientSet, targetURL); err != nil || ok {
		return endpoint, ok, err
	}
	if endpoint, ok, err := resolveEndpointByClusterIP(ctx, clientSet, targetURL); err != nil || ok {
		return endpoint, ok, err
	}
	return lokiEndpoint{}, false, nil
}

func resolveEndpointByDNS(ctx context.Context, clientSet kubernetes.Interface, targetURL *url.URL) (lokiEndpoint, bool, error) {
	host := strings.ToLower(strings.TrimSpace(targetURL.Hostname()))
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return lokiEndpoint{}, false, nil
	}

	serviceName, namespace := parts[0], parts[1]
	service, err := clientSet.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return lokiEndpoint{}, false, nil
	}
	port := resolveServicePortFromURL(service, targetURL)
	if port == 0 {
		return lokiEndpoint{}, false, fmt.Errorf("failed to resolve port for service %s/%s", namespace, serviceName)
	}
	return lokiEndpoint{
		Namespace: namespace,
		Service:   serviceName,
		Port:      port,
		Scheme:    normalizeProxyScheme(targetURL.Scheme),
	}, true, nil
}

func resolveEndpointByClusterIP(ctx context.Context, clientSet kubernetes.Interface, targetURL *url.URL) (lokiEndpoint, bool, error) {
	ip := net.ParseIP(strings.TrimSpace(targetURL.Hostname()))
	if ip == nil {
		return lokiEndpoint{}, false, nil
	}

	serviceList, err := clientSet.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return lokiEndpoint{}, false, err
	}
	for i := range serviceList.Items {
		service := &serviceList.Items[i]
		if service.Spec.ClusterIP != ip.String() {
			continue
		}
		port := resolveServicePortFromURL(service, targetURL)
		if port == 0 {
			return lokiEndpoint{}, false, fmt.Errorf("failed to resolve port for service %s/%s", service.Namespace, service.Name)
		}
		return lokiEndpoint{
			Namespace: service.Namespace,
			Service:   service.Name,
			Port:      port,
			Scheme:    normalizeProxyScheme(targetURL.Scheme),
		}, true, nil
	}
	return lokiEndpoint{}, false, nil
}

func resolveServicePortFromURL(service *v1.Service, targetURL *url.URL) int {
	if service == nil {
		return 0
	}
	if portText := strings.TrimSpace(targetURL.Port()); portText != "" {
		for _, port := range service.Spec.Ports {
			if fmt.Sprintf("%d", port.Port) == portText {
				return int(port.Port)
			}
		}
	}
	return findLokiServicePort(service)
}

func findLokiServicePort(service *v1.Service) int {
	if service == nil {
		return 0
	}
	for _, port := range service.Spec.Ports {
		if strings.EqualFold(port.Name, "http") {
			return int(port.Port)
		}
	}
	for _, port := range service.Spec.Ports {
		if port.Port == 80 || port.Port == 3100 {
			return int(port.Port)
		}
	}
	for _, port := range service.Spec.Ports {
		if port.Protocol == "" || port.Protocol == v1.ProtocolTCP {
			return int(port.Port)
		}
	}
	return 0
}

func normalizeProxyScheme(scheme string) string {
	switch strings.ToLower(strings.TrimSpace(scheme)) {
	case "https":
		return "https"
	default:
		return defaultLokiServiceScheme
	}
}

func applyDatasourceHeaders(req *http.Request, datasource *types.LogDatasourceProxyConfig) {
	if datasource == nil {
		return
	}
	for _, header := range datasource.Headers {
		key := strings.TrimSpace(header.Key)
		if key == "" {
			continue
		}
		req.Header.Set(key, header.Value)
	}
	if strings.TrimSpace(datasource.Username) != "" || strings.TrimSpace(datasource.Password) != "" {
		req.SetBasicAuth(datasource.Username, datasource.Password)
	}
}

func joinURLPath(basePath, suffix string) string {
	base := strings.TrimRight(basePath, "/")
	tail := strings.TrimLeft(suffix, "/")
	if base == "" {
		return "/" + tail
	}
	return base + "/" + tail
}
