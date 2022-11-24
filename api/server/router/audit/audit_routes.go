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
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

// TODO: 根据发生事件获取审计事件列表
func (audit *auditRouter) listAuditEvents(c *gin.Context) {
	r := httputils.NewResponse()
	var err error
	if r.Result, err = pixiu.CoreV1.Audit().List(c, "1h"); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
