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

package tenant

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type TenantMeta struct {
	TenantId int64 `uri:"tenantId" binding:"required"`
}

func (t *tenantRouter) createTenant(c *gin.Context) {
	r := httputils.NewResponse()

	var req types.CreateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := t.c.Tenant().Create(c, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (t *tenantRouter) updateTenant(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt TenantMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	var req types.UpdateTenantRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = t.c.Tenant().Update(c, opt.TenantId, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (t *tenantRouter) deleteTenant(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt TenantMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = t.c.Tenant().Delete(c, opt.TenantId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (t *tenantRouter) getTenant(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt TenantMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = t.c.Tenant().Get(c, opt.TenantId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (t *tenantRouter) listTenants(c *gin.Context) {
	r := httputils.NewResponse()

	var err error
	if r.Result, err = t.c.Tenant().List(c); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
