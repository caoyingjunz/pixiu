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

package audit

import (
	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

type auditRouter struct {
	c controller.PixiuInterface
}

func NewRouter(o *options.Options) {
	router := &auditRouter{
		c: o.Controller,
	}
	router.initRoutes(o.HttpEngine)
}

func (a *auditRouter) initRoutes(ginEngine *gin.Engine) {
	group := &apiregistry.Group{
		Name:    "审计",
		BaseURL: "/pixiu/audits",
		Entries: []apiregistry.RouteEntry{
			//{Method: "GET", RelativePath: "/:auditId", Handler: a.getAudit, Description: "获取审计日志详情"},
			{Method: "GET", RelativePath: "", Handler: a.listAudits, Description: "获取审计列表"},
		},
	}
	group.Register(ginEngine.Group("/pixiu/audits"), a.c.APIResource())
}
