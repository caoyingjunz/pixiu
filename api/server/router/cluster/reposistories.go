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

// import (
// 	"github.com/caoyingjunz/pixiu/cmd/app/options"
// 	"github.com/caoyingjunz/pixiu/pkg/controller"
// 	"github.com/gin-gonic/gin"
// )

// func (cr *clusterRouter) initRoutes(httpEngine *gin.Engine) {
// 	repoRoute := httpEngine.Group("/pixiu/reposistories")
// 	{
// 		repoRoute.GET("", cr.listReposistories)
// 		repoRoute.POST("", r.createReposistories)
// 		repoRoute.GET("/:id", r.getReposistory)
// 		repoRoute.GET("/name/:name", r.getReposistoryByName)
// 		repoRoute.PUT("/:id", r.updateReposistory)
// 		repoRoute.DELETE("/:id", r.deleteReposistory)
// 		repoRoute.GET("/:id/charts", r.getRepoCharts)
// 		repoRoute.GET("/charts", r.getRepoChartsByURL)
// 	}
// }
