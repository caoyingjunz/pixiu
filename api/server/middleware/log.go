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
	"time"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	klog "github.com/sirupsen/logrus"

	"github.com/caoyingjunz/pixiu/pkg/db"
)

const (
	SuccessMsg = "SUCCESS"
	ErrorMsg   = "ERROR"
	FailMsg    = "FAIL"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		c.Set(db.SQLContextKey, new(db.SQLs)) // set SQL context key

		// 处理请求操作
		c.Next()

		fields := klog.Fields{
			"request_id":  requestid.Get(c),
			"method":      c.Request.Method,
			"uri":         c.Request.RequestURI,
			"status_code": c.Writer.Status(),
			"latency":     fmt.Sprintf("%dµs", time.Since(startTime).Microseconds()),
			"client_ip":   c.ClientIP(),
		}
		if sqls := db.GetSQLs(c); len(sqls) > 0 {
			fields["sqls"] = sqls
		}

		if errs := c.Errors; len(errs) > 0 {
			fields["raw_error"] = errs.Errors()
			klog.WithFields(fields).Error(FailMsg)
			return
		}

		klog.WithFields(fields).Info(SuccessMsg)
	}
}
