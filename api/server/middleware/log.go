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
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/db"
	logutil "github.com/caoyingjunz/pixiu/pkg/util/log"
)

func Logger(cfg *logutil.LogOptions) gin.HandlerFunc {
	return func(c *gin.Context) {
		l := logutil.NewLogger(cfg)
		c.Set(db.SQLContextKey, new(db.SQLs)) // set SQL context key

		// 处理请求操作
		c.Next()

		l.WithLogFields(map[string]interface{}{
			"request_id":              requestid.Get(c),
			"method":                  c.Request.Method,
			"uri":                     c.Request.RequestURI,
			httputils.ResponseCodeKey: httputils.GetResponseCode(c),
			"client_ip":               c.ClientIP(),
		})
		l.Log(c, logutil.InfoLevel, httputils.GetRawError(c))
	}
}
