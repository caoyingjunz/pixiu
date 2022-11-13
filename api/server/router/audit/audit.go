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

package audit

import (
	"github.com/gin-gonic/gin"
)

type auditRouter struct{}

func NewRouter(ginEngine *gin.Engine) {
	a := &auditRouter{}
	a.initRoutes(ginEngine)
}

func (a *auditRouter) initRoutes(ginEngine *gin.Engine) {
	auditRouter := ginEngine.Group("/audit")
	{
		// 逻辑删除操作记录
		auditRouter.DELETE("/operation_log", a.deleteOperationLog)
		// 分页查询操作记录
		auditRouter.GET("/operation_logs", a.listOperationLog)
	}
}
