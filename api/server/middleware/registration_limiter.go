/*
Copyright 2026 The Pixiu Authors.

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
	"golang.org/x/time/rate"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/util/lru"
)

const (
	registrationCodePath = "/pixiu/users/registration-codes"
	registrationPath     = "/pixiu/users/register"
	registrationIPCap    = 8192
)

var (
	registrationCodeGlobal = rate.NewLimiter(5, 10)
	registrationGlobal     = rate.NewLimiter(20, 40)
	registrationIPLimits   = lru.NewLRUCache(registrationIPCap)
)

// RegistrationRateLimiter 单独限制公开注册接口，防止邮件轰炸和批量猜码。
func RegistrationRateLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			return
		}
		path := c.Request.URL.Path
		if path != registrationCodePath && path != registrationPath {
			return
		}

		global := registrationGlobal
		ipRate := rate.Limit(30.0 / 60.0)
		burst := 10
		keyPrefix := "register:"
		if path == registrationCodePath {
			global = registrationCodeGlobal
			ipRate = rate.Limit(10.0 / 3600.0)
			burst = 3
			keyPrefix = "registration-code:"
		}
		if !global.Allow() {
			httputils.AbortFailedWithCode(c, http.StatusTooManyRequests, errors.ErrTooManyRegistrationAttempts)
			return
		}
		ip := c.ClientIP()
		if ip == "" {
			ip = "unknown"
		}
		limiter := registrationIPLimits.GetOrAdd(keyPrefix+ip, func() interface{} {
			return rate.NewLimiter(ipRate, burst)
		}).(*rate.Limiter)
		if !limiter.Allow() {
			httputils.AbortFailedWithCode(c, http.StatusTooManyRequests, errors.ErrTooManyRegistrationAttempts)
		}
	}
}
