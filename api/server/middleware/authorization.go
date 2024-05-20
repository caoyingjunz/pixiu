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
	// 移除 debug 模式，将在下版本移除代码
	_ = o.ComponentConfig.Default.Mode

	u := o.Controller.User()

	return func(c *gin.Context) {
		// 此处已移除 debug 模式的判断
		if alwaysAllowPath.Has(c.Request.URL.Path) || initAdminUser(c) {
			return
		}

		uid, ok := c.Get("userId")
		if !ok {
			httputils.AbortFailedWithCode(c, http.StatusMethodNotAllowed,
				fmt.Errorf("failed to get uid"))
			return
		}
		userId, ok := uid.(int64)
		if !ok {
			httputils.AbortFailedWithCode(c, http.StatusMethodNotAllowed,
				fmt.Errorf("failed to assert uid"))
			return
		}

		status, err := u.GetStatus(c, userId)
		if err != nil {
			httputils.AbortFailedWithCode(c, http.StatusMethodNotAllowed, err)
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
