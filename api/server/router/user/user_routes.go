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

	pixiumeta "github.com/caoyingjunz/gopixiu/api/meta"
	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/errors"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
	"github.com/caoyingjunz/gopixiu/pkg/util"
)

// @Summary      Create a user
// @Description  Create a user
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        data body types.User true "user info"
// @Success      200  {object}  httputils.HttpOK
// @Failure      400  {object}  httputils.HttpError
// @Router       /users [post]
func (u *userRouter) createUser(c *gin.Context) {
	r := httputils.NewResponse()
	var user types.User
	if err := c.ShouldBindJSON(&user); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := pixiu.CoreV1.User().Create(c, &user); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// 更新用户属性：
// 不允许更改字段:
// 1. 用户名
// 2. 用户密码 —— 通过修改密码API进行修改
// @Summary      Update  user
// @Description  Update a user
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "user ID"  Format(int64)
// @Param        data body types.User true "user info"
// @Success      200  {object}  httputils.HttpOK
// @Failure      400  {object}  httputils.HttpError
// @Router       /users [put]
func (u *userRouter) updateUser(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		user types.User
	)
	if err = c.ShouldBindJSON(&user); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	user.Id, err = util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = pixiu.CoreV1.User().Update(c, &user); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// @Summary      Delete user by user id
// @Description  Delete user by user id
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "user ID"  Format(int64)
// @Success      200  {object}  httputils.HttpOK
// @Failure      400  {object}  httputils.HttpError
// @Router       /users/{id} [delete]
func (u *userRouter) deleteUser(c *gin.Context) {
	r := httputils.NewResponse()
	uid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = pixiu.CoreV1.User().Delete(c, uid); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// @Summary      Get user info by user id
// @Description  Get user info by user id
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "user ID"  Format(int64)
// @Success      200  {object}  httputils.Response{result=types.User}
// @Failure      400  {object}  httputils.HttpError
// @Router       /users/{id} [get]
func (u *userRouter) getUser(c *gin.Context) {
	r := httputils.NewResponse()
	uid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.User().Get(c, uid)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// @Summary      List user info
// @Description  List user info
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        page   query      int  false  "pageSize"
// @Param        limit   query      int  false  "page limit"
// @Success      200  {object}  httputils.Response{result=model.PageUser}
// @Failure      400  {object}  httputils.HttpError
// @Router       /users [get]
func (u *userRouter) listUsers(c *gin.Context) {
	r := httputils.NewResponse()
	var err error
	if r.Result, err = pixiu.CoreV1.User().List(c, pixiumeta.ParseListSelector(c)); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// login
// 1. 检验用户名和密码是否正确，
// 2. 返回 token
// @Summary      Login
// @Description  Login
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        data body types.User true "user info"
// @Success      200  {object}  httputils.HttpOK
// @Failure      400  {object}  httputils.HttpError
// @Router       /users/login [post]
func (u *userRouter) login(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		user types.User
		err  error
	)
	if err = c.ShouldBindJSON(&user); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = pixiu.CoreV1.User().Login(c, &user); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// TODO
func (u *userRouter) logout(c *gin.Context) {}

// @Summary      reset password by user id
// @Description  reset password by user id
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "user ID"  Format(int64)
// @Success      200  {object}  httputils.Response{result=types.User}
// @Failure      400  {object}  httputils.HttpError
// @Router       /users/{id}/password [put]
func (u *userRouter) resetPassword(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts pixiumeta.IdMeta
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = pixiu.CoreV1.User().ResetPassword(c, opts.Id, c.GetInt64("userId")); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// @Summary      Change user password
// @Description  Change user password
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "user ID"  Format(int64)
// @Param        data body types.Password true "password info"
// @Success      200  {object}  httputils.HttpOK
// @Failure      400  {object}  httputils.HttpError
// @Router       /users/{id} [put]
func (u *userRouter) changePassword(c *gin.Context) {
	r := httputils.NewResponse()
	var opts pixiumeta.IdMeta
	if err := c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	// 解析修改密码的三个参数
	//  1. 当前密码 2. 新密码 3. 确认新密码
	var password types.Password
	if err := c.ShouldBindJSON(&password); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	password.UserId = opts.Id

	// 需要通过 token 中的 id 判断当前操作的用户和需要修改密码的用户是否是同一个
	// Get the uid from token
	if err := pixiu.CoreV1.User().ChangePassword(c, c.GetInt64("userId"), &password); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// @Summary      Get user permission
// @Description  Get user permission
// @Tags         users
// @Accept       json
// @Produce      json
// @Success      200  {object}  httputils.Response{result=[]string}
// @Failure      400  {object}  httputils.HttpError
// @Router       /users/permissions [get]
func (u *userRouter) getButtonsByCurrentUser(c *gin.Context) {
	r := httputils.NewResponse()
	uidStr, exist := c.Get("userId")
	if !exist {
		httputils.SetFailed(c, r, errors.NoUserIdError)
		return
	}
	uid := uidStr.(int64)

	res, err := pixiu.CoreV1.User().GetButtonsByUserID(c, uid)
	if err != nil {
		httputils.SetFailed(c, r, errors.OperateFailed)
		return
	}
	r.Result = res
	httputils.SetSuccess(c, r)
}

// @Summary      Get left menus by current user
// @Description  Get left menus  by current user
// @Tags         users
// @Accept       json
// @Produce      json
// @Success      200  {object}  httputils.Response{result=[]model.Menu}
// @Failure      400  {object}  httputils.HttpError
// @Router       /users/menus [get]
func (u *userRouter) getLeftMenusByCurrentUser(c *gin.Context) {
	uidStr, exist := c.Get("userId")
	r := httputils.NewResponse()
	if !exist {
		httputils.SetFailed(c, r, errors.NoUserIdError)
		return
	}
	var err error
	uid := uidStr.(int64)
	r.Result, err = pixiu.CoreV1.User().GetLeftMenusByUserID(c, uid)
	if err != nil {
		httputils.SetFailed(c, r, errors.OperateFailed)
		return
	}

	httputils.SetSuccess(c, r)
}

// @Summary      Get user roles by user id
// @Description  Get users roles by user id
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "user ID"  Format(int64)
// @Success      200  {object}  httputils.Response{result=model.Role}
// @Failure      400  {object}  httputils.HttpError
// @Router       /users/{id}/roles [get]
func (u *userRouter) getUserRoles(c *gin.Context) {
	r := httputils.NewResponse()
	uid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, errors.ParamsError)
		return
	}
	result, err := pixiu.CoreV1.User().GetRoleIDByUser(c, uid)
	if err != nil {
		httputils.SetFailed(c, r, errors.OperateFailed)
		return
	}
	r.Result = result
	httputils.SetSuccess(c, r)
}

// @Summary      Assign User Roles base on user id
// @Description  Assign User Roles base on user id
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "user ID"  Format(int64)
// @Param        data body types.Roles true "role ids"
// @Success      200  {object}  httputils.HttpOK
// @Failure      400  {object}  httputils.HttpError
// @Router       /users/{id}/roles [post]
func (u *userRouter) setUserRoles(c *gin.Context) {
	var roles types.Roles
	r := httputils.NewResponse()
	err := c.ShouldBindJSON(&roles)
	if err != nil {
		httputils.SetFailed(c, r, errors.ParamsError)
		return
	}

	uid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, errors.ParamsError)
		return
	}

	res, err := pixiu.CoreV1.User().Get(c, uid)
	if err != nil || res == nil {
		httputils.SetFailed(c, r, errors.ParamsError)
		return
	}

	err = pixiu.CoreV1.User().SetUserRoles(c, uid, roles.RoleIds)
	if err != nil {
		httputils.SetFailed(c, r, errors.OperateFailed)
		return
	}
	httputils.SetSuccess(c, r)
}

// @Summary      Update  user status
// @Description  Update  user status
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "user ID"  Format(int64)
// @Param        status   path      int  true  "status"  Format(int64)
// @Success      200  {object}  httputils.HttpOK
// @Failure      400  {object}  httputils.HttpError
// @Router       /users/:id/status/:status [put]
func (u *userRouter) updateUserStatus(c *gin.Context) {
	r := httputils.NewResponse()

	userId, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	status, err := util.ParseInt64(c.Param("status"))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if err = pixiu.CoreV1.User().UpdateStatus(c, userId, status); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
