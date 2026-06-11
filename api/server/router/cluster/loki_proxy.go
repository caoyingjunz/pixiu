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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
)

const (
	lokiNamespaceLabelKey   = "pixiu-loki"
	lokiNamespaceLabelValue = "true"
	lokiGatewayLabelKey     = "pixiu-loki-gateway"
	lokiGatewayLabelValue   = "true"
	lokiServiceScheme       = "http"
)

type lokiClusterMeta struct {
	Cluster string `uri:"cluster" binding:"required"`
}

type lokiEndpoint struct {
	Namespace string `form:"namespace"`
	Service   string
	Port      int
}

func (cr *clusterRouter) proxyLoki(c *gin.Context) {
	resp := httputils.NewResponse()

	var clusterMeta lokiClusterMeta
	if err := c.ShouldBindUri(&clusterMeta); err != nil {
		httputils.SetFailed(c, resp, err)
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
	endpoint, err := discoverLokiEndpoint(c, clientSet)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	transport, err := rest.TransportFor(config)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	target, err := buildLokiProxyTarget(*c.Request.URL, config.Host, c.Param("act"), endpoint)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	httpProxy := proxy.NewUpgradeAwareHandler(target, transport, false, false, nil)
	httpProxy.UpgradeTransport = proxy.NewUpgradeRequestRoundTripper(transport, transport)
	httpProxy.ServeHTTP(c.Writer, c.Request)
}

func buildLokiProxyTarget(target url.URL, host string, act string, endpoint lokiEndpoint) (*url.URL, error) {
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
		lokiServiceScheme,
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

func discoverLokiEndpoint(ctx context.Context, clientSet kubernetes.Interface) (lokiEndpoint, error) {
	nsList, err := clientSet.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", lokiNamespaceLabelKey, lokiNamespaceLabelValue),
	})
	if err != nil {
		return lokiEndpoint{}, err
	}
	namespace, err := findLokiNamespace(nsList.Items)
	if namespace == "" {
		return lokiEndpoint{}, err
	}

	serviceList, err := clientSet.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", lokiGatewayLabelKey, lokiGatewayLabelValue),
	})
	if err != nil {
		return lokiEndpoint{}, err
	}
	selectedService, err := findLokiService(serviceList.Items)
	if selectedService == nil {
		return lokiEndpoint{}, err
	}
	port := findLokiServicePort(selectedService)
	if port == 0 {
		return lokiEndpoint{}, fmt.Errorf("failed to resolve a TCP port for loki service %q", selectedService.Name)
	}
	return lokiEndpoint{
		Namespace: namespace,
		Service:   selectedService.Name,
		Port:      port,
	}, nil
}

func findLokiNamespace(namespaces []v1.Namespace) (string, error) {
	switch len(namespaces) {
	case 0:
		return "", fmt.Errorf("failed to find loki namespace by label %s=%s", lokiNamespaceLabelKey, lokiNamespaceLabelValue)
	case 1:
		return namespaces[0].Name, nil
	default:
		return "", fmt.Errorf("found multiple loki namespaces by label %s=%s", lokiNamespaceLabelKey, lokiNamespaceLabelValue)
	}
}

func findLokiService(services []v1.Service) (*v1.Service, error) {
	switch len(services) {
	case 0:
		return nil, fmt.Errorf("failed to find loki gateway service by label %s=%s", lokiGatewayLabelKey, lokiGatewayLabelValue)
	case 1:
		return &services[0], nil
	default:
		return nil, fmt.Errorf("found multiple loki gateway services by label %s=%s", lokiGatewayLabelKey, lokiGatewayLabelValue)
	}
}

func findLokiServicePort(service *v1.Service) int {
	if service == nil {
		return 0
	}
	// Users only need to label the Loki gateway service; the proxy infers the
	// actual HTTP port from common service port conventions.
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
