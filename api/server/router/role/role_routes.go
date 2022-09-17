/*
Copyright 2021 The Pixiu Authors.

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
	"context"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/gopixiu/api/server/httpstatus"
	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
	"github.com/caoyingjunz/gopixiu/pkg/util"
)

func (o *roleRouter) addRole(c *gin.Context) {
	r := httputils.NewResponse()
	var role model.Role // TODO 后续优化
	if err := c.ShouldBindJSON(&role); err != nil {
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	if _, err := pixiu.CoreV1.Role().Create(context.TODO(), &role); err != nil {
		httputils.SetFailed(c, r, httpstatus.OperateFailed)
		return
	}
	httputils.SetSuccess(c, r)
}

func (o *roleRouter) updateRole(c *gin.Context) {
	r := httputils.NewResponse()
	var role model.Role // TODO 后续优化
	if err := c.ShouldBindJSON(&role); err != nil {
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	roleId, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	if err = pixiu.CoreV1.Role().Update(context.TODO(), &role, roleId); err != nil {
		httputils.SetFailed(c, r, httpstatus.OperateFailed)
		return
	}

	httputils.SetSuccess(c, r)
}

// 删除前弹窗提示检查该角色是否已经与用户绑定，如果绑定，删除后用户将没有此角色权限
func (o *roleRouter) deleteRole(c *gin.Context) {
	r := httputils.NewResponse()
	rid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	if err != nil {
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	if err = pixiu.CoreV1.Role().Delete(c, rid); err != nil {
		httputils.SetFailed(c, r, httpstatus.OperateFailed)
		return
	}

	httputils.SetSuccess(c, r)
}

func (o *roleRouter) getRole(c *gin.Context) {
	r := httputils.NewResponse()
	rid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	r.Result, err = pixiu.CoreV1.Role().Get(context.TODO(), rid)
	if err != nil {
		httputils.SetFailed(c, r, httpstatus.OperateFailed)
		return
	}

	httputils.SetSuccess(c, r)
}

func (o *roleRouter) listRoles(c *gin.Context) {
	r := httputils.NewResponse()
	var err error
	if r.Result, err = pixiu.CoreV1.Role().List(c); err != nil {
		httputils.SetFailed(c, r, httpstatus.OperateFailed)
		return
	}

	httputils.SetSuccess(c, r)
}

func (o *roleRouter) getMenusByRole(c *gin.Context) {
	r := httputils.NewResponse()
	rid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	if r.Result, err = pixiu.CoreV1.Role().GetMenusByRoleID(c, rid); err != nil {
		httputils.SetFailed(c, r, httpstatus.OperateFailed)
		return
	}
	httputils.SetSuccess(c, r)
}

func (o *roleRouter) setRoleMenus(c *gin.Context) {
	r := httputils.NewResponse()
	rid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	var menuIds types.Menus
	if err = c.ShouldBindJSON(&menuIds); err != nil {
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	if err = pixiu.CoreV1.Role().SetRole(c, rid, menuIds.MenuIDS); err != nil {
		httputils.SetFailed(c, r, httpstatus.OperateFailed)
		return
	}

	httputils.SetSuccess(c, r)
}
