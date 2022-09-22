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

package apidocs

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// docsRouter is a router to talk with the docs controller
type docsRouter struct{}

// NewRouter initializes a new cloud router
func NewRouter(ginEngine *gin.Engine) {
	s := &docsRouter{}
	s.initRoutes(ginEngine)
}

func (s *docsRouter) initRoutes(ginEngine *gin.Engine) {
	apiRefRoute := ginEngine.Group("/api-ref")

	{
		apiRefRoute.GET("/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}
}
