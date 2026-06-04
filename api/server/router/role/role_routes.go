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

package role

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type RoleMeta struct {
	RoleId int64 `uri:"roleId" binding:"required"`
}

func (r *roleRouter) createRole(c *gin.Context) {
	resp := httputils.NewResponse()

	var req types.CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Role().Create(c, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	httputils.SetSuccess(c, resp)
}

func (r *roleRouter) updateRole(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		opt RoleMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	var req types.UpdateRoleRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err = r.c.Role().Update(c, opt.RoleId, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	httputils.SetSuccess(c, resp)
}

func (r *roleRouter) deleteRole(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		opt RoleMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err = r.c.Role().Delete(c, opt.RoleId); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	httputils.SetSuccess(c, resp)
}

func (r *roleRouter) getRole(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		opt RoleMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if resp.Result, err = r.c.Role().Get(c, opt.RoleId); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	httputils.SetSuccess(c, resp)
}

func (r *roleRouter) listRoles(c *gin.Context) {
	resp := httputils.NewResponse()

	var req types.ListRoleRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	var err error
	if resp.Result, err = r.c.Role().List(c, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	httputils.SetSuccess(c, resp)
}

func (r *roleRouter) getRoleAPIs(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		opt RoleMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if resp.Result, err = r.c.Role().GetAPIs(c, opt.RoleId); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	httputils.SetSuccess(c, resp)
}

func (r *roleRouter) updateRoleAPIs(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		opt RoleMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	var req types.UpdateRoleAPIsRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err = r.c.Role().UpdateAPIs(c, opt.RoleId, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	httputils.SetSuccess(c, resp)
}

