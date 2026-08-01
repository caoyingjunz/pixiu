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

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/accesslog"
	"github.com/caoyingjunz/pixiu/pkg/db"
)

func Logger(cfg *config.LogOptions) gin.HandlerFunc {
	opts := accesslog.DefaultOptions()
	if cfg != nil {
		opts = cfg.AccessOptions()
	}
	return func(c *gin.Context) {
		start := time.Now()
		c.Set(db.SQLContextKey, new(db.SQLs))

		c.Next()

		err := httputils.GetRawError(c)
		fields := map[string]interface{}{
			"request_id":              requestid.Get(c),
			"method":                  c.Request.Method,
			"uri":                     c.Request.RequestURI,
			httputils.ResponseCodeKey: httputils.GetResponseCode(c),
			"client_ip":               c.ClientIP(),
			"latency":                 fmt.Sprintf("%dµs", time.Since(start).Microseconds()),
		}
		if opts.SQL {
			if sqls := db.GetSQLs(c); len(sqls) > 0 {
				fields["sqls"] = sqls
			}
		}
		if err != nil {
			fields["error"] = err.Error()
			accesslog.Emit(opts, "FAIL", "error", fields, err)
			return
		}
		// HTTP 成功请求均为 info 档；log.level=debug 时与 info 行为一致（仍输出）。
		if !opts.AllowSuccess(accesslog.TierInfo) {
			return
		}
		accesslog.Emit(opts, "SUCCESS", "info", fields, nil)
	}
}
