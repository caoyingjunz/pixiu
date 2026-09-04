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
	r.Result, err = a.c.Auth().SendCode(c, &req, c.ClientIP())
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (a *authRouter) registerUser(c *gin.Context) {
	r := httputils.NewResponse()
	var req types.RegisterUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetAuditOperator(c, req.Name)
	if err := a.c.Auth().Register(c, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}
