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

package role

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

type roleRouter struct {
	c controller.PixiuInterface
}

func NewRouter(o *options.Options) {
	router := &roleRouter{
		c: o.Controller,
	}
	router.initRoutes(o.HttpEngine)
}

func (r *roleRouter) initRoutes(ginEngine *gin.Engine) {
	roleGroup := &apiregistry.Group{
		Name:    "角色管理",
		BaseURL: "/pixiu/roles",
		Entries: []apiregistry.RouteEntry{
			{Method: "POST", RelativePath: "", Handler: r.createRole, Description: "创建角色"},
			{Method: "PUT", RelativePath: "/:roleId", Handler: r.updateRole, Description: "更新角色"},
			{Method: "DELETE", RelativePath: "/:roleId", Handler: r.deleteRole, Description: "删除角色"},
			{Method: "GET", RelativePath: "/:roleId", Handler: r.getRole, Description: "查看详情"},
			{Method: "GET", RelativePath: "", Handler: r.listRoles, Description: "查看列表"},
			{Method: "GET", RelativePath: "/:roleId/apis", Handler: r.getRoleAPIs, Description: "查看权限"},
			{Method: "PUT", RelativePath: "/:roleId/apis", Handler: r.updateRoleAPIs, Description: "修改权限"},
			{Method: "GET", RelativePath: "/:roleId/api-scopes", Handler: r.getRoleAPIScopes, Description: "查看 Kubernetes 权限"},
			{Method: "PUT", RelativePath: "/:roleId/api-scopes", Handler: r.updateRoleAPIScopes, Description: "修改 Kubernetes 权限"},
		},
	}
	roleGroup.Register(ginEngine.Group("/pixiu/roles"), r.c.APIResource())
}
