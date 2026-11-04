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
	"k8s.io/klog/v2"

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

type expansionMeta struct {
	ClusterId   int64 `uri:"clusterId" binding:"required"`
	ExpansionId int64 `uri:"expansionId" binding:"required"`
}

type expansionLogsQuery struct {
	Step  string `form:"step" binding:"omitempty,oneof=bootstrap add-node"`
	Watch bool   `form:"watch"`
}

func (cr *clusterRouter) getExpansionCapabilities(c *gin.Context) {
	r := httputils.NewResponse()

	var opt expandClusterMeta
	if err := c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	result, err := cr.c.ClusterExpand().Config(c, opt.ClusterId)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = result
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) listExpansionNodes(c *gin.Context) {
	r := httputils.NewResponse()

	var opt expandClusterMeta
	if err := c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	result, err := cr.c.ClusterExpand().ListNodes(c, opt.ClusterId)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = result
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) addExpansionNodes(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt expandClusterMeta
		req types.CreateClusterNodesRequest
	)
	if err := httputils.ShouldBindAny(c, &req, &opt, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	result, err := cr.c.ClusterExpand().AddNodes(c, opt.ClusterId, &req)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = result
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) removeExpansionNode(c *gin.Context) {
	r := httputils.NewResponse()

	var opt expandNodeMeta
	if err := c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := cr.c.ClusterExpand().RemoveNode(c, opt.ClusterId, opt.NodeId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) createExpansion(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt expandClusterMeta
		req types.CreateClusterExpansionRequest
	)
	if err := httputils.ShouldBindAny(c, &req, &opt, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	expansionId, err := cr.c.ClusterExpand().Create(c, opt.ClusterId, &req)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = types.CreateClusterExpansionResponse{ExpansionId: expansionId}
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) createOrUpdateExpansionPlan(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt expandClusterMeta
		req types.SaveClusterExpansionPlanConfigRequest
	)
	if err := httputils.ShouldBindAny(c, &req, &opt, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := cr.c.ClusterExpand().CreateOrUpdateExpansionPlan(c, opt.ClusterId, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	// 返回保存后的计划表示，避免客户端再发起一次 GET
	result, err := cr.c.ClusterExpand().GetPlan(c, opt.ClusterId)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = result
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) generateExpansionJoinParams(c *gin.Context) {
	r := httputils.NewResponse()

	var opt expandClusterMeta
	if err := c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	result, err := cr.c.ClusterExpand().GenerateJoinParams(c, opt.ClusterId)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	c.Header("Cache-Control", "no-store") // join 参数包含 bootstrap token，禁止中间层与浏览器缓存
	c.Header("Pragma", "no-cache")

	r.Result = result
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) getExpansionPlan(c *gin.Context) {
	r := httputils.NewResponse()

	var opt expandClusterMeta
	if err := c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	result, err := cr.c.ClusterExpand().GetPlan(c, opt.ClusterId)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = result
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) deleteExpansionPlan(c *gin.Context) {
	r := httputils.NewResponse()

	var opt expandClusterMeta
	if err := c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := cr.c.ClusterExpand().DeletePlan(c, opt.ClusterId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) listExpansions(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt        expandClusterMeta
		listOption types.ListOptions
	)
	if err := c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := httputils.BindListOptionsWithUser(c, &listOption); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	result, err := cr.c.ClusterExpand().List(c, opt.ClusterId, listOption)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = result
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) getExpansion(c *gin.Context) {
	r := httputils.NewResponse()

	var opt expansionMeta
	if err := c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	result, err := cr.c.ClusterExpand().Get(c, opt.ClusterId, opt.ExpansionId)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = result
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) getExpansionLogs(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt   expansionMeta
		query expansionLogsQuery
	)
	if err := httputils.ShouldBindAny(c, nil, &opt, &query); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := cr.c.ClusterExpand().Logs(c, opt.ClusterId, opt.ExpansionId, query.Step, query.Watch, c.Writer, c.Request); err != nil {
		// 日志流一旦开始写出，响应头已发送，不能再写 JSON 业务错误
		if c.Writer.Written() {
			klog.Errorf("stream expansion(%d) logs: %v", opt.ExpansionId, err)
			c.Abort()
			return
		}
		httputils.SetFailed(c, r, err)
		return
	}
}
