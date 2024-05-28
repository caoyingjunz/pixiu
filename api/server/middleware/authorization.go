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
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
)

// Authorization 鉴权
func Authorization(o *options.Options) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 允许请求直接通过
		if alwaysAllowPath.Has(c.Request.URL.Path) || allowCustomRequest(c) {
			return
		}

		status, err := GetStatus(c, o)
		if err != nil {
			httputils.AbortFailedWithCode(c, http.StatusMethodNotAllowed, err)
			return
		}

		// 禁用用户无法进行任何操作
		if status == 2 {
			httputils.AbortFailedWithCode(c, http.StatusMethodNotAllowed, fmt.Errorf("用户已被禁用"))
			return
		}
		// status 为 1，表示用户只读模式, 只读模式只允许查询请求
		if status == 1 {
			if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodOptions {
				httputils.AbortFailedWithCode(c, http.StatusMethodNotAllowed, fmt.Errorf("无操作权限"))
				return
			}
		}
	}
}

func GetStatus(c *gin.Context, o *options.Options) (int, error) {
	// 从请求中获取用户ID
	uid, ok := c.Get("userId")
	if !ok {
		return 0, fmt.Errorf("failed to get uid")
	}

	// 转换为 int64 的用户ID
	userId, ok := uid.(int64)
	if !ok {
		return 0, fmt.Errorf("failed to assert uid")
	}
	return o.Controller.User().GetStatus(c, userId)
}
