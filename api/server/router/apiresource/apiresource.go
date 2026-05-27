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

package apiresource

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

type apiResourceRouter struct {
	c controller.PixiuInterface
}

func NewRouter(o *options.Options) {
	router := &apiResourceRouter{
		c: o.Controller,
	}
	router.initRoutes(o.HttpEngine)
}

func (a *apiResourceRouter) initRoutes(ginEngine *gin.Engine) {
	group := &apiregistry.Group{
		Name:    "API管理",
		BaseURL: "/pixiu/apis",
		Entries: []apiregistry.RouteEntry{
			{Method: "POST", RelativePath: "", Handler: a.createAPI, Description: "创建API"},
			{Method: "PUT", RelativePath: "/:apiId", Handler: a.updateAPI, Description: "更新API"},
			{Method: "DELETE", RelativePath: "/:apiId", Handler: a.deleteAPI, Description: "删除API"},
			{Method: "GET", RelativePath: "/:apiId", Handler: a.getAPI, Description: "API详情"},
			{Method: "GET", RelativePath: "", Handler: a.listAPIs, Description: "API列表"},
		},
	}
	group.Register(ginEngine.Group("/pixiu/apis"), a.c.APIResource())
}
