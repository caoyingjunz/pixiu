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
	verificationCodePath   = "/pixiu/auth/verification-codes"
	forgotPasswordCodePath = "/pixiu/auth/forgot-password/verification-codes"
	registrationPath       = "/pixiu/auth/register"
	resetPasswordPath      = "/pixiu/auth/reset-password"
	registrationIPCap      = 8192
)

var (
	registrationCodeGlobal = rate.NewLimiter(5, 10)
	registrationGlobal     = rate.NewLimiter(20, 40)
	registrationIPLimits   = lru.NewLRUCache(registrationIPCap)
)

// RegistrationRateLimiter 单独限制公开注册/发码/忘记密码接口，防止邮件轰炸和批量猜码。
// 发码（注册/忘记密码）：全局 5/s + 每 IP 10/h；注册/重置：全局 20/s + 每 IP 30/min。
// 同类接口按场景使用独立 IP 配额（keyPrefix 区分），互不挤占。
func RegistrationRateLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			return
		}
		path := c.Request.URL.Path

		global := registrationGlobal
		ipRate := rate.Limit(30.0 / 60.0)
		burst := 10
		keyPrefix := "register:"
		switch path {
		case verificationCodePath, forgotPasswordCodePath:
			global = registrationCodeGlobal
			ipRate = rate.Limit(10.0 / 3600.0)
			burst = 3
			keyPrefix = "registration-code:"
			if path == forgotPasswordCodePath {
				keyPrefix = "forgot-password-code:"
			}
		case resetPasswordPath:
			keyPrefix = "reset-password:"
		case registrationPath:
			// 保持默认分支（register:）
		default:
			return
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
