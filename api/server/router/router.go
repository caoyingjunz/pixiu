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
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	// 导入 docs.json 文件
	_ "github.com/caoyingjunz/pixiu/api/docs"
	_ "github.com/caoyingjunz/pixiu/api/server/validator"

	"github.com/caoyingjunz/pixiu/api/server/middleware"
	"github.com/caoyingjunz/pixiu/api/server/router/cluster"
	"github.com/caoyingjunz/pixiu/api/server/router/plan"
	"github.com/caoyingjunz/pixiu/api/server/router/proxy"
	"github.com/caoyingjunz/pixiu/api/server/router/tenant"
	"github.com/caoyingjunz/pixiu/api/server/router/user"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
)

type RegisterFunc func(o *options.Options)

func InstallRouters(o *options.Options) {
	fs := []RegisterFunc{
		middleware.InstallMiddlewares, cluster.NewRouter, proxy.NewRouter, tenant.NewRouter, user.NewRouter, plan.NewRouter,
	}

	install(o, fs...)

	// 启动健康检查
	o.HttpEngine.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	// 启动 APIs 服务
	o.HttpEngine.GET("/api-ref/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}

func install(o *options.Options, fs ...RegisterFunc) {
	for _, f := range fs {
		f(o)
	}
}
