/*
Copyright 2026 The Pixiu Authors.

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

package auth

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

type authRouter struct {
	c controller.PixiuInterface
}

const (
	authBaseURL = "/pixiu/auth"
)

var persistPublicAuthAPI = false

func NewRouter(o *options.Options) {
	router := &authRouter{
		c: o.Controller,
	}
	router.initRoutes(o.HttpEngine)
}

func (a *authRouter) initRoutes(httpEngine *gin.Engine) {
	authGroup := &apiregistry.Group{
		Name:    "认证",
		BaseURL: authBaseURL,
		Entries: []apiregistry.RouteEntry{
			// 登录相关APIs
			{Method: "POST", RelativePath: "/login", Handler: a.login, Description: "登录"},
			{Method: "POST", RelativePath: "/logout", Handler: a.logout, Description: "登出"},
			{Method: "POST", RelativePath: "/refresh", Handler: a.refresh, Description: "刷新登录token（暂未实现）"},

			{Method: "POST", RelativePath: "/verification-codes", Handler: a.sendVerificationCode, Description: "发送注册验证码", Persist: &persistPublicAuthAPI},
			{Method: "POST", RelativePath: "/register", Handler: a.registerUser, Description: "注册用户", Persist: &persistPublicAuthAPI},
			{Method: "GET", RelativePath: "/oauth/providers", Handler: a.listOAuthProviders, Description: "查看第三方登录源"},
			{Method: "GET", RelativePath: "/oauth/providers/:provider/config", Handler: a.getOAuthProviderConfig, Description: "查看第三方登录配置"},
			{Method: "PUT", RelativePath: "/oauth/providers/:provider/config", Handler: a.updateOAuthProviderConfig, Description: "更新第三方登录配置"},
			{Method: "GET", RelativePath: "/oauth/providers/:provider/login-url", Handler: a.getOAuthProviderLoginURL, Description: "获取第三方登录地址"},
			{Method: "POST", RelativePath: "/oauth/providers/:provider/login", Handler: a.loginWithOAuthProvider, Description: "第三方登录"},
		},
	}
	authGroup.Register(httpEngine.Group(authBaseURL), a.c.APIResource())
}
