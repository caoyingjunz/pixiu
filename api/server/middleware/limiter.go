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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/juju/ratelimit"
	"golang.org/x/time/rate"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/pkg/util/lru"
)

const (
	capacity = 100
	quantum  = 20
	cap      = 200
)

// UserRateLimiter 针对每个用户的请求进行限速
// TODO 限速大小从配置中读取
func UserRateLimiter() gin.HandlerFunc {
	// 初始化一个 LRU Cache
	cache, _ := lru.NewLRUCache(cap)

	return func(c *gin.Context) {
		r := httputils.NewResponse()
		// 把 key: clientIP value: *ratelimit.Bucket 存入 LRU Cache 中
		clientIP := c.ClientIP()
		if !cache.Contains(clientIP) {
			cache.Add(clientIP, ratelimit.NewBucketWithQuantum(time.Second, capacity, quantum))
			return
		}
		// 通过 ClientIP 取出 bucket
		val := cache.Get(clientIP)
		if val == nil {
			return
		}

		// 判断是否还有可用的 bucket
		bucket := val.(*ratelimit.Bucket)
		if bucket.TakeAvailable(1) == 0 {
			r.SetCode(http.StatusGatewayTimeout)
			httputils.SetFailed(c, r, fmt.Errorf("the system is busy. please try again later"))
			c.Abort()
			return
		}
	}
}

func Limiter() gin.HandlerFunc {
	// 初始化一个限速器，每秒产生1000个令牌，桶的大小为1000个
	// 初始化状态桶是满的
	limiter := rate.NewLimiter(1000, 1000)

	return func(c *gin.Context) {
		if !limiter.Allow() {
			r := httputils.NewResponse()
			r.SetCode(http.StatusForbidden)
			httputils.SetFailed(c, r, fmt.Errorf("the system is busy. please try again later"))
			c.Abort()
			return
		}
	}
}
