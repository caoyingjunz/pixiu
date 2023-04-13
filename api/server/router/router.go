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

package router

import (
	"github.com/gin-gonic/gin"
	"net/http"

	"github.com/caoyingjunz/pixiu/api/server/middleware"
	"github.com/caoyingjunz/pixiu/api/server/router/audit"
	"github.com/caoyingjunz/pixiu/api/server/router/cicd"
	"github.com/caoyingjunz/pixiu/api/server/router/cloud"
	"github.com/caoyingjunz/pixiu/api/server/router/menu"
	"github.com/caoyingjunz/pixiu/api/server/router/proxy"
	"github.com/caoyingjunz/pixiu/api/server/router/role"
	"github.com/caoyingjunz/pixiu/api/server/router/user"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
)

func InstallRouters(opt *options.Options) {
	middleware.InstallMiddlewares(opt.GinEngine) // 安装中间件

	user.NewRouter(opt.GinEngine)  // 注册 user 路由
	role.NewRouter(opt.GinEngine)  // 注册 role 路由
	menu.NewRouter(opt.GinEngine)  // 注册 menu 路由
	audit.NewRouter(opt.GinEngine) // 注册 audit 路由

	// 注册 cicd 路由，根据配置文件中的开关判断是否注册
	if opt.ComponentConfig.Cicd.Enable {
		cicd.NewRouter(opt.GinEngine)
	}

	cloud.NewRouter(opt.GinEngine) // 注册 cloud 路由
	proxy.NewRouter(opt.GinEngine) // 注册 kubernetes proxy

	// 启动检查检查
	opt.GinEngine.GET("/healthz", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
}
