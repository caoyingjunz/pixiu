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

package deployagent

import (
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
	deployctl "github.com/caoyingjunz/pixiu/pkg/controller/deployagent"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type router struct {
	c controller.PixiuInterface
}

func NewRouter(o *options.Options) {
	r := &router{c: o.Controller}
	r.initRoutes(o.HttpEngine)
}

func (r *router) initRoutes(engine *gin.Engine) {
	g := engine.Group("/pixiu/deploy-agents")

	// Agent 作业 API 必须在 /:agentId 之前注册，避免被参数路由吞掉
	g.POST("/heartbeat", r.heartbeat)
	g.GET("/tasks/claim", r.claim)
	g.POST("/tasks/:jobId/logs", r.logs)
	g.POST("/tasks/:jobId/result", r.result)
	g.GET("/tasks/:jobId/bundle", r.bundle)

	// 管理端 API（JWT）
	g.POST("", r.create)
	g.GET("", r.list)
	g.GET("/:agentId", r.get)
	g.DELETE("/:agentId", r.delete)
	g.GET("/:agentId/install", r.install)
}

func (r *router) create(c *gin.Context) {
	resp := httputils.NewResponse()
	var req types.CreateDeployAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	var err error
	if resp.Result, err = r.c.DeployAgent().Create(c, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) list(c *gin.Context) {
	resp := httputils.NewResponse()
	var err error
	if resp.Result, err = r.c.DeployAgent().List(c); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) get(c *gin.Context) {
	resp := httputils.NewResponse()
	id, err := strconv.ParseInt(c.Param("agentId"), 10, 64)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if resp.Result, err = r.c.DeployAgent().Get(c, id); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) delete(c *gin.Context) {
	resp := httputils.NewResponse()
	id, err := strconv.ParseInt(c.Param("agentId"), 10, 64)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err = r.c.DeployAgent().Delete(c, id); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) install(c *gin.Context) {
	resp := httputils.NewResponse()
	id, err := strconv.ParseInt(c.Param("agentId"), 10, 64)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if resp.Result, err = r.c.DeployAgent().GetInstall(c, id); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func tokenFrom(c *gin.Context) string {
	return c.GetHeader(deployctl.TokenHeader)
}

func (r *router) heartbeat(c *gin.Context) {
	resp := httputils.NewResponse()
	var req types.DeployAgentHeartbeatRequest
	_ = c.ShouldBindJSON(&req)
	if err := r.c.DeployAgent().Heartbeat(c, tokenFrom(c), &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) claim(c *gin.Context) {
	resp := httputils.NewResponse()
	job, err := r.c.DeployAgent().Claim(c, tokenFrom(c))
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = job
	httputils.SetSuccess(c, resp)
}

func (r *router) logs(c *gin.Context) {
	resp := httputils.NewResponse()
	jobId, err := strconv.ParseInt(c.Param("jobId"), 10, 64)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	var req types.DeployJobLogsRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err = r.c.DeployAgent().AppendLogs(c, tokenFrom(c), jobId, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) result(c *gin.Context) {
	resp := httputils.NewResponse()
	jobId, err := strconv.ParseInt(c.Param("jobId"), 10, 64)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	var req types.DeployJobResultRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err = r.c.DeployAgent().ReportResult(c, tokenFrom(c), jobId, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) bundle(c *gin.Context) {
	jobId, err := strconv.ParseInt(c.Param("jobId"), 10, 64)
	if err != nil {
		httputils.AbortFailedWithCode(c, http.StatusBadRequest, err)
		return
	}
	path, err := r.c.DeployAgent().BundlePath(c, tokenFrom(c), jobId)
	if err != nil {
		resp := httputils.NewResponse()
		httputils.SetFailed(c, resp, err)
		return
	}
	defer os.Remove(path)
	c.FileAttachment(path, "plan-bundle.tar.gz")
}
