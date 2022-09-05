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

type userRouter struct{}

func NewRouter(ginEngine *gin.Engine) {
	u := &userRouter{}
	u.initRoutes(ginEngine)
}

func (u *userRouter) initRoutes(ginEngine *gin.Engine) {
	userRoute := ginEngine.Group("/users")
	{
		userRoute.POST("", u.createUser)
		userRoute.DELETE("/:id", u.deleteUser)
		userRoute.PUT("/:id", u.updateUser)
		userRoute.GET("/:id", u.getUser)
		userRoute.GET("", u.listUsers)

		// 用户的登陆或者退出
		userRoute.POST("/login", u.login)
		userRoute.POST("/:id/logout", u.logout)

		// 修改密码
		userRoute.PUT("/change/:id/password", u.changePassword)
		// 重置密码
	}
}
