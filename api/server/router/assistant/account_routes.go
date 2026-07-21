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

package assistant

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type accountMeta struct {
	AccountId int64 `uri:"accountId" binding:"required"`
}

func (r *router) createAccount(c *gin.Context) {
	resp := httputils.NewResponse()
	var req types.CreateAIAccountRequest
	if err := httputils.ShouldBindAny(c, &req, nil, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Assistant().Account().Create(c, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) updateAccount(c *gin.Context) {
	resp := httputils.NewResponse()
	var meta accountMeta
	var req types.UpdateAIAccountRequest
	if err := httputils.ShouldBindAny(c, &req, &meta, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	req.Id = meta.AccountId
	if err := r.c.Assistant().Account().Update(c, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) deleteAccount(c *gin.Context) {
	resp := httputils.NewResponse()
	var meta accountMeta
	if err := httputils.ShouldBindAny(c, nil, &meta, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Assistant().Account().Delete(c, meta.AccountId); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) getAccount(c *gin.Context) {
	resp := httputils.NewResponse()
	var meta accountMeta
	var err error
	if err = httputils.ShouldBindAny(c, nil, &meta, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if resp.Result, err = r.c.Assistant().Account().Get(c, meta.AccountId); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) listAccounts(c *gin.Context) {
	resp := httputils.NewResponse()
	var opts types.ListOptions
	var err error
	if err = httputils.ShouldBindAny(c, nil, nil, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if resp.Result, err = r.c.Assistant().Account().List(c, opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}
