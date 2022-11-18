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
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	apiTypes "github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/log"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
	"github.com/caoyingjunz/gopixiu/pkg/types"
)

// Audit 操作记录
func Audit() gin.HandlerFunc {
	return func(c *gin.Context) {
		urlPath := c.Request.URL.Path
		if AuditAllowPath.Has(urlPath) {
			return
		}
		var param []byte
		var ip string
		var userId int64
		if c.Request.Method != http.MethodGet && urlPath != types.LoginURL {
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
			userId = types.CanNotFindUserId
		} else {
			userId = uId.(int64)
		}
		// 登录请求, 查询用户id
		if urlPath == types.LoginURL {
			var (
				user apiTypes.User
				err  error
			)
			if err = c.ShouldBindBodyWith(&user, binding.JSON); err != nil {
				userId = types.CanNotFindUserId
			}
			userInfo, err := pixiu.CoreV1.User().GetUserIdByName(c, user.Name)
			if err != nil {
				userId = types.CanNotFindUserId
			}
			userId = userInfo.Id
		}
		ip = c.ClientIP()
		operationLog := model.Audit{
			Ip:     ip,
			Agent:  httputils.GetUserAgent(c.Request.UserAgent()),
			Path:   urlPath,
			Method: c.Request.Method,
			Param:  string(param),
			UserID: userId,
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
		operationLog.PespResult = writer.body.String()

		go func() {
			if err := pixiu.CoreV1.Audit().Create(c, &operationLog); err != nil {
				log.Logger.Errorf("save operationLog error: %v", err)
			}
		}()
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
