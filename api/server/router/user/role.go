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

package user

import "github.com/gin-gonic/gin"

type roleRouter struct{}

func NewRoleRouter(ginEngine *gin.Engine) {
	u := &roleRouter{}
	u.initRoutes(ginEngine)
}

func (r *roleRouter) initRoutes(ginEngine *gin.Engine) {
	roleRoute := ginEngine.Group("/roles")
	{
		// 添加角色
		roleRoute.POST("", r.addRole)
		// 删除角色
		roleRoute.DELETE("", r.deleteRole)
		// 根据角色ID更新角色信息
		roleRoute.PUT("/:id", r.updateRole)
		// 根据角色ID获取角色信息
		roleRoute.GET("/:id", r.getRole)
		// 获取所有角色
		roleRoute.GET("", r.listRoles)

		// 根据角色ID获取角色权限
		roleRoute.GET("/:id/menus", r.getMenusByRole)
		// 根据角色ID设置角色权限
		roleRoute.POST("/:id/menus", r.setRoleMenus)
	}
}
