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

package networkpolicy

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

// networkPolicyMeta path 参数：集群 id（与 /pixiu/clusters 其他接口一致）+ 命名空间 + 策略名
type networkPolicyMeta struct {
	ClusterId int64  `uri:"clusterId" binding:"required"`
	Namespace string `uri:"namespace" binding:"required"`
	Name      string `uri:"name"`
}

func (r *networkPolicyRouter) listNetworkPolicies(c *gin.Context) {
	resp := httputils.NewResponse()

	var meta networkPolicyMeta
	if err := c.ShouldBindUri(&meta); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	user, err := httputils.GetUserFromContext(c)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	if resp.Result, err = r.c.NetworkPolicy().List(c, user, meta.ClusterId, meta.Namespace); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *networkPolicyRouter) getNetworkPolicy(c *gin.Context) {
	resp := httputils.NewResponse()

	var meta networkPolicyMeta
	if err := c.ShouldBindUri(&meta); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	user, err := httputils.GetUserFromContext(c)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	if resp.Result, err = r.c.NetworkPolicy().Get(c, user, meta.ClusterId, meta.Namespace, meta.Name); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *networkPolicyRouter) createNetworkPolicy(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		meta networkPolicyMeta
		req  types.NetworkPolicyCreateRequest
	)
	if err := httputils.ShouldBindAny(c, &req, &meta, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	user, err := httputils.GetUserFromContext(c)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	if resp.Result, err = r.c.NetworkPolicy().Create(c, user, meta.ClusterId, meta.Namespace, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *networkPolicyRouter) updateNetworkPolicy(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		meta networkPolicyMeta
		req  types.NetworkPolicyUpdateRequest
	)
	if err := httputils.ShouldBindAny(c, &req, &meta, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	user, err := httputils.GetUserFromContext(c)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	if resp.Result, err = r.c.NetworkPolicy().Update(c, user, meta.ClusterId, meta.Namespace, meta.Name, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *networkPolicyRouter) deleteNetworkPolicy(c *gin.Context) {
	resp := httputils.NewResponse()

	var meta networkPolicyMeta
	if err := c.ShouldBindUri(&meta); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	user, err := httputils.GetUserFromContext(c)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	if err = r.c.NetworkPolicy().Delete(c, user, meta.ClusterId, meta.Namespace, meta.Name); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}
