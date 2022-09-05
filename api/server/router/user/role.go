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
	userRoute := ginEngine.Group("/role")
	{
		userRoute.POST("", r.addRole)
		userRoute.DELETE("", r.deleteRole)
		userRoute.PUT("/:id", r.updateRole)
		userRoute.GET("/:id", r.getRole)
		userRoute.GET("", r.listRoles)

		// 获取角色权限
		userRoute.GET("/:id/menus", r.getMenusByRole)
		userRoute.POST("/:id/menus", r.setRoleMenus)

	}
}
