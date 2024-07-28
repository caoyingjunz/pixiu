/*
Copyright 2024 The Pixiu Authors.

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
	"context"
	"encoding/json"
	"k8s.io/klog/v2"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

// 自定义 ResponseWriter 用于捕获写入的数据
type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w responseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func Audit(o *options.Options) gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		body := new(bytes.Buffer)
		c.Writer = &responseWriter{c.Writer, body}
		c.Next()

		// 处理401错误处理
		if method == http.MethodGet {
			if c.Writer.Status() == http.StatusUnauthorized {
				goto auditNext
			}
			return
		}

	auditNext:
		// 获取写入的数据
		respBody := body.String()
		var respData map[string]interface{}

		// 尝试解析 JSON 数据
		status := model.OperationSuccess
		if err := json.Unmarshal([]byte(respBody), &respData); err != nil {
			status = model.OperationUnknow
		}
		if respData != nil && respData["code"] != http.StatusOK {
			status = model.OperationFail
		}

		obj, _, ok := httputils.GetObjectFromRequest(c)
		if !ok {
			return
		}

		// 持久化审计记录
		saveAudit(o, c, obj, status)
	}
}

func saveAudit(o *options.Options, c *gin.Context, obj string, status model.OperationStatus) {
	var userName string
	user := c.Value("user")
	if _, ok := user.(model.User); !ok {
		userName = "unknown"
	} else {
		userName = user.(model.User).Name
	}

	if _, err := o.Factory.Audit().Create(context.TODO(), &model.Audit{
		Action:       c.Request.Method,
		Ip:           c.ClientIP(),
		Operator:     userName,
		Path:         c.Request.RequestURI,
		ResourceType: obj,
		Status:       status,
	}); err != nil {
		klog.Error("failed to save %s action record: %v", userName, err)
	}
}
