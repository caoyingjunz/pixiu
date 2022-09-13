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

package cicd

import (
	"github.com/gin-gonic/gin"
)

// cicdRouter is a router to talk with the cicd controller
type cicdRouter struct{}

// NewRouter initializes a new cicd router
func NewRouter(ginEngine *gin.Engine) {
	s := &cicdRouter{}
	s.initRoutes(ginEngine)
}

func (s *cicdRouter) initRoutes(ginEngine *gin.Engine) {
	cicdRoute := ginEngine.Group("/cicd")
	{
		// 安全重启jenkins
		cicdRoute.POST("/restart", s.restart)
		//Job API
		cicdRoute.POST("/jobs/run", s.runJob)
		cicdRoute.POST("/jobs", s.createJob)
		cicdRoute.GET("/jobs", s.getAllJobs)
		cicdRoute.DELETE("/jobs/:name", s.deleteJob)
		cicdRoute.POST("/jobs/copy", s.copyJob)
		cicdRoute.POST("/jobs/rename", s.renameJob)
		cicdRoute.POST("/jobs/disable", s.disable)
		cicdRoute.POST("/jobs/enable", s.enable)
		cicdRoute.POST("/jobs/stop", s.stop)
		cicdRoute.POST("/jobs/config", s.config)
		cicdRoute.POST("/jobs/updateconfig", s.updateConfig)
		cicdRoute.POST("/view", s.addViewJob)
		cicdRoute.GET("/jobs/details/:name", s.details)
		cicdRoute.POST("/jobs/failed", s.getLastFailedBuild)
		cicdRoute.POST("/jobs/success", s.getLastSuccessfulBuild)
		cicdRoute.POST("/jobs/history", s.history)
		cicdRoute.GET("/view", s.getAllViews)
		cicdRoute.DELETE("/view/:name/:viewname", s.deleteViewJob)
		cicdRoute.GET("/nodes", s.getAllNodes)
		cicdRoute.DELETE("/nodes/:name", s.deleteNode)
	}
}
