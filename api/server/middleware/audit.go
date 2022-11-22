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
	"context"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
	"github.com/caoyingjunz/gopixiu/pkg/types"
)

// Audit 操作记录
func Audit() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		go func(c *gin.Context) {
			// 临时关闭
			if true {
				return
			}
			// TODO: 继续实现
			handleEvent(&types.Event{
				ClientIP: c.ClientIP(),
				Operator: c.Value("create").(types.EventType),
				Message:  c.Value("message").(string),
			})
		}(c)
	}
}

func handleEvent(event *types.Event) {
	_ = pixiu.CoreV1.Audit().Create(context.TODO(), event)
}
