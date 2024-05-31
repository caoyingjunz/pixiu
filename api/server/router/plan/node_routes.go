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

package plan

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type planNodeMeta struct {
	planMeta `json:",inline"`

	NodeId int64 `uri:"nodeId" binding:"required"`
}

func (t *planRouter) createPlanNode(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt planMeta
		req types.CreatePlanNodeRequest
		err error
	)
	if err = httputils.ShouldBindAny(c, &req, &opt, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = t.c.Plan().CreateNode(c, opt.PlanId, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (t *planRouter) updatePlanNode(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt planNodeMeta
		req types.UpdatePlanNodeRequest
		err error
	)
	if err = httputils.ShouldBindAny(c, &req, &opt, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = t.c.Plan().UpdateNode(c, opt.PlanId, opt.NodeId, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (t *planRouter) deletePlanNode(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt planNodeMeta
		err error
	)
	if err = httputils.ShouldBindAny(c, nil, &opt, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = t.c.Plan().DeleteNode(c, opt.PlanId, opt.NodeId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (t *planRouter) getPlanNode(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt planNodeMeta
		err error
	)
	if err = httputils.ShouldBindAny(c, nil, &opt, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = t.c.Plan().GetNode(c, opt.PlanId, opt.NodeId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (t *planRouter) listPlanNodes(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt planMeta
		err error
	)
	if err = httputils.ShouldBindAny(c, nil, &opt, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = t.c.Plan().ListNodes(c, opt.PlanId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
