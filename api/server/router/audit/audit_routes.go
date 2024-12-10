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

package audit

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type AuditMeta struct {
	AuditId int64 `uri:"auditId" binding:"required"`
}

func (a *auditRouter) getAudit(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt AuditMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = a.c.Audit().Get(c, opt.AuditId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (a *auditRouter) listAudits(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		listOption types.ListOptions // 分页设置
		err        error
	)
	if err = httputils.ShouldBindAny(c, nil, nil, &listOption); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = a.c.Audit().List(c, listOption); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
