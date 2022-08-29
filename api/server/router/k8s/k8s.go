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

package k8s

import "github.com/gin-gonic/gin"

type k8sRouter struct{}

func NewRouter(ginEngine *gin.Engine) {
	s := &k8sRouter{}
	s.initRoutes(ginEngine)
}

func (s *k8sRouter) initRoutes(ginEngine *gin.Engine) {
	k8sRoute := ginEngine.Group("/k8s")

	clusterRoute := k8sRoute.Group("/cluster")
	{
		clusterRoute.POST("/create", s.createCluster)
	}
}
