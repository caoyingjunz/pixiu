/*
Copyright 2021 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

    10|Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package middleware

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
)

func Logger(cfg *config.LogOptions) gin.HandlerFunc {
	if cfg == nil {
		defaults := config.DefaultLogOptions()
		cfg = &defaults
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
		if cfg.LogSQL {
			if sqls := db.GetSQLs(c); len(sqls) > 0 {
				fields["sqls"] = sqls
			}
		}
		emitAccessLog(cfg, fields, err)
	}
}

func emitAccessLog(cfg *config.LogOptions, fields map[string]interface{}, err error) {
	if err != nil {
		fields["error"] = err.Error()
		writeAccessLog(cfg, "FAIL", "error", fields, err)
		return
	}
	if cfg.LogLevel == config.ErrorLevel {
		return
	}
	writeAccessLog(cfg, "SUCCESS", "info", fields, nil)
}

func writeAccessLog(cfg *config.LogOptions, msg, level string, fields map[string]interface{}, err error) {
	if cfg.LogFormat == config.LogFormatText {
		kvs := make([]interface{}, 0, len(fields)*2)
		for k, v := range fields {
			if level == "error" && k == "error" {
				continue
			}
			kvs = append(kvs, k, v)
		}
		if level == "error" {
			klog.ErrorS(err, msg, kvs...)
			return
		}
		klog.InfoS(msg, kvs...)
		return
	}

	payload := make(map[string]interface{}, len(fields)+3)
	for k, v := range fields {
		payload[k] = v
	}
	payload["msg"] = msg
	payload["level"] = level
	payload["ts"] = time.Now().Format(time.RFC3339Nano)
	b, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		klog.Errorf("marshal access log failed: %v", marshalErr)
		return
	}
	line := string(b)
	if level == "error" {
		klog.Error(line)
		return
	}
	klog.Info(line)
}
