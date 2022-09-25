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
	"github.com/caoyingjunz/gopixiu/pkg/log"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

func Rbac() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/users/login" {
			return
		}
		// 用户ID
		uid, exist := c.Get("userId")
		r := httputils.NewResponse()
		if !exist {
			r.SetCode(http.StatusUnauthorized)
			httputils.SetFailed(c, r, httpstatus.NoPermission)
			return
		}

		method := c.Request.Method
		enforcer := pixiu.CoreV1.Policy().GetEnforce()
		if enforcer == nil {
			log.Logger.Errorf("failed to get enforce.")
			return
		}
		uidStr := strconv.FormatInt(uid.(int64), 10)
		ok, err := enforcer.Enforce(uidStr, path, method)
		if err != nil {
			r.SetCode(http.StatusInternalServerError)
			httputils.SetFailed(c, r, httpstatus.InnerError)
			c.Abort()
			return
		}
		if !ok {
			r.SetCode(http.StatusUnauthorized)
			httputils.SetFailed(c, r, httpstatus.NoPermission)
			c.Abort()
			return
		}
	}
}
