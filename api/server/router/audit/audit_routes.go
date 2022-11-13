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
	"github.com/caoyingjunz/gopixiu/api/server/httpstatus"
	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
	"github.com/gin-gonic/gin"
	"strconv"
)

func (u *auditRouter) deleteOperationLog(c *gin.Context) {
	r := httputils.NewResponse()
	var idsReq model.IdsReq
	_ = c.ShouldBindJSON(&idsReq)
	if err := pixiu.CoreV1.OperationLog().Delete(c, idsReq.Ids); err != nil {
		httputils.SetFailed(c, r, err)
	}
	httputils.SetSuccess(c, r)
}

// @Summary      List operationLog info
// @Description  List operationLog info
// @Tags         audit
// @Accept       json
// @Produce      json
// @Param        page   query      int  false  "pageSize"
// @Param        limit   query      int  false  "page limit"
// @Success      200  {object}  httputils.Response{result=model.PageUser}
// @Failure      400  {object}  httputils.HttpError
// @Router       /operation_logs [get]
func (u *auditRouter) listOperationLog(c *gin.Context) {
	r := httputils.NewResponse()
	pageStr := c.DefaultQuery("page", "0")
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}

	limitStr := c.DefaultQuery("limit", "0")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	if r.Result, err = pixiu.CoreV1.OperationLog().List(c, page, limit); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}
