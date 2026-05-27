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

package node

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

type nodeRouter struct {
	c controller.PixiuInterface
}

func NewRouter(o *options.Options) {
	router := &nodeRouter{
		c: o.Controller,
	}
	router.initRoutes(o.HttpEngine)
}

func (n *nodeRouter) initRoutes(ginEngine *gin.Engine) {
	group := &apiregistry.Group{
		Name:    "节点管理",
		BaseURL: "/pixiu/nodes",
		Entries: []apiregistry.RouteEntry{
			{Method: "POST", RelativePath: "", Handler: n.createNode, Description: "创建节点"},
			{Method: "PUT", RelativePath: "/:nodeId", Handler: n.updateNode, Description: "更新节点"},
			{Method: "DELETE", RelativePath: "/:nodeId", Handler: n.deleteNode, Description: "删除节点"},
			{Method: "GET", RelativePath: "/:nodeId", Handler: n.getNode, Description: "节点详情"},
			{Method: "GET", RelativePath: "", Handler: n.listNodes, Description: "节点列表"},
		},
	}
	group.Register(ginEngine.Group("/pixiu/nodes"), n.c.APIResource())
}
