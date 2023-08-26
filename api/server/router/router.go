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

	"github.com/caoyingjunz/pixiu/api/server/middleware"
	"github.com/caoyingjunz/pixiu/api/server/router/cloud"
	"github.com/caoyingjunz/pixiu/api/server/router/helm"
	"github.com/caoyingjunz/pixiu/api/server/router/proxy"
	"github.com/caoyingjunz/pixiu/api/server/router/user"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
)

type RegisterFunc func(*gin.Engine)

func InstallRouters(opt *options.Options) {
	fs := []RegisterFunc{
		middleware.InstallMiddlewares, user.NewRouter, cloud.NewRouter, proxy.NewRouter, helm.NewRouter,
	}

	install(opt.GinEngine, fs...)

	// 启动检查检查
	opt.GinEngine.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
}

func install(engine *gin.Engine, fs ...RegisterFunc) {
	for _, f := range fs {
		f(engine)
	}
}
