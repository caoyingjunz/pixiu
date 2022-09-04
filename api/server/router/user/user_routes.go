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
	"strings"

	"github.com/gin-gonic/gin"

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
}

// TODO
func (u *userRouter) logout(c *gin.Context) {

}

func (u *userRouter) changePassword(c *gin.Context) {
	r := httputils.NewResponse()
	uid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	// 解析修改密码的三个参数
	//   1. 当前密码 2. 新密码 3. 确认新密码
	var pass types.Password
	if err := c.ShouldBindJSON(&pass); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	// 需要通过 token 中的 id 判断当前操作的用户和需要修改密码的用户是否是同一个
	// Authorization 头在中间件 authN 已经进行参数校验, 此处不需要进行校验了
	fileds := strings.Fields(c.GetHeader("Authorization"))
	if err := pixiu.CoreV1.User().ChangePassword(context.TODO(), &pass, uid, fileds[1]); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
