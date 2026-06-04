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

	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
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
	userGroup := &apiregistry.Group{
		Name:    "用户管理",
		BaseURL: "/pixiu/users",
		Entries: []apiregistry.RouteEntry{
			{Method: "POST", RelativePath: "", Handler: u.createUser, Description: "创建用户"},
			{Method: "PUT", RelativePath: "/:userId", Handler: u.updateUser, Description: "更新用户"},
			{Method: "DELETE", RelativePath: "/:userId", Handler: u.deleteUser, Description: "删除用户"},
			{Method: "GET", RelativePath: "/:userId", Handler: u.getUser, Description: "查看详情"},
			{Method: "GET", RelativePath: "", Handler: u.listUsers, Description: "查看列表"},
			{Method: "PUT", RelativePath: "/:userId/password", Handler: u.updatePassword, Description: "修改密码"},
			{Method: "POST", RelativePath: "/login", Handler: u.login, Description: "登录"},
			{Method: "POST", RelativePath: "/:userId/logout", Handler: u.logout, Description: "登出"},
		},
	}
	userGroup.Register(httpEngine.Group("/pixiu/users"), u.c.APIResource())
}
