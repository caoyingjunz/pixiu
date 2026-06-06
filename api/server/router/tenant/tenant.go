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

package tenant

import (
	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

type tenantRouter struct {
	c controller.PixiuInterface
}

func NewRouter(o *options.Options) {
	router := &tenantRouter{
		c: o.Controller,
	}
	router.initRoutes(o.HttpEngine)
}

func (t *tenantRouter) initRoutes(ginEngine *gin.Engine) {
	group := &apiregistry.Group{
		Name:    "租户管理",
		BaseURL: "/pixiu/tenants",
		Entries: []apiregistry.RouteEntry{
			{Method: "POST", RelativePath: "", Handler: t.createTenant, Description: "创建租户"},
			{Method: "PUT", RelativePath: "/:tenantId", Handler: t.updateTenant, Description: "更新租户"},
			{Method: "DELETE", RelativePath: "/:tenantId", Handler: t.deleteTenant, Description: "删除租户"},
			{Method: "GET", RelativePath: "/:tenantId", Handler: t.getTenant, Description: "查看详情"},
			{Method: "GET", RelativePath: "", Handler: t.listTenants, Description: "查看列表"},
		},
	}
	group.Register(ginEngine.Group("/pixiu/tenants"), t.c.APIResource())
}
