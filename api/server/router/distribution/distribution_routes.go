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

package distribution

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type distributionMeta struct {
	DistributionId int64 `uri:"distributionId" binding:"required"`
}

func (r *distributionRouter) createDistribution(c *gin.Context) {
	resp := httputils.NewResponse()

	var req types.CreateDistributionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Distribution().CreateDistribution(c, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	httputils.SetSuccess(c, resp)
}

func (r *distributionRouter) updateDistribution(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		opt distributionMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	var req types.UpdateDistributionRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err = r.c.Distribution().UpdateDistribution(c, opt.DistributionId, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	httputils.SetSuccess(c, resp)
}

func (r *distributionRouter) deleteDistribution(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		opt distributionMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err = r.c.Distribution().DeleteDistribution(c, opt.DistributionId); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	httputils.SetSuccess(c, resp)
}

func (r *distributionRouter) getDistribution(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		opt distributionMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if resp.Result, err = r.c.Distribution().GetDistribution(c, opt.DistributionId); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	httputils.SetSuccess(c, resp)
}

func (r *distributionRouter) listDistributions(c *gin.Context) {
	resp := httputils.NewResponse()

	var req types.ListDistributionRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	var err error
	if resp.Result, err = r.c.Distribution().ListDistributions(c, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	httputils.SetSuccess(c, resp)
}
