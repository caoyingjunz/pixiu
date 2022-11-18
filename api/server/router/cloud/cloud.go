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
		cloudRoute.POST("", s.createCloud)      // 导入已存在的k8s集群，直接导入 kubeConfig 文件
		cloudRoute.POST("/build", s.buildCloud) // 自建 kubernetes 集群
		cloudRoute.PUT("/:id", s.updateCloud)
		cloudRoute.DELETE("/:id", s.deleteCloud)
		cloudRoute.GET("/:id", s.getCloud)
		cloudRoute.GET("", s.listClouds)

		// 检查 kubernetes 的连通性
		cloudRoute.POST("/ping", s.pingCloud)

		// kubeConfig API
		// 点击生成指定权限的 kubeConfig，支持用完销毁
		cloudRoute.POST("/v1/:cloud_name/kubeconfigs", s.createKubeConfig)
		cloudRoute.PUT("/v1/:cloud_name/kubeconfigs/:id", s.updateKubeConfig)
		cloudRoute.DELETE("/v1/:cloud_name/kubeconfigs/:id", s.deleteKubeConfig)
		cloudRoute.GET("/v1/:cloud_name/kubeconfigs/:id", s.getKubeConfig)
		cloudRoute.GET("/v1/:cloud_name/kubeconfigs", s.listKubeConfig)

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

		// Event API
		// TODO: 事件的优化，精细化输出
		cloudRoute.GET("/core/v1/:cloud_name/namespaces/:namespace/events", s.listEvents)

		//webShell API
		cloudRoute.GET("/webshell/ws", s.webShell)

		// Deployments API
		cloudRoute.POST("/apps/v1/:cloud_name/namespaces/:namespace/deployments", s.createDeployment)
		cloudRoute.PUT("/apps/v1/:cloud_name/namespaces/:namespace/deployments/:object_name", s.updateDeployment)
		cloudRoute.DELETE("/apps/v1/:cloud_name/namespaces/:namespace/deployments/:object_name", s.deleteDeployment)
		// listDeployments API: apps/v1/<cloud_name>/namespaces/<ns>/deployments
		cloudRoute.GET("/apps/v1/:cloud_name/namespaces/:namespace/deployments", s.listDeployments)

		// StatefulSet API
		cloudRoute.POST("/apps/v1/:cloud_name/namespaces/:namespace/statefulsets", s.createStatefulSet)
		cloudRoute.PUT("/apps/v1/:cloud_name/namespaces/:namespace/statefulsets/:object_name", s.updateStatefulSet)
		cloudRoute.DELETE("/apps/v1/:cloud_name/namespaces/:namespace/statefulsets/:object_name", s.deleteStatefulSet)
		cloudRoute.GET("/apps/v1/:cloud_name/namespaces/:namespace/statefulsets/:object_name", s.getStatefulSet)
		cloudRoute.GET("/apps/v1/:cloud_name/namespaces/:namespace/statefulsets", s.listStatefulSets)

		// DaemonSet API
		cloudRoute.POST("/apps/v1/:cloud_name/namespaces/:namespace/daemonsets", s.createDaemonSet)
		cloudRoute.PUT("/apps/v1/:cloud_name/namespaces/:namespace/daemonsets/:object_name", s.updateDaemonSet)
		cloudRoute.DELETE("/apps/v1/:cloud_name/namespaces/:namespace/daemonsets/:object_name", s.deleteDaemonSet)
		cloudRoute.GET("/apps/v1/:cloud_name/namespaces/:namespace/daemonsets/:object_name", s.getDaemonSet)
		cloudRoute.GET("/apps/v1/:cloud_name/namespaces/:namespace/daemonsets", s.listDaemonsets)

		// Pod API
		cloudRoute.GET("/apps/v1/:cloud_name/namespaces/:namespace/pods/:object_name/logs", s.getLog)

		// Ingress API
		cloudRoute.POST("/network/v1/:cloud_name/namespaces/:namespace/ingress", s.createIngress)
		cloudRoute.DELETE("/network/v1/:cloud_name/namespaces/:namespace/ingress/:object_name", s.deleteIngress)
		cloudRoute.GET("/network/v1/:cloud_name/namespaces/:namespace/ingress/:object_name", s.getIngress)
		cloudRoute.GET("/network/v1/:cloud_name/namespaces/:namespace/ingress", s.listIngress)

		// Job API
		cloudRoute.POST("/batch/v1/:cloud_name/namespaces/:namespace/jobs", s.createJob)
		cloudRoute.PUT("/batch/v1/:cloud_name/namespaces/:namespace/jobs/:object_name", s.updateJob)
		cloudRoute.DELETE("/batch/v1/:cloud_name/namespaces/:namespace/jobs/:object_name", s.deleteJob)
		cloudRoute.GET("/batch/v1/:cloud_name/namespaces/:namespace/jobs/:object_name", s.getJob)
		cloudRoute.GET("/batch/v1/:cloud_name/namespaces/:namespace/jobs", s.listJobs)

	}
}
