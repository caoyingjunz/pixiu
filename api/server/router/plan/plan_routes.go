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

type planMeta struct {
	PlanId int64 `uri:"planId" binding:"required"`
}

// 创建部署计划，同时创建配置和节点
func (t *planRouter) createPlan(c *gin.Context) {
	r := httputils.NewResponse()

	var req types.CreatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := t.c.Plan().Create(c, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (t *planRouter) updatePlan(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt planMeta
		req types.UpdatePlanRequest
		err error
	)
	if err = httputils.ShouldBindAny(c, &req, &opt, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = t.c.Plan().Update(c, opt.PlanId, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (t *planRouter) deletePlan(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt planMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = t.c.Plan().Delete(c, opt.PlanId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (t *planRouter) getPlan(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt planMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = t.c.Plan().Get(c, opt.PlanId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// getPlanWithSubResources
// 获取 plan
// 获取 configs
// 获取 nodes
func (t *planRouter) getPlanWithSubResources(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt planMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = t.c.Plan().GetWithSubResources(c, opt.PlanId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (t *planRouter) listPlans(c *gin.Context) {
	r := httputils.NewResponse()

	var err error
	if r.Result, err = t.c.Plan().List(c); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (t *planRouter) startPlan(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt planMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = t.c.Plan().Start(c, opt.PlanId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (t *planRouter) stopPlan(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt planMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = t.c.Plan().Stop(c, opt.PlanId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
