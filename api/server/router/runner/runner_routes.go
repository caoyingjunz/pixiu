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
	res := httputils.NewResponse()

	var (
		req types.CreateRunnerRequest
		err error
	)
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, res, err)
		return
	}
	if err = r.c.Runner().Create(c, &req); err != nil {
		httputils.SetFailed(c, res, err)
		return
	}

	httputils.SetSuccess(c, res)
}

func (r *runnerRouter) updateRunner(c *gin.Context) {
	res := httputils.NewResponse()

	// 先读取原始请求体，用于调试
	bodyBytes, _ := c.GetRawData()
	log.Printf("updateRunner 收到原始请求: %s", string(bodyBytes))
	// 重新填充请求体，供 ShouldBindJSON 使用
	c.Request.Body = c.Request.Body // 这里可能需要重新设置，但 Gin 可能已经处理了

	var (
		opt RunnerMeta
		req types.UpdateRunnerRequest
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		log.Printf("updateRunner ShouldBindUri 失败: %v", err)
		httputils.SetFailed(c, res, err)
		return
	}
	// 重新绑定，因为我们之前读取了 raw data
	if err = c.ShouldBindJSON(&req); err != nil {
		log.Printf("updateRunner ShouldBindJSON 失败: %v", err)
		httputils.SetFailed(c, res, err)
		return
	}
	// 打印解析后的 req
	reqJson, _ := json.Marshal(req)
	log.Printf("updateRunner 解析后的 req: %s", string(reqJson))
	if err = r.c.Runner().Update(c, opt.RunnerId, &req); err != nil {
		httputils.SetFailed(c, res, err)
		return
	}

	httputils.SetSuccess(c, res)
}

func (r *runnerRouter) deleteRunner(c *gin.Context) {
	res := httputils.NewResponse()

	var (
		opt RunnerMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, res, err)
		return
	}
	if err = r.c.Runner().Delete(c, opt.RunnerId); err != nil {
		httputils.SetFailed(c, res, err)
		return
	}

	httputils.SetSuccess(c, res)
}

func (r *runnerRouter) getRunner(c *gin.Context) {
	res := httputils.NewResponse()

	var (
		opt RunnerMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, res, err)
		return
	}
	if res.Result, err = r.c.Runner().Get(c, opt.RunnerId); err != nil {
		httputils.SetFailed(c, res, err)
		return
	}

	httputils.SetSuccess(c, res)
}

func (r *runnerRouter) listRunners(c *gin.Context) {
	res := httputils.NewResponse()

	var (
		listOption types.RunnerListOptions
		err        error
	)
	if err = httputils.ShouldBindAny(c, nil, nil, &listOption); err != nil {
		httputils.SetFailed(c, res, err)
		return
	}
	if res.Result, err = r.c.Runner().List(c, listOption); err != nil {
		httputils.SetFailed(c, res, err)
		return
	}

	httputils.SetSuccess(c, res)
}
