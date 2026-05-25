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

package cluster

import (
	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

const (
	kubeProxyBaseURL = "/pixiu/kubeproxy"
	helmBaseURL      = "/pixiu/helms"
	indexerBaseURL   = "/pixiu/indexer"
)

// clusterRouter is a router to talk with the cluster controller
type clusterRouter struct {
	c controller.PixiuInterface
}

// NewRouter initializes a new cluster router
func NewRouter(o *options.Options) {
	s := &clusterRouter{
		c: o.Controller,
	}
	s.initRoutes(o.HttpEngine)
}

func (cr *clusterRouter) initRoutes(ginEngine *gin.Engine) {
	group := &apiregistry.Group{
		Name:    "集群管理",
		BaseURL: "/pixiu/clusters",
		Entries: []apiregistry.RouteEntry{
			{Method: "POST", RelativePath: "", Handler: cr.createCluster, Description: "创建集群"},
			{Method: "PUT", RelativePath: "/:clusterId", Handler: cr.updateCluster, Description: "更新集群"},
			{Method: "DELETE", RelativePath: "/:clusterId", Handler: cr.deleteCluster, Description: "删除集群"},
			{Method: "GET", RelativePath: "/:clusterId", Handler: cr.getCluster, Description: "获取集群详情"},
			{Method: "GET", RelativePath: "", Handler: cr.listClusters, Description: "获取集群列表"},
			{Method: "POST", RelativePath: "/ping", Handler: cr.pingCluster, Description: "检查K8s连通性"},
			{Method: "POST", RelativePath: "/protect/:clusterId", Handler: cr.protectCluster, Description: "设置集群删除保护"},
		},
	}
	group.Register(ginEngine.Group("/pixiu/clusters"), cr.c.APIResource())

	// kube proxy 路由（透传 K8s API，不注册到 API 管理）
	kubeRoute := ginEngine.Group(kubeProxyBaseURL)
	{
		kubeRoute.GET("/clusters/:cluster/namespaces/:namespace/pods/:pod/log", cr.watchPodLog)
		kubeRoute.GET("/clusters/:cluster/namespaces/:namespace/name/:name/kind/:kind/events", cr.aggregateEvents)
		kubeRoute.GET("/clusters/:cluster/api/v1/events", cr.getEventList)
		kubeRoute.GET("/ws", cr.webShell)
		kubeRoute.GET("/nodes/ws", cr.nodeWebShell)
		kubeRoute.POST("/clusters/:cluster/namespaces/:namespace/jobs/:name", cr.ReRunJob)
	}
}
