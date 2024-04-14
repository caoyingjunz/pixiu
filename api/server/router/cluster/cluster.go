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
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
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

func (cr *clusterRouter) initRoutes(httpEngine *gin.Engine) {
	clusterRoute := httpEngine.Group("/pixiu/clusters")
	{
		clusterRoute.POST("", cr.createCluster)
		clusterRoute.PUT("/:clusterId", cr.updateCluster)
		clusterRoute.DELETE("/:clusterId", cr.deleteCluster)
		clusterRoute.GET("/:clusterId", cr.getCluster)
		clusterRoute.GET("", cr.listClusters)

		// 检查 kubernetes 的连通性
		clusterRoute.POST("/ping", cr.pingCluster)

		// 设置集群的删除保护模式
		clusterRoute.POST("/protect/:clusterId", cr.protectCluster)
	}

	// 调用 kubernetes 对象
	kubeRoute := httpEngine.Group("/pixiu/kubeproxy")
	{
		// Deprecated 聚合 events
		kubeRoute.GET("/clusters/:cluster/namespaces/:namespace/name/:name/kind/:kind/events", cr.aggregateEvents)
		// 获取指定对象的 events，支持事件聚合
		kubeRoute.GET("/clusters/:cluster/api/v1/events", cr.getEventList)

		// ws
		kubeRoute.GET("/ws", cr.webShell)
	}

	// 调用 helm 对象
	helmRoute := httpEngine.Group("/pixiu/helms")
	{
		// 获取 release 列表
		helmRoute.GET("/clusters/:cluster/v1/namespaces/:namespace/releases", cr.ListReleases)
	}
}
