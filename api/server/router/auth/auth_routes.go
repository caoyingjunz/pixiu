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

package auth

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func (a *authRouter) login(c *gin.Context) {
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
	loginResp, err := a.c.Auth().Login(c, &req)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = loginResp

	httputils.SetSuccess(c, r)
}

// logout 无 idMeta：当前用户由中间件写入的上下文取得
func (a *authRouter) logout(c *gin.Context) {
	r := httputils.NewResponse()

	if err := a.c.Auth().Logout(c); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// refresh 认证由中间件保证，刷新逻辑暂未实现
func (a *authRouter) refresh(c *gin.Context) {
	r := httputils.NewResponse()

	if err := a.c.Auth().Refresh(c); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (a *authRouter) sendVerificationCode(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		req types.SendRegistrationCodeRequest
		err error
	)
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	// 发码为未认证公开接口，审计 Operator 默认记为 unknown；此处将目标邮箱作为操作者留痕，
	// 便于审计中按邮箱检索发码记录、排查邮件轰炸等滥用行为（审计记录同时含来源 IP）。
	httputils.SetAuditOperator(c, req.Email)
	r.Result, err = a.c.Auth().SendVerificationCode(c, &req, c.ClientIP())
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (a *authRouter) registerUser(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		req types.RegisterUserRequest
		err error
	)
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetAuditOperator(c, req.Name)
	if err = a.c.Auth().Register(c, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (a *authRouter) listOAuthProviders(c *gin.Context) {
	r := httputils.NewResponse()

	enabledOnly := c.Query("enabled_only") == "true"
	var err error
	if r.Result, err = a.c.Auth().ListOAuthProviders(c, enabledOnly); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (a *authRouter) getOAuthProviderConfig(c *gin.Context) {
	r := httputils.NewResponse()

	var err error
	if r.Result, err = a.c.Auth().GetOAuthProviderConfig(c, c.Param("provider")); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (a *authRouter) updateOAuthProviderConfig(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		req types.UpdateOAuthProviderConfigRequest
		err error
	)
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = a.c.Auth().UpdateOAuthProviderConfig(c, c.Param("provider"), &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (a *authRouter) getOAuthProviderLoginURL(c *gin.Context) {
	r := httputils.NewResponse()

	var err error
	if r.Result, err = a.c.Auth().GetOAuthProviderLoginURL(c, c.Param("provider")); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (a *authRouter) loginWithOAuthProvider(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		req types.OAuthLoginRequest
		err error
	)
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	loginResp, err := a.c.Auth().LoginWithOAuthProvider(c, c.Param("provider"), &req)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = loginResp
	httputils.SetSuccess(c, r)
}
