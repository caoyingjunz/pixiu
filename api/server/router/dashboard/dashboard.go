/*
Copyright 2026 The Pixiu Authors.

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

package dashboard

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

const dashboardBaseURL = "/pixiu/dashboard"

type dashboardRouter struct {
	c controller.PixiuInterface
}

func NewRouter(o *options.Options) {
	r := &dashboardRouter{c: o.Controller}
	r.initRoutes(o.HttpEngine)
}

func (r *dashboardRouter) initRoutes(engine *gin.Engine) {
	group := &apiregistry.Group{
		Name:    "仪表盘",
		BaseURL: dashboardBaseURL,
		Entries: []apiregistry.RouteEntry{
			{Method: "GET", RelativePath: "/definition", Handler: r.definition, Description: "查看仪表盘定义"},
			{Method: "GET", RelativePath: "/variables", Handler: r.variables, Description: "查看仪表盘筛选项"},
			{Method: "POST", RelativePath: "/query", Handler: r.query, Description: "查询仪表盘数据"},
		},
	}
	group.Register(engine.Group(dashboardBaseURL), r.c.APIResource())
}
