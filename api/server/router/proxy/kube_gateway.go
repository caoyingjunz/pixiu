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

package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/tunnel"
)

const kubeGatewayBaseURL = "/k8s"

func (p *proxyRouter) initKubeGatewayRoutes(ginEngine *gin.Engine) {
	ginEngine.Any("/k8s/:clusterName/*act", p.kubeGatewayHandler)
}

func (p *proxyRouter) kubeGatewayHandler(c *gin.Context) {
	var cluster struct {
		Name string `uri:"clusterName" binding:"required"`
	}
	if err := c.ShouldBindUri(&cluster); err != nil {
		writeKubeStatus(c, http.StatusBadRequest, metav1.StatusReasonBadRequest, err.Error())
		return
	}

	user, err := httputils.GetUserFromRequest(c)
	if err != nil {
		writeKubeStatus(c, http.StatusUnauthorized, metav1.StatusReasonUnauthorized, "unauthorized")
		return
	}

	if accessCluster, err := httputils.GetKubeAccessClusterFromContext(c); err == nil {
		if accessCluster != cluster.Name {
			writeKubeStatus(c, http.StatusForbidden, metav1.StatusReasonForbidden, "token is not allowed for this cluster")
			return
		}
	}

	obj, err := p.c.Cluster().AuthorizeClusterAccessByName(c, user, cluster.Name)
	if err != nil {
		writeAuthzError(c, err)
		return
	}

	if obj.ConnectMode == model.ConnectModeTunnel {
		if tm := tunnel.Default(); tm == nil || !tm.AgentConnected(obj.Name) {
			writeKubeStatus(c, http.StatusServiceUnavailable, metav1.StatusReasonServiceUnavailable, "cluster agent is not connected")
			return
		}
	}

	clusterSet, err := p.c.Cluster().GetClusterSetByName(context.TODO(), cluster.Name)
	if err != nil {
		writeKubeStatus(c, http.StatusBadGateway, metav1.StatusReasonInternalError,
			fmt.Sprintf("failed to get cluster %q: %v", cluster.Name, err))
		return
	}

	config := clusterSet.Config
	transport, err := rest.TransportFor(config)
	if err != nil {
		writeKubeStatus(c, http.StatusInternalServerError, metav1.StatusReasonInternalError, err.Error())
		return
	}
	target, err := p.parseKubeGatewayTarget(*c.Request.URL, config.Host, cluster.Name)
	if err != nil {
		writeKubeStatus(c, http.StatusBadRequest, metav1.StatusReasonBadRequest, err.Error())
		return
	}

	c.Request.Header.Del("Authorization")
	c.Request.Header.Del("Cookie")

	klog.V(2).Infof("kube gateway proxying cluster=%s path=%s user=%d", cluster.Name, c.Request.URL.Path, user.Id)
	httpProxy := proxy.NewUpgradeAwareHandler(target, transport, false, false, nil)
	httpProxy.UpgradeTransport = proxy.NewUpgradeRequestRoundTripper(transport, transport)
	httpProxy.ServeHTTP(c.Writer, c.Request)
}

func (p *proxyRouter) parseKubeGatewayTarget(target url.URL, host string, name string) (*url.URL, error) {
	kubeURL, err := url.Parse(host)
	if err != nil {
		return nil, err
	}
	prefix := kubeGatewayBaseURL + "/" + name
	if !strings.HasPrefix(target.Path, prefix) {
		return nil, fmt.Errorf("invalid gateway path")
	}
	target.Path = target.Path[len(prefix):]
	if target.Path == "" {
		target.Path = "/"
	}
	target.Host = kubeURL.Host
	target.Scheme = kubeURL.Scheme
	return &target, nil
}

func writeAuthzError(c *gin.Context, err error) {
	msg := err.Error()
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "not found"):
		writeKubeStatus(c, http.StatusNotFound, metav1.StatusReasonNotFound, msg)
	case strings.Contains(lower, "unauthorized"):
		writeKubeStatus(c, http.StatusUnauthorized, metav1.StatusReasonUnauthorized, msg)
	default:
		writeKubeStatus(c, http.StatusForbidden, metav1.StatusReasonForbidden, msg)
	}
}

func writeKubeStatus(c *gin.Context, code int, reason metav1.StatusReason, message string) {
	c.AbortWithStatusJSON(code, metav1.Status{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Status",
			APIVersion: "v1",
		},
		Status:  metav1.StatusFailure,
		Message: message,
		Reason:  reason,
		Code:    int32(code),
	})
}
