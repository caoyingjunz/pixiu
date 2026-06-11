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
	"sort"
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
	defaultLokiScheme = "http"
	defaultLokiOrgID  = "fake"

	lokiNamespaceLabelKey   = "pixiu-loki"
	lokiNamespaceLabelValue = "true"
	lokiGatewayLabelKey     = "pixiu-loki-gateway"
	lokiGatewayLabelValue   = "true"
)

var preferredLokiServiceNames = []string{
	"loki-distributed-gateway",
	"loki-gateway",
	"loki",
}

type lokiClusterMeta struct {
	Cluster string `uri:"cluster" binding:"required"`
}

type lokiProxyOptions struct {
	Namespace string `form:"namespace"`
	Service   string `form:"service"`
	Port      int    `form:"port"`
	Scheme    string `form:"scheme"`
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
	opts, err := discoverLokiProxyOptions(c, clientSet)
	if err != nil {
		httputils.SetFailed(c, resp, err)
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
	return base + normalizeLokiAPIPath(act)
}

func ensureDefaultLokiOrgID(c *gin.Context) {
	if strings.TrimSpace(c.GetHeader("X-Scope-OrgID")) != "" {
		return
	}
	c.Request.Header.Set("X-Scope-OrgID", defaultLokiOrgID)
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

func discoverLokiProxyOptions(ctx context.Context, clientSet kubernetes.Interface) (lokiProxyOptions, error) {
	resolved := lokiProxyOptions{Scheme: defaultLokiScheme}

	nsList, err := clientSet.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", lokiNamespaceLabelKey, lokiNamespaceLabelValue),
	})
	if err != nil {
		return lokiProxyOptions{}, err
	}
	namespace := selectLokiNamespace(nsList.Items)
	if namespace == "" {
		return lokiProxyOptions{}, fmt.Errorf("failed to find loki namespace by label %s=%s", lokiNamespaceLabelKey, lokiNamespaceLabelValue)
	}
	resolved.Namespace = namespace

	serviceList, err := clientSet.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", lokiGatewayLabelKey, lokiGatewayLabelValue),
	})
	if err != nil {
		return lokiProxyOptions{}, err
	}
	selectedService := selectLokiService(serviceList.Items)
	if selectedService == nil {
		return lokiProxyOptions{}, fmt.Errorf("failed to find loki gateway service in namespace %q by label %s=%s", namespace, lokiGatewayLabelKey, lokiGatewayLabelValue)
	}
	port := selectLokiServicePort(selectedService)
	if port == 0 {
		return lokiProxyOptions{}, fmt.Errorf("failed to resolve a TCP port for loki service %q", selectedService.Name)
	}
	resolved.Service = selectedService.Name
	resolved.Port = port
	return resolved, nil
}

func selectLokiNamespace(namespaces []v1.Namespace) string {
	if len(namespaces) == 0 {
		return ""
	}
	sort.Slice(namespaces, func(i, j int) bool {
		return namespaces[i].Name < namespaces[j].Name
	})
	for _, namespace := range namespaces {
		if namespace.Labels[lokiNamespaceLabelKey] == lokiNamespaceLabelValue {
			return namespace.Name
		}
	}
	return namespaces[0].Name
}

func selectLokiService(services []v1.Service) *v1.Service {
	if len(services) == 0 {
		return nil
	}

	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})
	for _, preferredName := range preferredLokiServiceNames {
		for i := range services {
			service := &services[i]
			if service.Name == preferredName {
				return service
			}
		}
	}
	for i := range services {
		service := &services[i]
		if strings.Contains(service.Name, "loki") && strings.Contains(service.Name, "gateway") {
			return service
		}
	}
	return &services[0]
}

func selectLokiServicePort(service *v1.Service) int {
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
