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
	"bytes"
	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/log"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
	"github.com/caoyingjunz/gopixiu/pkg/types"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"time"
)

// OperationLog 操作记录
func OperationLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		if types.HealthURL == (c.Request.URL.Path) || types.LoginURL == (c.Request.URL.Path) {
			return
		}
		// cCp := c.Copy()
		var param []byte
		var ip string
		var userId int64
		if c.Request.Method != http.MethodGet {
			var err error
			param, err = io.ReadAll(c.Request.Body)
			if err != nil {
				log.Logger.Errorf("get param from request error: %v", err)
			} else {
				c.Request.Body = io.NopCloser(bytes.NewBuffer(param))
			}
		}
		uId, _ := c.Get(types.UserId)
		if uId == nil {
			userId = 0
		} else {
			userId = uId.(int64)
		}
		ip = c.ClientIP()
		operationLog := model.OperationLog{
			Ip:       ip,
			Location: httputils.GetCityByIp(ip),
			Agent:    httputils.GetUserAgent(c.Request.UserAgent()),
			Path:     c.Request.URL.Path,
			Method:   c.Request.Method,
			Param:    string(param),
			UserID:   userId,
		}
		writer := responseBodyWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
		}
		c.Writer = writer
		now := time.Now()

		c.Next()

		latency := time.Now().Sub(now)
		operationLog.ErrMsg = c.Errors.ByType(gin.ErrorTypePrivate).String()
		operationLog.Status = c.Writer.Status()
		operationLog.Latency = latency
		operationLog.Pesp_result = writer.body.String()

		if err := pixiu.CoreV1.OperationLog().Create(c, &operationLog); err != nil {
			log.Logger.Errorf("save operationLog error: %v", err)
		}
	}
}

type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r responseBodyWriter) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}
