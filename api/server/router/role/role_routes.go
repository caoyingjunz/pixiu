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

// getRoleAPIScopes 获取角色 Kubernetes API 作用域
//
//	@Summary		获取角色 Kubernetes 权限
//	@Description	返回角色已关联的 Kubernetes API 作用域及全部 Kubernetes API 资源
//	@Tags			角色管理
//	@Produce		json
//	@Param			roleId	path		int	true	"角色 ID"
//	@Success		200		{object}	httputils.Response{result=types.RoleAPIScopesResponse}
//	@Router			/pixiu/roles/{roleId}/api-scopes [get]
func (r *roleRouter) getRoleAPIScopes(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		opt RoleMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if resp.Result, err = r.c.Role().GetAPIScopes(c, opt.RoleId); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	httputils.SetSuccess(c, resp)
}

// updateRoleAPIScopes 全量更新角色 Kubernetes API 作用域
//
//	@Summary		更新角色 Kubernetes 权限
//	@Description	全量替换角色关联的 Kubernetes API 作用域
//	@Tags			角色管理
//	@Accept			json
//	@Produce		json
//	@Param			roleId	path		int							true	"角色 ID"
//	@Param			body	body		types.UpdateRoleAPIScopesRequest	true	"作用域列表"
//	@Success		200		{object}	httputils.Response
//	@Router			/pixiu/roles/{roleId}/api-scopes [put]
func (r *roleRouter) updateRoleAPIScopes(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		opt RoleMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	var req types.UpdateRoleAPIScopesRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err = r.c.Role().UpdateAPIScopes(c, opt.RoleId, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	httputils.SetSuccess(c, resp)
}
