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

// CreateUser godoc
//
//	@Summary      Create a user
//	@Description  Create by a json user
//	@Tags         Users
//	@Accept       json
//	@Produce      json
//	@Param        user  body      types.CreateUserRequest  true  "Create user"
//	@Success      200   {object}  httputils.Response
//	@Failure      400   {object}  httputils.Response
//	@Failure      404   {object}  httputils.Response
//	@Failure      500   {object}  httputils.Response
//	@Router       /pixiu/users/ [post]
//	              @Security  Bearer
func (u *userRouter) createUser(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		req types.CreateUserRequest
		err error
	)
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = u.c.User().Create(c, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// UpdateUser godoc
//
//	@Summary      Update an user
//	@Description  Update by json user
//	@Tags         Users
//	@Accept       json
//	@Produce      json
//	@Param        userId  path      int                      true  "User ID"
//	@Param        user    body      types.UpdateUserRequest  true  "Update user"
//	@Success      200     {object}  httputils.Response
//	@Failure      400     {object}  httputils.Response
//	@Failure      404     {object}  httputils.Response
//	@Failure      500     {object}  httputils.Response
//	@Router       /pixiu/users/{userId} [put]
//	              @Security  Bearer
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

// UpdateUserPassword godoc
//
//	@Summary      Update user password
//	@Description  Update by json user
//	@Tags         Users
//	@Accept       json
//	@Produce      json
//	@Param        userId  path      int                              true  "User ID"
//	@Param        user    body      types.UpdateUserPasswordRequest  true  "Update user password"
//	@Success      200     {object}  httputils.Response
//	@Failure      400     {object}  httputils.Response
//	@Failure      404     {object}  httputils.Response
//	@Failure      500     {object}  httputils.Response
//	@Router       /pixiu/users/password [put]
//	              @Security  Bearer
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

// DeleteUser godoc
//
//	@Summary      Delete user by userId
//	@Description  Delete by userID
//	@Tags         Users
//	@Accept       json
//	@Produce      json
//	@Param        userId  path      int  true  "User ID"
//	@Success      200     {object}  httputils.Response
//	@Failure      400     {object}  httputils.Response
//	@Failure      404     {object}  httputils.Response
//	@Failure      500     {object}  httputils.Response
//	@Router       /pixiu/users/{userId} [delete]
//	              @Security  Bearer
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

// Getuser godoc
//
//	@Summary      Get user by userId
//	@Description  Get by user ID
//	@Tags         Users
//	@Accept       json
//	@Produce      json
//	@Param        userId  path      int  true  "User ID"
//	@Success      200     {object}  httputils.Response{result=types.User}
//	@Failure      400     {object}  httputils.Response
//	@Failure      404     {object}  httputils.Response
//	@Failure      500     {object}  httputils.Response
//	@Router       /pixiu/users/{userId} [get]
//	              @Security  Bearer
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

// Listusers godoc
//
//	@Summary      List users
//	@Description  List users
//	@Tags         Users
//	@Accept       json
//	@Produce      json
//	@Success      200  {array}   httputils.Response{result=[]types.User}
//	@Failure      400  {object}  httputils.Response
//	@Failure      404  {object}  httputils.Response
//	@Failure      500  {object}  httputils.Response
//	@Router       /pixiu/users [get]
//	              @Security  Bearer
func (u *userRouter) listUsers(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		opts types.ListOptions
		err  error
	)
	if err = httputils.ShouldBindAny(c, nil, nil, &opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if opts.Count {
		r.Result, err = u.c.User().GetCount(c, opts)
	} else {
		r.Result, err = u.c.User().List(c, opts)
	}
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// Login godoc
//
//	@Summary      User login
//	@Description  Login by a json user
//	@Tags         Login
//	@Accept       json
//	@Produce      json
//	@Param        user  body      types.LoginRequest  true  "User login"
//	@Success      200   {object}  httputils.Response
//	@Failure      400   {object}  httputils.Response
//	@Failure      404   {object}  httputils.Response
//	@Failure      500   {object}  httputils.Response
//	@Router       /pixiu/users/login [post]
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
	loginResp, err := u.c.User().Login(c, &req)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = loginResp
	httputils.SetUserToContext(c, loginResp.User)

	httputils.SetSuccess(c, r)
}

// TODO
func (u *userRouter) logout(c *gin.Context) {
	r := httputils.NewResponse()

	httputils.SetSuccess(c, r)
}
