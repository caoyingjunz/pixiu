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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pixiuerrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/tunnel"
)

// TODO: 确认是否兼容 kubectl 的原生命令行
const kubeGatewayBaseURL = "/k8s"

func (p *proxyRouter) initKubeGatewayRoutes(ginEngine *gin.Engine) {
	ginEngine.Any("/k8s/:clusterName/*act", p.kubeGatewayHandler)
}

func (p *proxyRouter) kubeGatewayHandler(c *gin.Context) {
	var cluster struct {
		Name string `uri:"clusterName" binding:"required"`
	}
	if err := c.ShouldBindUri(&cluster); err != nil {
		httputils.WriteKubeError(c, http.StatusBadRequest, metav1.StatusReasonBadRequest, err.Error())
		return
	}

	user, err := httputils.GetUserFromContext(c)
	if err != nil {
		httputils.WriteKubeError(c, http.StatusUnauthorized, metav1.StatusReasonUnauthorized, "unauthorized")
		return
	}

	if accessCluster, err := httputils.GetKubeAccessClusterFromContext(c); err == nil {
		if accessCluster != cluster.Name {
			httputils.WriteKubeError(c, http.StatusForbidden, metav1.StatusReasonForbidden, "token is not allowed for this cluster")
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
			httputils.WriteKubeError(c, http.StatusServiceUnavailable, metav1.StatusReasonServiceUnavailable, "cluster agent is not connected")
			return
		}
	}

	target, err := p.parseKubeGatewayTarget(*c.Request.URL, obj.Name)
	if err != nil {
		httputils.WriteKubeError(c, http.StatusBadRequest, metav1.StatusReasonBadRequest, err.Error())
		return
	}

	if err = p.forwardToCluster(c, cluster.Name, target); err != nil {
		httputils.WriteKubeError(c, http.StatusBadGateway, metav1.StatusReasonInternalError, err.Error())
	}
}

func (p *proxyRouter) parseKubeGatewayTarget(target url.URL, name string) (*url.URL, error) {
	prefix := kubeGatewayBaseURL + "/" + name
	if !strings.HasPrefix(target.Path, prefix) {
		return nil, fmt.Errorf("invalid gateway path")
	}
	target.Path = target.Path[len(prefix):]
	if target.Path == "" {
		target.Path = "/"
	}
	return &target, nil
}

func writeAuthzError(c *gin.Context, err error) {
	if errors.Is(err, pixiuerrors.ErrUnauthorized) {
		httputils.WriteKubeError(c, http.StatusUnauthorized, metav1.StatusReasonUnauthorized, err.Error())
		return
	}
	if errors.Is(err, pixiuerrors.ErrClusterNotFound) || errors.Is(err, pixiuerrors.ErrUserNotFound) {
		httputils.WriteKubeError(c, http.StatusNotFound, metav1.StatusReasonNotFound, err.Error())
		return
	}
	httputils.WriteKubeError(c, http.StatusForbidden, metav1.StatusReasonForbidden, err.Error())
}
