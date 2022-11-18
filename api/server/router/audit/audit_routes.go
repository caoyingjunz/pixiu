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

package audit

import (
	"context"

	pixiumeta "github.com/caoyingjunz/gopixiu/api/meta"
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

func (u *auditRouter) deleteAudit(c *gin.Context) {
	r := httputils.NewResponse()
	var idsReq model.IdsReq
	err := c.ShouldBindJSON(&idsReq)
	if err != nil {
		httputils.SetFailed(c, r, err)
	}
	if err := pixiu.CoreV1.Audit().Delete(c, idsReq.Ids); err != nil {
		httputils.SetFailed(c, r, err)
	}
	httputils.SetSuccess(c, r)
}

func (u *auditRouter) listAudit(c *gin.Context) {
	r := httputils.NewResponse()
	var err error
	if r.Result, err = pixiu.CoreV1.Audit().List(context.TODO(), pixiumeta.ParseListSelector(c)); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
