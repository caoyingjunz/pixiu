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

// NewRouter initializes a new cloud router
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

		// Node API
		cloudRoute.GET("/v1/:cloud_name/nodes/:object_name", s.getNode)
		cloudRoute.GET("/v1/:cloud_name/nodes", s.listNodes)

		// Namespaces API
		cloudRoute.POST("/v1/:cloud_name/namespaces", s.createNamespace)
		cloudRoute.PUT("/v1/:cloud_name/namespaces/:object_name", s.updateNamespace)
		cloudRoute.DELETE("/v1/:cloud_name/namespaces/:object_name", s.deleteNamespace)
		cloudRoute.GET("/v1/:cloud_name/namespaces/:object_name", s.getNamespace)
		cloudRoute.GET("/v1/:cloud_name/namespaces", s.listNamespaces)

		// Service API
		cloudRoute.POST("/core/v1/:cloud_name/namespaces/:namespace/services", s.createService)
		cloudRoute.PUT("/core/v1/:cloud_name/namespaces/:namespace/services/:object_name", s.updateService)
		cloudRoute.DELETE("/core/v1/:cloud_name/namespaces/:namespace/services/:object_name", s.deleteService)
		cloudRoute.GET("/core/v1/:cloud_name/namespaces/:namespace/services/:object_name", s.getService)
		cloudRoute.GET("/core/v1/:cloud_name/namespaces/:namespace/services", s.listServices)

		// Deployments API
		// 创建 deployments
		cloudRoute.POST("/apps/v1/:cloud_name/namespaces/:namespace/deployments/:object_name", s.createDeployment)
		// listDeployments API: apps/v1/<cloud_name>/namespaces/<ns>/deployments
		cloudRoute.GET("/apps/v1/:cloud_name/namespaces/:namespace/deployments", s.listDeployments)
		cloudRoute.DELETE("/apps/v1/:cloud_name/namespaces/:namespace/deployments/:object_name", s.deleteDeployment)

		// Job API
		cloudRoute.POST("/batch/v1/:cloud_name/namespaces/:namespace/jobs/:object_name", s.createJob)
		cloudRoute.PUT("/batch/v1/:cloud_name/namespaces/:namespace/jobs/:object_name", s.updateJob)
		cloudRoute.DELETE("/batch/v1/:cloud_name/namespaces/:namespace/jobs/:object_name", s.deleteJob)
		cloudRoute.GET("/batch/v1/:cloud_name/namespaces/:namespace/jobs/:object_name", s.getJob)
		cloudRoute.GET("/batch/v1/:cloud_name/namespaces/:namespace/jobs", s.listJobs)

		// StatefulSet API
		cloudRoute.POST("/apps/v1/:cloud_name/namespaces/:namespace/statefulsets/:object_name", s.createStatefulSet)
		cloudRoute.PUT("/apps/v1/:cloud_name/namespaces/:namespace/statefulsets/:object_name", s.updateStatefulSet)
		cloudRoute.DELETE("/apps/v1/:cloud_name/namespaces/:namespace/statefulsets/:object_name", s.deleteStatefulSet)
		cloudRoute.GET("/apps/v1/:cloud_name/namespaces/:namespace/statefulsets/:object_name", s.getStatefulSet)
		cloudRoute.GET("/apps/v1/:cloud_name/namespaces/:namespace/statefulsets", s.listStatefulSets)

		// DaemonSet API
		cloudRoute.POST("/apps/v1/:cloud_name/namespaces/:namespace/daemonsets/:object_name", s.createDaemonSet)
		cloudRoute.PUT("/apps/v1/:cloud_name/namespaces/:namespace/daemonsets/:object_name", s.updateDaemonSet)
		cloudRoute.DELETE("/apps/v1/:cloud_name/namespaces/:namespace/daemonsets/:object_name", s.deleteDaemonSet)
		cloudRoute.GET("/apps/v1/:cloud_name/namespaces/:namespace/daemonsets/:object_name", s.getDaemonSet)
		cloudRoute.GET("/apps/v1/:cloud_name/namespaces/:namespace/daemonsets", s.listDaemonsets)
	}
}
