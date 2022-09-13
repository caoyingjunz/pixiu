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

	"github.com/gin-gonic/gin"
	"github.com/juju/ratelimit"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/pkg/log"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
	"github.com/caoyingjunz/gopixiu/pkg/util/lru"
)

func InitMiddlewares(ginEngine *gin.Engine) {
	ginEngine.Use(LoggerToFile(), UserRateLimiter(100, 20), AuthN)
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

// Limiter TODO
func Limiter(c *gin.Context) {}

// UserRateLimiter 针对每个用户的请求进行限速
// TODO 限速大小从配置中读取
func UserRateLimiter(capacity int64, quantum int64) gin.HandlerFunc {
	// 初始化一个 LRU Cache
	cache, _ := lru.NewLRUCache(200)

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
