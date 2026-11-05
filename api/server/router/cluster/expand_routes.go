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

package cluster

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type expandClusterMeta struct {
	ClusterId int64 `uri:"clusterId" binding:"required"`
}

type expandNodeMeta struct {
	ClusterId int64 `uri:"clusterId" binding:"required"`
	NodeId    int64 `uri:"nodeId" binding:"required"`
}

// 扩容计划 API
func (cr *clusterRouter) createOrUpdateExpansionPlan(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt expandClusterMeta
		req types.CreateOrUpdateClusterExpansionPlanRequest
		err error
	)
	if err = httputils.ShouldBindAny(c, &req, &opt, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = cr.c.ClusterExpand().CreateOrUpdateExpansionPlan(c, opt.ClusterId, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) deleteExpansionPlan(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt expandClusterMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = cr.c.ClusterExpand().DeleteExpansionPlan(c, opt.ClusterId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) getExpansionPlan(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt expandClusterMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = cr.c.ClusterExpand().GetExpansionPlan(c, opt.ClusterId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) getExpansionCapabilities(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt expandClusterMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = cr.c.ClusterExpand().Capabilities(c, opt.ClusterId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) listExpansionNodes(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt expandClusterMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = cr.c.ClusterExpand().ListNodes(c, opt.ClusterId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) addExpansionNodes(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt expandClusterMeta
		req types.CreateClusterNodesRequest
		err error
	)
	if err = httputils.ShouldBindAny(c, &req, &opt, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = cr.c.ClusterExpand().AddNodes(c, opt.ClusterId, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) removeExpansionNode(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt expandNodeMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = cr.c.ClusterExpand().RemoveNode(c, opt.ClusterId, opt.NodeId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) startExpansion(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt expandClusterMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = cr.c.ClusterExpand().Start(c, opt.ClusterId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) generateExpansionJoinParams(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt expandClusterMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = cr.c.ClusterExpand().GenerateJoinParams(c, opt.ClusterId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	c.Header("Cache-Control", "no-store") // join 参数包含 bootstrap token，禁止中间层与浏览器缓存
	c.Header("Pragma", "no-cache")

	httputils.SetSuccess(c, r)
}
