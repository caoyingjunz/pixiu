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

package role

import "github.com/gin-gonic/gin"

type roleRouter struct{}

func NewRouter(ginEngine *gin.Engine) {
	o := &roleRouter{}
	o.initRoutes(ginEngine)
}

func (o *roleRouter) initRoutes(ginEngine *gin.Engine) {
	roleRoute := ginEngine.Group("/roles")
	{
		roleRoute.POST("", o.addRole)          // 添加角色
		roleRoute.PUT("/:id", o.updateRole)    // 根据角色ID更新角色信息
		roleRoute.DELETE("/:id", o.deleteRole) // 删除角色
		roleRoute.GET("/:id", o.getRole)       // 根据角色ID获取角色信息
		roleRoute.GET("", o.listRoles)         // 获取所有角色

		roleRoute.GET("/:id/menus", o.getMenusByRole) // 根据角色ID获取角色权限
		roleRoute.POST("/:id/menus", o.setRoleMenus)  // 根据角色ID设置角色权限
		roleRoute.PUT("/:id/status/:status", o.updateRoleStatus)
	}
}
