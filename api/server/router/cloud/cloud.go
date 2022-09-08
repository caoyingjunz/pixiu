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

import "github.com/gin-gonic/gin"

// cloudRouter is a router to talk with the cloud controller
type cloudRouter struct{}

// NewRouter initializes a new container router
func NewRouter(ginEngine *gin.Engine) {
	s := &cloudRouter{}
	s.initRoutes(ginEngine)
}

func (s *cloudRouter) initRoutes(ginEngine *gin.Engine) {
	// Set a lower memory limit for multipart forms (default is 32 MiB)
	ginEngine.MaxMultipartMemory = 8 << 20 // 8 MiB

	cloudRoute := ginEngine.Group("/clouds")
	{
		//  k8s cluster API
		cloudRoute.POST("/:name", s.createCloud) // TODO: will optimise
		cloudRoute.PUT("/:cid", s.updateCloud)
		cloudRoute.DELETE("/:cid", s.deleteCloud)
		cloudRoute.GET("/:cid", s.getCloud)
		cloudRoute.GET("", s.listClouds)

		// Namespaces API
		cloudRoute.POST("/v1/:cloud_name/namespaces/:object_name", s.createNamespace)
		cloudRoute.PUT("/v1/:cloud_name/namespaces/:object_name", s.updateNamespace)
		cloudRoute.DELETE("/v1/:cloud_name/namespaces/:object_name", s.deleteNamespace)
		cloudRoute.GET("/v1/:cloud_name/namespaces/:object_name", s.getNamespace)
		cloudRoute.GET("/v1/:cloud_name/namespaces", s.listNamespaces)

		// Deployments API
		// 创建 deployments
		cloudRoute.POST("/apps/v1/:cloud_name/namespaces/:namespace/deployments/:object_name", s.createDeployment)
		// listDeployments API: apps/v1/<cloud_name>/namespaces/<ns>/deployments
		cloudRoute.GET("/apps/v1/:cloud_name/namespaces/:namespace/deployments", s.listDeployments)
		cloudRoute.DELETE("/apps/v1/:cloud_name/namespaces/:namespace/deployments/:object_name", s.deleteDeployment)

		// Job API
		cloudRoute.GET("/batch/v1/:cloud_name/namespaces/:namespace/jobs", s.listJobs)
	}
}
