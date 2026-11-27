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

package cicd

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type pathMeta struct {
	PipelineId int64  `uri:"pipelineId"`
	RunId      int64  `uri:"runId"`
	ClusterId  int64  `uri:"clusterId"`
	Namespace  string `uri:"namespace"`
	Name       string `uri:"name"`
}

func (r *cicdRouter) engineStatus(c *gin.Context) {
	result := httputils.NewResponse()
	var meta pathMeta
	err := c.ShouldBindUri(&meta)
	if err == nil {
		result.Result, err = r.c.CICD().GetEngineStatus(c, meta.ClusterId)
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}

func (r *cicdRouter) createPipeline(c *gin.Context) {
	result := httputils.NewResponse()
	var req types.CreateCICDPipelineRequest
	var err error
	if err = httputils.BindCreateRequest(c, &req); err == nil {
		user, uErr := httputils.GetUserFromContext(c)
		if uErr != nil {
			err = uErr
		} else {
			req.UserName = user.Name
			result.Result, err = r.c.CICD().CreatePipeline(c, &req)
		}
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}

func (r *cicdRouter) updatePipeline(c *gin.Context) {
	result := httputils.NewResponse()
	var meta pathMeta
	var req types.UpdateCICDPipelineRequest
	err := httputils.ShouldBindAny(c, &req, &meta, nil)
	if err == nil {
		err = r.c.CICD().UpdatePipeline(c, meta.PipelineId, &req)
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}

func (r *cicdRouter) deletePipeline(c *gin.Context) {
	result := httputils.NewResponse()
	var meta pathMeta
	err := c.ShouldBindUri(&meta)
	if err == nil {
		err = r.c.CICD().DeletePipeline(c, meta.PipelineId)
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}

func (r *cicdRouter) getPipeline(c *gin.Context) {
	result := httputils.NewResponse()
	var meta pathMeta
	err := c.ShouldBindUri(&meta)
	if err == nil {
		result.Result, err = r.c.CICD().GetPipeline(c, meta.PipelineId)
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}

func (r *cicdRouter) listPipelines(c *gin.Context) {
	result := httputils.NewResponse()
	var query types.ListCICDPipelinesQuery
	err := c.ShouldBindQuery(&query)
	if err == nil {
		var total int64
		var items []types.CICDPipeline
		items, total, err = r.c.CICD().ListPipelines(c, &query)
		if err == nil {
			result.Result = map[string]interface{}{"items": items, "total": total}
		}
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}

func (r *cicdRouter) triggerRun(c *gin.Context) {
	result := httputils.NewResponse()
	var meta pathMeta
	var req types.RunPipelineRequest
	err := httputils.ShouldBindAny(c, &req, &meta, nil)
	if err == nil {
		user, uErr := httputils.GetUserFromContext(c)
		if uErr != nil {
			err = uErr
		} else {
			req.UserName = user.Name
			result.Result, err = r.c.CICD().TriggerRun(c, meta.PipelineId, &req)
		}
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}

func (r *cicdRouter) listRuns(c *gin.Context) {
	result := httputils.NewResponse()
	var query types.ListCICDRunsQuery
	err := c.ShouldBindQuery(&query)
	if err == nil {
		var total int64
		var items []types.CICDRun
		items, total, err = r.c.CICD().ListRuns(c, &query)
		if err == nil {
			result.Result = map[string]interface{}{"items": items, "total": total}
		}
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}

func (r *cicdRouter) getRun(c *gin.Context) {
	result := httputils.NewResponse()
	var meta pathMeta
	err := c.ShouldBindUri(&meta)
	if err == nil {
		result.Result, err = r.c.CICD().GetRun(c, meta.RunId)
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}

func (r *cicdRouter) cancelRun(c *gin.Context) {
	result := httputils.NewResponse()
	var meta pathMeta
	err := c.ShouldBindUri(&meta)
	if err == nil {
		err = r.c.CICD().CancelRun(c, meta.RunId)
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}

func (r *cicdRouter) retryRun(c *gin.Context) {
	result := httputils.NewResponse()
	var meta pathMeta
	err := c.ShouldBindUri(&meta)
	if err == nil {
		result.Result, err = r.c.CICD().RetryRun(c, meta.RunId)
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}

func (r *cicdRouter) runLogs(c *gin.Context) {
	var meta pathMeta
	if err := c.ShouldBindUri(&meta); err != nil {
		httputils.SetFailed(c, httputils.NewResponse(), err)
		return
	}
	if err := r.c.CICD().WatchRunLogs(c, meta.RunId, c.Writer, c.Request); err != nil {
		// SSE 已可能写出部分内容，这里仅记录，不再改写响应。
		return
	}
}

func (r *cicdRouter) listCredentials(c *gin.Context) {
	result := httputils.NewResponse()
	var meta pathMeta
	err := c.ShouldBindUri(&meta)
	if err == nil {
		result.Result, err = r.c.CICD().ListCredentials(c, meta.ClusterId, meta.Namespace)
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}

func (r *cicdRouter) createCredential(c *gin.Context) {
	result := httputils.NewResponse()
	var meta pathMeta
	var req types.CreateCICDCredentialRequest
	err := httputils.ShouldBindAny(c, &req, &meta, nil)
	if err == nil {
		err = r.c.CICD().CreateCredential(c, meta.ClusterId, meta.Namespace, &req)
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}

func (r *cicdRouter) deleteCredential(c *gin.Context) {
	result := httputils.NewResponse()
	var meta pathMeta
	err := c.ShouldBindUri(&meta)
	if err == nil {
		err = r.c.CICD().DeleteCredential(c, meta.ClusterId, meta.Namespace, meta.Name)
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}
