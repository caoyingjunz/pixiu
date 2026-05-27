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

package agent

import (
	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

type agentRouter struct {
	c controller.PixiuInterface
}

func NewRouter(o *options.Options) {
	router := &agentRouter{
		c: o.Controller,
	}
	router.initRoutes(o.HttpEngine)
}

func (a *agentRouter) initRoutes(ginEngine *gin.Engine) {
	persist := false
	group := &apiregistry.Group{
		Name:    "代理管理",
		BaseURL: "/pixiu/agents",
		Entries: []apiregistry.RouteEntry{
			{Method: "POST", RelativePath: "", Handler: a.createAgent, Description: "创建代理", Persist: &persist},
			{Method: "PUT", RelativePath: "/:agentId", Handler: a.updateAgent, Description: "更新代理", Persist: &persist},
			{Method: "DELETE", RelativePath: "/:agentId", Handler: a.deleteAgent, Description: "删除代理", Persist: &persist},
			{Method: "GET", RelativePath: "/:agentId", Handler: a.getAgent, Description: "代理详情", Persist: &persist},
			{Method: "GET", RelativePath: "", Handler: a.listAgents, Description: "代理列表", Persist: &persist},
		},
	}
	group.Register(ginEngine.Group("/pixiu/agents"), a.c.APIResource())
}
