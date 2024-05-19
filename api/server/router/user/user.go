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

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

type userRouter struct {
	c controller.PixiuInterface
}

func NewRouter(o *options.Options) {
	router := &userRouter{
		c: o.Controller,
	}
	router.initRoutes(o.HttpEngine)
}

func (u *userRouter) initRoutes(httpEngine *gin.Engine) {
	// TODO: Base pixiu 后续作为常量定义
	userRoute := httpEngine.Group("/pixiu/users")
	{
		userRoute.POST("", u.createUser)
		userRoute.PUT("/:userId", u.updateUser)
		userRoute.DELETE("/:userId", u.deleteUser)
		userRoute.GET("/:userId", u.getUser)
		userRoute.GET("", u.listUsers)

		// 用户修改密码或者管理员重置密码
		userRoute.PUT("/:userId/password", u.updatePassword)

		// 用户的登陆或者退出
		userRoute.POST("/login", u.login)
		userRoute.POST("/:userId/logout", u.logout)
	}
}
