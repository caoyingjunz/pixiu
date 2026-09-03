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

package user

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type IdMeta struct {
	UserId int64 `uri:"userId" binding:"required"`
}

func (u *userRouter) createUser(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		req types.CreateUserRequest
		err error
	)
	if err = httputils.BindCreateRequest(c, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = u.c.User().Create(c, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (u *userRouter) updateUser(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		idMeta IdMeta
		req    types.UpdateUserRequest
		err    error
	)
	if err = httputils.ShouldBindAny(c, &req, &idMeta, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = u.c.User().Update(c, idMeta.UserId, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (u *userRouter) updatePassword(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		idMeta IdMeta
		req    types.UpdateUserPasswordRequest
		err    error
	)
	if err = httputils.ShouldBindAny(c, &req, &idMeta, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = u.c.User().UpdatePassword(c, idMeta.UserId, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (u *userRouter) deleteUser(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		idMeta IdMeta
		err    error
	)
	if err = c.ShouldBindUri(&idMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = u.c.User().Delete(c, idMeta.UserId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (u *userRouter) getUser(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		idMeta IdMeta
		err    error
	)
	if err = c.ShouldBindUri(&idMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = u.c.User().Get(c, idMeta.UserId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (u *userRouter) listUsers(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		listOption types.ListOptions
		err        error
	)
	if err = httputils.BindListOptionsWithUser(c, &listOption); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = u.c.User().List(c, listOption); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// login 用户登录。
// TODO: 注册/验证码已迁至 POST /pixiu/auth/*；login 可择机迁至 POST /pixiu/auth/login，
// 与 auth 路由组统一。迁移时需保留 POST /pixiu/users/login 作为兼容 alias，并同步
// pixiu-ui、dashboard、nginx 限流及 security/baseline 等引用。
func (u *userRouter) login(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		req types.LoginRequest
		err error
	)
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetAuditOperator(c, req.Name)
	loginResp, err := u.c.User().Login(c, &req)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = loginResp

	httputils.SetSuccess(c, r)
}

func (u *userRouter) logout(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		idMeta IdMeta
		err    error
	)
	if err = httputils.ShouldBindAny(c, nil, &idMeta, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = u.c.User().Logout(c, idMeta.UserId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (u *userRouter) getCurrentUserPermissions(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err error
	)
	if r.Result, err = u.c.User().GetCurrentUserPermissions(c); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}
