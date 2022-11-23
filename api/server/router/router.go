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
	"github.com/caoyingjunz/gopixiu/api/server/middleware"
	"github.com/caoyingjunz/gopixiu/api/server/router/audit"
	"github.com/caoyingjunz/gopixiu/api/server/router/cicd"
	"github.com/caoyingjunz/gopixiu/api/server/router/cloud"
	"github.com/caoyingjunz/gopixiu/api/server/router/healthz"
	"github.com/caoyingjunz/gopixiu/api/server/router/menu"
	"github.com/caoyingjunz/gopixiu/api/server/router/role"
	"github.com/caoyingjunz/gopixiu/api/server/router/user"
	"github.com/caoyingjunz/gopixiu/cmd/app/options"
)

func InstallRouters(opt *options.Options) {
	middleware.InstallMiddlewares(opt.GinEngine) // 安装中间件

	cloud.NewRouter(opt.GinEngine)   // 注册 cloud 路由
	user.NewRouter(opt.GinEngine)    // 注册 user 路由
	cicd.NewRouter(opt.GinEngine)    // 注册 cicd 路由
	role.NewRouter(opt.GinEngine)    // 注册 role 路由
	menu.NewRouter(opt.GinEngine)    // 注册 menu 路由
	healthz.NewRouter(opt.GinEngine) // 注册 healthz 路由
	audit.NewRouter(opt.GinEngine)   // 注册 audit 路由
}
