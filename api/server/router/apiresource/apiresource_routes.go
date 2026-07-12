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

package apiresource

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type APIMeta struct {
	APIId int64 `uri:"apiId" binding:"required"`
}

func (a *apiResourceRouter) createAPI(c *gin.Context) {
	resp := httputils.NewResponse()

	var req types.CreateAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := a.c.APIResource().Create(c, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	httputils.SetSuccess(c, resp)
}

func (a *apiResourceRouter) updateAPI(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		opt APIMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	var req types.UpdateAPIRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err = a.c.APIResource().Update(c, opt.APIId, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	httputils.SetSuccess(c, resp)
}

func (a *apiResourceRouter) deleteAPI(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		opt APIMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err = a.c.APIResource().Delete(c, opt.APIId); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	httputils.SetSuccess(c, resp)
}

func (a *apiResourceRouter) getAPI(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		opt APIMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if resp.Result, err = a.c.APIResource().Get(c, opt.APIId); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	httputils.SetSuccess(c, resp)
}

func (a *apiResourceRouter) listAPIs(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		listOption types.ListOptions
		err        error
	)
	if err = httputils.ShouldBindAny(c, nil, nil, &listOption); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if resp.Result, err = a.c.APIResource().List(c, listOption); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	httputils.SetSuccess(c, resp)
}
