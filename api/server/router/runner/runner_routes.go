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

package runner

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type RunnerMeta struct {
	RunnerId int64 `uri:"runnerId" binding:"required"`
}

func (r *runnerRouter) createRunner(c *gin.Context) {
	result := httputils.NewResponse()

	var (
		req types.CreateRunnerRequest
		err error
	)
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	if err = r.c.Runner().Create(c, &req); err != nil {
		httputils.SetFailed(c, result, err)
		return
	}

	httputils.SetSuccess(c, result)
}

func (r *runnerRouter) updateRunner(c *gin.Context) {
	result := httputils.NewResponse()
	var (
		idMeta RunnerMeta
		req    types.UpdateRunnerRequest
		err    error
	)
	if err = httputils.ShouldBindAny(c, &req, &idMeta, nil); err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	req.Id = idMeta.RunnerId
	if err = r.c.Runner().Update(c, &req); err != nil {
		httputils.SetFailed(c, result, err)
		return
	}

	httputils.SetSuccess(c, result)
}

func (r *runnerRouter) deleteRunner(c *gin.Context) {
	result := httputils.NewResponse()

	var (
		idMeta RunnerMeta
		err    error
	)
	if err = c.ShouldBindUri(&idMeta); err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	if err = r.c.Runner().Delete(c, idMeta.RunnerId); err != nil {
		httputils.SetFailed(c, result, err)
		return
	}

	httputils.SetSuccess(c, result)
}

func (r *runnerRouter) getRunner(c *gin.Context) {
	result := httputils.NewResponse()

	var (
		idMeta RunnerMeta
		err    error
	)
	if err = c.ShouldBindUri(&idMeta); err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	if result.Result, err = r.c.Runner().Get(c, idMeta.RunnerId); err != nil {
		httputils.SetFailed(c, result, err)
		return
	}

	httputils.SetSuccess(c, result)
}

func (r *runnerRouter) listRunners(c *gin.Context) {
	result := httputils.NewResponse()

	var (
		listOption types.ListOptions
		err        error
	)
	if err = httputils.ShouldBindAny(c, nil, nil, &listOption); err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	if result.Result, err = r.c.Runner().List(c, listOption); err != nil {
		httputils.SetFailed(c, result, err)
		return
	}

	httputils.SetSuccess(c, result)
}
