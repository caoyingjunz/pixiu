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
	"os"

	"net/http"
	"strings"
	"time"

	"github.com/bluele/gcache"
	"github.com/gin-gonic/gin"
	"github.com/juju/ratelimit"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/pkg/log"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

func InitMiddlewares(ginEngine *gin.Engine) {
	ginEngine.Use(LoggerToFile(), RateLimiter(100, 20), AuthN)
}

func LoggerToFile() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// 处理请求操作
		c.Next()

		endTime := time.Now()

		latencyTime := endTime.Sub(startTime)

		reqMethod := c.Request.Method
		reqUri := c.Request.RequestURI
		statusCode := c.Writer.Status()
		clientIp := c.ClientIP()

		log.AccessLog.Infof("| %3d | %13v | %15s | %s | %s |", statusCode, latencyTime, clientIp, reqMethod, reqUri)
	}
}

// 该 RateLimiter 使用了两个开源组件
//  1. google 实现的 LRU Cache 的库
//  2. token bucket 的一个开源实现
//
// 使用 LRU 的原因: 如果不使用LRU, 当用户很多的时候, 需要在内存中维护一个很大的结构, 可能造成OOM
func RateLimiter(capacity int64, quantum int64) gin.HandlerFunc {
	// 初始化一个 LRU Cache
	cache := gcache.New(200).LRU().Build()

	return func(c *gin.Context) {
		r := httputils.NewResponse()
		// 把 key: clientIP value: *ratelimit.Bucket 存入 LRU Cache 中
		clientIP := c.ClientIP()
		if !cache.Has(clientIP) {
			if err := cache.SetWithExpire(clientIP, ratelimit.NewBucketWithQuantum(time.Second, capacity, quantum), time.Minute*5); err != nil {
				httputils.SetFailed(c, r, err)
				c.Abort()
				return
			}
		}

		// 通过 ClientIP 取出 bucket
		val, err := cache.Get(clientIP)
		if err != nil {
			httputils.SetFailed(c, r, err)
			c.Abort()
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

func AuthN(c *gin.Context) {
	if os.Getenv("DEBUG") == "true" {
		return
	}

	// Authentication 身份认证
	if c.Request.URL.Path == "/users/login" {
		return
	}

	r := httputils.NewResponse()
	token := c.GetHeader("Authorization")
	if len(token) == 0 {
		r.SetCode(http.StatusUnauthorized)
		httputils.SetFailed(c, r, fmt.Errorf("authorization header is not provided"))
		c.Abort()
		return
	}

	fields := strings.Fields(token)
	if len(fields) != 2 {
		r.SetCode(http.StatusUnauthorized)
		httputils.SetFailed(c, r, fmt.Errorf("invalid authorization header format"))
		c.Abort()
		return
	}

	if fields[0] != "Bearer" {
		r.SetCode(http.StatusUnauthorized)
		httputils.SetFailed(c, r, fmt.Errorf("unsupported authorization type"))
		c.Abort()
		return
	}

	accessToken := fields[1]
	jwtKey := pixiu.CoreV1.User().GetJWTKey()
	claims, err := httputils.ParseToken(accessToken, []byte(jwtKey))
	if err != nil {
		r.SetCode(http.StatusUnauthorized)
		httputils.SetFailed(c, r, err)
		c.Abort()
		return
	}

	c.Set("userId", claims.Id)
}
