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

package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/gopixiu/api/server/httpstatus"
	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

// Authorization 使用 rbac 授权策略
// TODO: 后续优化
func Authorization() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if AlwaysAllowPath.Has(path) {
			return
		}

		// 用户 ID
		uid, exist := c.Get("userId")
		r := httputils.NewResponse()
		if !exist {
			httputils.SetFailedWithAbortCode(c, r, http.StatusUnauthorized, httpstatus.NoPermission)
			return
		}

		enforcer := pixiu.CoreV1.Policy().GetEnforce()
		if enforcer == nil {
			httputils.SetFailedWithAbortCode(c, r, http.StatusInternalServerError, httpstatus.InnerError)
			return
		}
		uidStr := strconv.FormatInt(uid.(int64), 10)
		ok, err := enforcer.Enforce(uidStr, path, c.Request.Method)
		if err != nil {
			httputils.SetFailedWithAbortCode(c, r, http.StatusInternalServerError, httpstatus.InnerError)
			return
		}
		if !ok {
			httputils.SetFailedWithAbortCode(c, r, http.StatusUnauthorized, httpstatus.NoPermission)
			return
		}
	}
}
