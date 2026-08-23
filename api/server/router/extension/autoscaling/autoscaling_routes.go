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

package autoscaling

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type meta struct {
	CronHpaId int64 `uri:"cronHpaId"`
}

func (ar *autoscalingRouter) createCronHpa(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		req types.CronHpaRequest
		err error
	)
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = ar.c.Extension().Autoscaling().Create(c, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (ar *autoscalingRouter) listCronHpas(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opts types.CronHpaListOptions
		err  error
	)
	if err = httputils.ShouldBindAny(c, nil, nil, &opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = ar.c.Extension().Autoscaling().List(c, &opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (ar *autoscalingRouter) getCronHpa(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		err error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = ar.c.Extension().Autoscaling().Get(c, m.CronHpaId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (ar *autoscalingRouter) updateCronHpa(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		req types.CronHpaRequest
		err error
	)
	if err = httputils.ShouldBindAny(c, &req, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = ar.c.Extension().Autoscaling().Update(c, m.CronHpaId, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (ar *autoscalingRouter) deleteCronHpa(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		err error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = ar.c.Extension().Autoscaling().Delete(c, m.CronHpaId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (ar *autoscalingRouter) setCronHpaStatus(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		req types.CronHpaStatusRequest
		err error
	)
	if err = httputils.ShouldBindAny(c, &req, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = ar.c.Extension().Autoscaling().SetStatus(c, m.CronHpaId, req.Status); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (ar *autoscalingRouter) listCronHpaHistories(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m    meta
		opts types.CronHpaHistoryOptions
		err  error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = ar.c.Extension().Autoscaling().ListHistories(c, m.CronHpaId, opts.Limit); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}
