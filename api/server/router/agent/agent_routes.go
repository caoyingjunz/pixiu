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
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
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
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = c.ShouldBindJSON(&req); err != nil {
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
		listOption types.AgentListOptions
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
