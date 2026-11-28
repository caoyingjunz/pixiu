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

// Package networkpolicy 提供 NetworkPolicy 的 HTTP 接口。
// 路由挂在 /pixiu/clusters 下，命名空间作用域，与集群资源的其他接口保持一致。
package networkpolicy

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

const networkPolicyBaseURL = "/pixiu/clusters"

type networkPolicyRouter struct {
	c controller.PixiuInterface
}

func NewRouter(o *options.Options) {
	router := &networkPolicyRouter{
		c: o.Controller,
	}
	router.initRoutes(o.HttpEngine)
}

func (r *networkPolicyRouter) initRoutes(ginEngine *gin.Engine) {
	group := &apiregistry.Group{
		Name:    "网络策略",
		BaseURL: networkPolicyBaseURL,
		Entries: []apiregistry.RouteEntry{
			{Method: "GET", RelativePath: "/:clusterId/namespaces/:namespace/networkpolicies", Handler: r.listNetworkPolicies, Description: "网络策略列表"},
			{Method: "POST", RelativePath: "/:clusterId/namespaces/:namespace/networkpolicies", Handler: r.createNetworkPolicy, Description: "创建网络策略"},
			{Method: "GET", RelativePath: "/:clusterId/namespaces/:namespace/networkpolicies/:name", Handler: r.getNetworkPolicy, Description: "查看网络策略"},
			{Method: "PUT", RelativePath: "/:clusterId/namespaces/:namespace/networkpolicies/:name", Handler: r.updateNetworkPolicy, Description: "更新网络策略"},
			{Method: "DELETE", RelativePath: "/:clusterId/namespaces/:namespace/networkpolicies/:name", Handler: r.deleteNetworkPolicy, Description: "删除网络策略"},
		},
	}
	group.Register(ginEngine.Group(networkPolicyBaseURL), r.c.APIResource())
}
