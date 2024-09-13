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

package auth

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type IdMeta struct {
	PolicyId int64 `uri:"policyId" binding:"required"`
}

func (a *authRouter) listPolicies(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		req types.ListRBACPolicyRequest
		err error
	)
	if err = c.ShouldBindQuery(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = a.c.Auth().ListRBACPolicies(c, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (a *authRouter) createPolicy(c *gin.Context) {
	r := httputils.NewResponse()
	var req types.RBACPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := a.c.Auth().CreateRBACPolicy(c, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (a *authRouter) deletePolicy(c *gin.Context) {
	r := httputils.NewResponse()
	var req types.RBACPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := a.c.Auth().DeleteRBACPolicy(c, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (a *authRouter) listBindings(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		req types.ListGroupBindingRequest
		err error
	)
	if err = c.ShouldBindQuery(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = a.c.Auth().ListGroupBindings(c, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (a *authRouter) createBinding(c *gin.Context) {
	r := httputils.NewResponse()
	var req types.GroupBindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := a.c.Auth().CreateGroupBinding(c, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (a *authRouter) deleteBinding(c *gin.Context) {
	r := httputils.NewResponse()
	var req types.GroupBindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := a.c.Auth().DeleteGroupBinding(c, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
