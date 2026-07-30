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

package agent

import (
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	agentctl "github.com/caoyingjunz/pixiu/pkg/controller/agent"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type AgentMeta struct {
	AgentId int64 `uri:"agentId" binding:"required"`
}

func (a *agentRouter) createAgent(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		req types.CreateAgentRequest
		err error
	)
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = a.c.Agent().Create(c, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (a *agentRouter) updateAgent(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt AgentMeta
		req types.UpdateAgentRequest
		err error
	)
	if err = httputils.ShouldBindAny(c, &req, &opt, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = a.c.Agent().Update(c, opt.AgentId, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (a *agentRouter) deleteAgent(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt AgentMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = a.c.Agent().Delete(c, opt.AgentId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (a *agentRouter) getAgent(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt AgentMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = a.c.Agent().Get(c, opt.AgentId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (a *agentRouter) listAgents(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		listOption types.ListOptions
		err        error
	)
	if err = httputils.ShouldBindAny(c, nil, nil, &listOption); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = a.c.Agent().List(c, listOption); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// Agent token-authenticated task APIs (no JWT).
func getAgentToken(c *gin.Context) string {
	return c.GetHeader(agentctl.TokenHeader)
}

func (a *agentRouter) heartbeat(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		req types.AgentHeartbeatRequest
		err error
	)
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = a.c.Agent().Heartbeat(c, getAgentToken(c), &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (a *agentRouter) claim(c *gin.Context) {
	r := httputils.NewResponse()

	var err error
	r.Result, err = a.c.Agent().Claim(c, getAgentToken(c))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (a *agentRouter) agentLogs(c *gin.Context) {
	r := httputils.NewResponse()
	jobId, err := strconv.ParseInt(c.Param("jobId"), 10, 64)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	var req types.AgentJobLogsRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = a.c.Agent().AppendLogs(c, getAgentToken(c), jobId, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (a *agentRouter) agentResult(c *gin.Context) {
	r := httputils.NewResponse()
	jobId, err := strconv.ParseInt(c.Param("jobId"), 10, 64)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	var req types.AgentJobResultRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = a.c.Agent().ReportResult(c, getAgentToken(c), jobId, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (a *agentRouter) agentBundle(c *gin.Context) {
	jobId, err := strconv.ParseInt(c.Param("jobId"), 10, 64)
	if err != nil {
		httputils.AbortFailedWithCode(c, http.StatusBadRequest, err)
		return
	}
	path, err := a.c.Agent().BundlePath(c, getAgentToken(c), jobId)
	if err != nil {
		httputils.SetFailed(c, httputils.NewResponse(), err)
		return
	}
	defer os.Remove(path)
	c.FileAttachment(path, "plan-bundle.tar.gz")
}
