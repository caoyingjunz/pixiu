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
	"context"
	"github.com/caoyingjunz/gopixiu/api/server/common"
	"github.com/gin-gonic/gin"
	"net/http"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
	"github.com/caoyingjunz/gopixiu/pkg/util"
)

func (u *userRouter) createUser(c *gin.Context) {
	r := httputils.NewResponse()
	var user types.User
	if err := c.ShouldBindJSON(&user); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := pixiu.CoreV1.User().Create(context.TODO(), &user); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// 更新用户属性：
// 不允许更改字段:
// 1. 用户名
// 2. 用户密码 —— 通过修改密码API进行修改
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
	if err = pixiu.CoreV1.User().Update(context.TODO(), &user); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (u *userRouter) deleteUser(c *gin.Context) {
	r := httputils.NewResponse()
	uid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = pixiu.CoreV1.User().Delete(context.TODO(), uid); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (u *userRouter) getUser(c *gin.Context) {
	r := httputils.NewResponse()
	uid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.User().Get(context.TODO(), uid)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (u *userRouter) listUsers(c *gin.Context) {
	r := httputils.NewResponse()
	var err error
	if r.Result, err = pixiu.CoreV1.User().List(context.TODO()); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (u *userRouter) getRoleIDsByUser(c *gin.Context) {
	uid, isExit := c.Get("userId")
	r := httputils.NewResponse()
	if !isExit {
		r.SetCode(common.ErrorCodePermissionDeny)
		httputils.SetFailed(c, r, "无权限")
		return
	}
	result, err := pixiu.CoreV1.User().GetRoleIDByUser(c, uid.(int64))
	if err != nil {
		r.SetCode(http.StatusBadRequest)
		httputils.SetFailed(c, r, "获取失败")
	}
	r.Result = result

	httputils.SetSuccess(c, r)
}

func (u *userRouter) setRolesByUserId(c *gin.Context) {

	var roleIds []int64
	roleId := map[string][]int64{
		"role_ids": roleIds,
	}
	r := httputils.NewResponse()
	err := c.ShouldBindJSON(&roleId)
	if err != nil {
		r.SetCode(http.StatusBadRequest)
		httputils.SetFailed(c, r, "参数错误")
		return
	}
	uid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	err = pixiu.CoreV1.User().SetUserRoles(c, uid, roleId["role_ids"])
	if err != nil {
		r.SetCode(http.StatusBadRequest)
		httputils.SetFailed(c, r, "内部错误")
		return
	}
	r.SetCode(http.StatusOK)
	httputils.SetSuccess(c, r)
}

// login
// 1. 检验用户名和密码是否正确，
// 2. 返回 token
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
	if r.Result, err = pixiu.CoreV1.User().Login(context.TODO(), &user); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
	util.GetUUID()
}

func (u *userRouter) getMenus(c *gin.Context) {
	uidStr, _ := c.Get("userId")
	r := httputils.NewResponse()
	uid := uidStr.(int64)
	//if err != nil {
	//	r.SetCode(http.StatusBadRequest)
	//	httputils.SetFailed(c, r, "参数错误")
	//	return
	//}

	res, err := pixiu.CoreV1.User().GetMenus(c, uid)
	if err != nil {
		r.SetCode(http.StatusBadRequest)
		httputils.SetFailed(c, r, "内部错误")
		return
	}
	r.Result = res
	httputils.SetSuccess(c, r)
}

// TODO
func (u *userRouter) logout(c *gin.Context) {

}
