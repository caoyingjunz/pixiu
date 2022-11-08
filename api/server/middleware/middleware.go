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
	"os"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/caoyingjunz/gopixiu/pkg/types"
)

var AlwaysAllowPath sets.String

func InitMiddlewares(ginEngine *gin.Engine) {
	// 初始化可忽略的请求路径
	AlwaysAllowPath = sets.NewString(types.HealthURL, types.LoginURL, types.LogoutURL)

	// 依次进行跨域，日志，单用户限速，总量限速，验证，和鉴权
	ginEngine.Use(Cors(), LoggerToFile(), UserRateLimiter(), Limiter(), Authentication())
	// TODO: 临时关闭
	if os.Getenv("DEBUG") != "true" {
		ginEngine.Use(Rbac())
	}
}
