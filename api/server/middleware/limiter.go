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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/juju/ratelimit"
	"golang.org/x/time/rate"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
	"github.com/caoyingjunz/pixiu/pkg/util/lru"
)

const (
	capacity = 100
	quantum  = 20
	// 提高 IP 桶缓存容量，降低轮换 IP 导致的频繁淘汰
	ipLimiterCacheCap = 4096
)

// UserRateLimiter 针对每个客户端 IP 的请求进行限速。
// 每次请求（含首次）都扣减令牌；登录接口由 LoginRateLimiter 单独治理。
func UserRateLimiter() gin.HandlerFunc {
	cache := lru.NewLRUCache(ipLimiterCacheCap)

	return func(c *gin.Context) {
		if c.Request.Method == http.MethodPost && c.Request.URL.Path == loginPath {
			return
		}
		clientIP := c.ClientIP()
		bucket := cache.GetOrAdd(clientIP, func() interface{} {
			return ratelimit.NewBucketWithQuantum(time.Second, capacity, quantum)
		}).(*ratelimit.Bucket)

		if bucket.TakeAvailable(1) == 0 {
			httputils.AbortFailedWithCode(c, http.StatusForbidden, errors.ErrBusySystem)
		}
	}
}

func Limiter() gin.HandlerFunc {
	// 初始化一个限速器，每秒产生 1000 个令牌，桶的大小为 1000 个
	// TODO: 限速的值从配置或者环境变量中获取
	limiter := rate.NewLimiter(1000, 1000)

	return func(c *gin.Context) {
		if c.Request.Method == http.MethodPost && c.Request.URL.Path == loginPath {
			return
		}
		if !limiter.Allow() {
			httputils.AbortFailedWithCode(c, http.StatusForbidden, errors.ErrBusySystem)
		}
	}
}
