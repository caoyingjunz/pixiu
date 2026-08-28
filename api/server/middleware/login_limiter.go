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

package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/util/loginlimit"
)

const loginPath = "/pixiu/users/login"

// LoginRateLimiter 对登录接口按 IP 严格限流，在进入 handler / bcrypt 之前拦截刷登录。
func LoginRateLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost || c.Request.URL.Path != loginPath {
			return
		}
		if !loginlimit.AllowIP(c.ClientIP()) {
			httputils.AbortFailedWithCode(c, http.StatusTooManyRequests, errors.ErrTooManyLoginAttempts)
		}
	}
}
