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

package runner

import (
	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

type runnerRouter struct {
	c controller.PixiuInterface
}

func NewRouter(o *options.Options) {
	router := &runnerRouter{
		c: o.Controller,
	}
	router.initRoutes(o.HttpEngine)
}

func (r *runnerRouter) initRoutes(ginEngine *gin.Engine) {
	persist := false
	group := &apiregistry.Group{
		Name:    "Runner",
		BaseURL: "/pixiu/runners",
		Entries: []apiregistry.RouteEntry{
			{Method: "POST", RelativePath: "", Handler: r.createRunner, Description: "创建 Runner", Persist: &persist},
			{Method: "PUT", RelativePath: "/:runnerId", Handler: r.updateRunner, Description: "更新 Runner", Persist: &persist},
			{Method: "DELETE", RelativePath: "/:runnerId", Handler: r.deleteRunner, Description: "删除 Runner", Persist: &persist},
			{Method: "GET", RelativePath: "/:runnerId", Handler: r.getRunner, Description: "Runner 详情", Persist: &persist},
			{Method: "GET", RelativePath: "", Handler: r.listRunners, Description: "Runner 列表", Persist: &persist},
		},
	}
	group.Register(ginEngine.Group("/pixiu/runners"), r.c.APIResource())
}
