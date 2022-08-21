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

package service

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

func InitCicdRouter(ginEngine *gin.Engine) {
	cicdRouter := ginEngine.Group("/cicd")
	{
		cicdRouter.POST("/job/run", runJob)
		cicdRouter.POST("/job/createJob", createJob)
		cicdRouter.DELETE("/job/deleteJob", deleteJob)
		cicdRouter.POST("/view/addJob", addViewJob)
	}
}

func InitCloudRouter(ginEngine *gin.Engine) {
	cloudRouter := ginEngine.Group("/cloud")
	{
		cloudRouter.GET("getCloud", func(context *gin.Context) {
			fmt.Println("TODO")
		})
	}
}
