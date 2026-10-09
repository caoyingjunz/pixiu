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
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/klog/v2"

	pixiuclient "github.com/caoyingjunz/pixiu/pkg/client"
	datasourceauth "github.com/caoyingjunz/pixiu/pkg/datasource/auth"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

var (
	serviceProxyPathRe = regexp.MustCompile(`^/api/v1/namespaces/([^/]+)/services/([^/:]+):(\d+)/proxy(/.*)?$`)
	freePortMu         sync.Mutex
)

type serviceProxyTarget struct {
	namespace string
	service   string
	port      int
	path      string
}

type podProxyTarget struct {
	namespace  string
	podName    string
	remotePort int32
	path       string
}

func parseServiceProxyPath(k8sPath string) (*serviceProxyTarget, bool) {
	m := serviceProxyPathRe.FindStringSubmatch(k8sPath)
	if m == nil {
		return nil, false
	}
	port, err := strconv.Atoi(m[3])
	if err != nil || port <= 0 {
		return nil, false
	}
	path := m[4]
	if path == "" {
		path = "/"
	}
	return &serviceProxyTarget{
		namespace: m[1],
		service:   m[2],
		port:      port,
		path:      path,
	}, true
}

// tryProxyAuthenticatedService forwards authenticated Service requests through a Pod.
func (p *proxyRouter) tryProxyAuthenticatedService(c *gin.Context, clientSet kubernetes.Interface, config *rest.Config, clusterName string, upstreamAuth string) (handled bool, err error) {
	target, ok := serviceProxyTargetFromRequest(c, clusterName)
	if !ok {
		klog.V(4).Infof("skip authenticated upstream proxy, path not service proxy: %q", c.Request.URL.EscapedPath())
		return false, nil
	}
	podTarget, err := pickOnePodForProxy(c.Request.Context(), clientSet, target)
	if err != nil {
		return true, err
	}
	return true, proxyPodRequest(c, config, clientSet, podTarget, upstreamAuth)
}

// tryProxyDatasourceService handles saved datasource credentials through a Pod.
func (p *proxyRouter) tryProxyDatasourceService(
	c *gin.Context,
	clientSet kubernetes.Interface,
	config *rest.Config,
	clusterName string,
	datasource *types.Datasource,
) (bool, error) {
	if !datasourceauth.RequiresPodProxy(datasource) {
		return false, nil
	}

	target, ok := serviceProxyTargetFromRequest(c, clusterName)
	if !ok {
		return false, nil
	}
	podTarget, err := pickOnePodForProxy(c.Request.Context(), clientSet, target)
	if err != nil {
		return true, err
	}
	upstreamAuth, err := p.prepareDatasourceServiceRequest(c, clientSet, config, podTarget, target.path, datasource)
	if err != nil {
		return true, err
	}
	return true, proxyPodRequest(c, config, clientSet, podTarget, upstreamAuth)
}

func proxyPodRequest(
	c *gin.Context,
	config *rest.Config,
	clientSet kubernetes.Interface,
	podTarget *podProxyTarget,
	upstreamAuth string,
) error {
	response, err := proxyViaPodPortForward(c.Request.Context(), config, clientSet, podTarget, c.Request, upstreamAuth)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	copyProxyResponse(c, response)
	return nil
}

func (p *proxyRouter) prepareExternalDatasourceRequest(c *gin.Context, target *url.URL, datasource *types.Datasource) (string, error) {
	return datasourceauth.Prepare(c.Request.Context(), datasource, target, func(request *http.Request) (*http.Response, error) {
		return (&http.Client{Transport: externalProxyTransport, Timeout: externalProxyRequestTimeout}).Do(request)
	})
}

func (p *proxyRouter) prepareDatasourceServiceRequest(
	c *gin.Context,
	clientSet kubernetes.Interface,
	config *rest.Config,
	podTarget *podProxyTarget,
	requestPath string,
	datasource *types.Datasource,
) (string, error) {
	target := &url.URL{Scheme: "http", Host: "upstream.local", Path: requestPath, RawQuery: c.Request.URL.RawQuery}
	upstreamAuth, err := datasourceauth.Prepare(c.Request.Context(), datasource, target, func(request *http.Request) (*http.Response, error) {
		loginTarget := *podTarget
		loginTarget.path = request.URL.Path
		return proxyViaPodPortForward(c.Request.Context(), config, clientSet, &loginTarget, request, "")
	})
	if err == nil {
		c.Request.URL.RawQuery = target.RawQuery
	}
	return upstreamAuth, err
}

func serviceProxyTargetFromRequest(c *gin.Context, clusterName string) (*serviceProxyTarget, bool) {
	escapedPath := c.Request.URL.EscapedPath()
	prefix := proxyBaseURL + "/" + clusterName
	if strings.HasPrefix(escapedPath, prefix) {
		escapedPath = escapedPath[len(prefix):]
	}
	if escapedPath == "" {
		escapedPath = "/"
	}
	return parseServiceProxyPath(escapedPath)
}

func copyProxyResponse(c *gin.Context, response *http.Response) {
	for key, values := range response.Header {
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}
	c.Status(response.StatusCode)
	_, _ = io.Copy(c.Writer, response.Body)
}

func pickOnePodForProxy(ctx context.Context, clientSet kubernetes.Interface, target *serviceProxyTarget) (*podProxyTarget, error) {
	svc, err := clientSet.CoreV1().Services(target.namespace).Get(ctx, target.service, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get service %s/%s: %w", target.namespace, target.service, err)
	}

	_, targetPort, err := resolveServicePorts(svc, target.port)
	if err != nil {
		return nil, err
	}

	eps, err := clientSet.CoreV1().Endpoints(target.namespace).Get(ctx, target.service, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get endpoints %s/%s: %w", target.namespace, target.service, err)
	}

	for _, subset := range eps.Subsets {
		for _, addr := range subset.Addresses {
			if addr.TargetRef == nil || addr.TargetRef.Kind != "Pod" || addr.TargetRef.Name == "" {
				continue
			}
			for _, port := range subset.Ports {
				if !portMatchesTarget(port, targetPort) {
					continue
				}
				klog.V(2).Infof(
					"selected pod %s/%s:%d for service %s/%s",
					target.namespace, addr.TargetRef.Name, port.Port, target.namespace, target.service,
				)
				return &podProxyTarget{
					namespace:  target.namespace,
					podName:    addr.TargetRef.Name,
					remotePort: port.Port,
					path:       target.path,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("no ready pod found for service %s/%s", target.namespace, target.service)
}

func proxyViaPodPortForward(
	ctx context.Context,
	config *rest.Config,
	clientSet kubernetes.Interface,
	target *podProxyTarget,
	req *http.Request,
	upstreamAuth string,
) (*http.Response, error) {
	localPort, err := reserveLocalPort()
	if err != nil {
		return nil, err
	}

	stopCh := make(chan struct{})
	readyCh := make(chan struct{})
	defer close(stopCh)

	roundTripper, upgrader, err := pixiuclient.RoundTripperFor(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create spdy round tripper: %w", err)
	}

	pfURL := clientSet.CoreV1().RESTClient().
		Post().
		Namespace(target.namespace).
		Resource("pods").
		Name(target.podName).
		SubResource("portforward").
		URL()

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, pfURL)
	ports := []string{fmt.Sprintf("%d:%d", localPort, target.remotePort)}
	forwarder, err := portforward.New(dialer, ports, stopCh, readyCh, io.Discard, io.Discard)
	if err != nil {
		return nil, fmt.Errorf("failed to create port forwarder: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- forwarder.ForwardPorts()
	}()

	select {
	case <-readyCh:
	case err := <-errCh:
		return nil, fmt.Errorf("port-forward to pod %s/%s failed: %w", target.namespace, target.podName, err)
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	url := fmt.Sprintf("http://127.0.0.1:%d%s", localPort, target.path)
	if req.URL.RawQuery != "" {
		url += "?" + req.URL.RawQuery
	}

	proxyReq, err := cloneUpstreamRequest(ctx, req, url, upstreamAuth)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		return nil, fmt.Errorf(
			"upstream request through pod %s/%s port-forward failed: %w",
			target.namespace, target.podName, err,
		)
	}
	return resp, nil
}

func resolveServicePorts(svc *corev1.Service, requestedPort int) (servicePort int32, targetPort intstr.IntOrString, err error) {
	for _, port := range svc.Spec.Ports {
		if int(port.Port) == requestedPort {
			return port.Port, port.TargetPort, nil
		}
	}
	return 0, intstr.IntOrString{}, fmt.Errorf("service port %d not found on %s/%s", requestedPort, svc.Namespace, svc.Name)
}

func portMatchesTarget(port corev1.EndpointPort, targetPort intstr.IntOrString) bool {
	if targetPort.Type == intstr.Int {
		return port.Port == targetPort.IntVal
	}
	return port.Name == targetPort.StrVal
}

func cloneUpstreamRequest(ctx context.Context, orig *http.Request, url string, upstreamAuth string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, orig.Method, url, orig.Body)
	if err != nil {
		return nil, err
	}

	for key, values := range orig.Header {
		lowerKey := strings.ToLower(key)
		if lowerKey == "authorization" ||
			lowerKey == strings.ToLower(upstreamDatasourceIDHeader) ||
			lowerKey == strings.ToLower(externalProxyAuthorizationHeaderKey) {
			continue
		}
		if lowerKey == "host" || lowerKey == "cookie" {
			continue
		}
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	if upstreamAuth != "" {
		req.Header.Set("Authorization", upstreamAuth)
	}
	if orig.ContentLength > 0 {
		req.ContentLength = orig.ContentLength
	}
	return req, nil
}

func reserveLocalPort() (int, error) {
	freePortMu.Lock()
	defer freePortMu.Unlock()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	_, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return 0, err
	}
	return port, nil
}
