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

package cloud

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

// cloudRouter is a router to talk with the cloud controller
type cloudRouter struct {
	c controller.PixiuInterface
}

// NewRouter initializes a new cloud router
func NewRouter(o *options.Options) {
	s := &cloudRouter{
		c: o.Controller,
	}
	s.initRoutes(o.HttpEngine)
}

func (s *cloudRouter) initRoutes(httpEngine *gin.Engine) {
	// Set a lower memory limit for multipart forms (default is 32 MiB)
	httpEngine.MaxMultipartMemory = 8 << 20 // 8 MiB

	//  k8s cluster API
	httpEngine.POST("/load/cloud", s.loadCloud)   // 上传已存在的k8s集群，直接导入 kubeConfig 文件
	httpEngine.POST("/build/cloud", s.buildCloud) // 自建 kubernetes 集群

	cloudRoute := httpEngine.Group("/clouds")
	{
		cloudRoute.DELETE("/:id", s.deleteCloud)
		cloudRoute.GET("/:id", s.getCloud)
		cloudRoute.GET("", s.listClouds)

		// 检查 kubernetes 的连通性
		cloudRoute.POST("/ping", s.pingCloud)
	}
}
