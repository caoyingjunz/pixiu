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

package distribution

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

const distributionURL = "/pixiu/distributions"

type distributionRouter struct {
	c controller.PixiuInterface
}

func NewRouter(o *options.Options) {
	router := &distributionRouter{
		c: o.Controller,
	}
	router.initRoutes(o.HttpEngine)
}

func (r *distributionRouter) initRoutes(ginEngine *gin.Engine) {
	group := &apiregistry.Group{
		Name:    "操作系统",
		BaseURL: distributionURL,
		Entries: []apiregistry.RouteEntry{
			{Method: "POST", RelativePath: "", Handler: r.createDistribution, Description: "创建操作系统"},
			{Method: "PUT", RelativePath: "/:distributionId", Handler: r.updateDistribution, Description: "更新操作系统"},
			{Method: "DELETE", RelativePath: "/:distributionId", Handler: r.deleteDistribution, Description: "删除操作系统"},
			{Method: "GET", RelativePath: "", Handler: r.listDistributions, Description: "操作系统列表"},
			{Method: "GET", RelativePath: "/:distributionId", Handler: r.getDistribution, Description: "操作系统详情"},
		},
	}
	group.Register(ginEngine.Group(distributionURL), r.c.APIResource())
}
