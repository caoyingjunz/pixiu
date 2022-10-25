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

package menu

import "github.com/gin-gonic/gin"

type menuRouter struct{}

func NewRouter(ginEngine *gin.Engine) {
	u := &menuRouter{}
	u.initRoutes(ginEngine)
}

func (m *menuRouter) initRoutes(ginEngine *gin.Engine) {
	menuRoute := ginEngine.Group("/menus")
	{
		menuRoute.POST("", m.addMenu)
		menuRoute.PUT("/:id", m.updateMenu)
		menuRoute.DELETE("/:id", m.deleteMenu)
		menuRoute.GET("/:id", m.getMenu)
		menuRoute.GET("", m.listMenus)
		menuRoute.PUT("/:id/status/:status", m.updateMenuStatus)
	}
}
