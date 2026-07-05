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

// tryAuthenticatedServiceProxy 在携带上游 Basic 认证时，绕过 K8s service proxy 转发。
// apiserver 的 service proxy 会剥离 Authorization，导致 ES 等上游服务返回 401。
func (p *proxyRouter) tryAuthenticatedServiceProxy(
	c *gin.Context,
	config *rest.Config,
	clusterName string,
	upstreamAuth string,
) (handled bool, err error) {
	k8sPath := c.Request.URL.Path[len(proxyBaseURL+"/"+clusterName):]
	target, ok := parseServiceProxyPath(k8sPath)
	if !ok {
		klog.V(4).Infof("skip authenticated upstream proxy, path not service proxy: %q", k8sPath)
		return false, nil
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return true, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	resp, err := forwardAuthenticatedRequest(c.Request.Context(), config, clientset, target, c.Request, upstreamAuth)
	if err != nil {
		return true, err
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}
	c.Status(resp.StatusCode)
	_, err = io.Copy(c.Writer, resp.Body)
	return true, err
}

func forwardAuthenticatedRequest(
	ctx context.Context,
	config *rest.Config,
	clientset kubernetes.Interface,
	target *serviceProxyTarget,
	req *http.Request,
	upstreamAuth string,
) (*http.Response, error) {
	if resp, err := directServiceRequest(ctx, clientset, target, req, upstreamAuth); err == nil {
		return resp, nil
	} else {
		klog.V(4).Infof("direct upstream request failed, fallback to port-forward: %v", err)
	}
	return portForwardServiceRequest(ctx, config, clientset, target, req, upstreamAuth)
}

func directServiceRequest(
	ctx context.Context,
	clientset kubernetes.Interface,
	target *serviceProxyTarget,
	req *http.Request,
	upstreamAuth string,
) (*http.Response, error) {
	host, port, err := resolveServiceDialAddress(ctx, clientset, target)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("http://%s:%d%s", host, port, target.path)
	if req.URL.RawQuery != "" {
		url += "?" + req.URL.RawQuery
	}

	proxyReq, err := cloneUpstreamRequest(ctx, req, url, upstreamAuth)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 120 * time.Second}
	return client.Do(proxyReq)
}

func portForwardServiceRequest(
	ctx context.Context,
	config *rest.Config,
	clientset kubernetes.Interface,
	target *serviceProxyTarget,
	req *http.Request,
	upstreamAuth string,
) (*http.Response, error) {
	podName, remotePort, err := pickPodPort(ctx, clientset, target)
	if err != nil {
		return nil, err
	}

	localPort, err := reserveLocalPort()
	if err != nil {
		return nil, err
	}

	stopCh := make(chan struct{})
	readyCh := make(chan struct{})
	defer close(stopCh)

	roundTripper, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create spdy round tripper: %w", err)
	}

	pfURL := clientset.CoreV1().RESTClient().Post().
		Namespace(target.namespace).
		Resource("pods").
		Name(podName).
		SubResource("portforward").
		URL()

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, pfURL)
	ports := []string{fmt.Sprintf("%d:%d", localPort, remotePort)}
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
		return nil, fmt.Errorf("port-forward failed: %w", err)
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
		return nil, fmt.Errorf("upstream request through port-forward failed: %w", err)
	}
	return resp, nil
}

func resolveServiceDialAddress(
	ctx context.Context,
	clientset kubernetes.Interface,
	target *serviceProxyTarget,
) (string, int32, error) {
	svc, err := clientset.CoreV1().Services(target.namespace).Get(ctx, target.service, metav1.GetOptions{})
	if err != nil {
		return "", 0, fmt.Errorf("failed to get service %s/%s: %w", target.namespace, target.service, err)
	}

	servicePort, targetPort, err := resolveServicePorts(svc, target.port)
	if err != nil {
		return "", 0, err
	}

	if svc.Spec.ClusterIP != "" && svc.Spec.ClusterIP != "None" {
		return svc.Spec.ClusterIP, servicePort, nil
	}

	eps, err := clientset.CoreV1().Endpoints(target.namespace).Get(ctx, target.service, metav1.GetOptions{})
	if err != nil {
		return "", 0, fmt.Errorf("failed to get endpoints %s/%s: %w", target.namespace, target.service, err)
	}

	for _, subset := range eps.Subsets {
		for _, addr := range subset.Addresses {
			if addr.IP == "" {
				continue
			}
			for _, port := range subset.Ports {
				if portMatchesTarget(port, targetPort) {
					return addr.IP, port.Port, nil
				}
			}
		}
	}

	return "", 0, fmt.Errorf("no ready endpoint found for service %s/%s", target.namespace, target.service)
}

func pickPodPort(
	ctx context.Context,
	clientset kubernetes.Interface,
	target *serviceProxyTarget,
) (string, int32, error) {
	svc, err := clientset.CoreV1().Services(target.namespace).Get(ctx, target.service, metav1.GetOptions{})
	if err != nil {
		return "", 0, fmt.Errorf("failed to get service %s/%s: %w", target.namespace, target.service, err)
	}

	_, targetPort, err := resolveServicePorts(svc, target.port)
	if err != nil {
		return "", 0, err
	}

	eps, err := clientset.CoreV1().Endpoints(target.namespace).Get(ctx, target.service, metav1.GetOptions{})
	if err != nil {
		return "", 0, fmt.Errorf("failed to get endpoints %s/%s: %w", target.namespace, target.service, err)
	}

	for _, subset := range eps.Subsets {
		for _, addr := range subset.Addresses {
			if addr.TargetRef == nil || addr.TargetRef.Kind != "Pod" || addr.TargetRef.Name == "" {
				continue
			}
			for _, port := range subset.Ports {
				if portMatchesTarget(port, targetPort) {
					return addr.TargetRef.Name, port.Port, nil
				}
			}
		}
	}

	return "", 0, fmt.Errorf("no ready pod found for service %s/%s", target.namespace, target.service)
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
		if lowerKey == "authorization" || lowerKey == strings.ToLower(upstreamAuthorizationHeader) {
			continue
		}
		if lowerKey == "host" || lowerKey == "cookie" {
			continue
		}
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	req.Header.Set("Authorization", upstreamAuth)
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
