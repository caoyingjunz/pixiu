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

package auth

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

type authRouter struct {
	c controller.PixiuInterface
}

func NewRouter(o *options.Options) {
	router := &authRouter{
		c: o.Controller,
	}
	router.initRoutes(o.HttpEngine)
}

func (a *authRouter) initRoutes(ge *gin.Engine) {
	authRoute := ge.Group("/pixiu/auth")
	{
		policyRoute := authRoute.Group("/policy")
		policyRoute.POST("", a.createPolicy)
	}
}
