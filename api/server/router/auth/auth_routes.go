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
	var req types.SendRegistrationCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetAuditOperator(c, req.Email)
	result, err := a.c.Registration().SendCode(c, &req, c.ClientIP())
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = result
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
	if err := a.c.Registration().Register(c, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}
