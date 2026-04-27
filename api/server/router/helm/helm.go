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

package helm

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

const (
	helmBaseURL = "/pixiu/helms"
)

type helmRouter struct {
	c controller.PixiuInterface
}

func NewRouter(o *options.Options) {
	hr := &helmRouter{
		c: o.Controller,
	}
	hr.initRoutes(o.HttpEngine)
}

func (hr *helmRouter) initRoutes(httpEngine *gin.Engine) {

	helmRoute := httpEngine.Group(helmBaseURL)
	{
		// helm Repository
		helmRoute.POST("/repositories", hr.createRepository)
		helmRoute.PUT("/repositories/:id", hr.updateRepository)
		helmRoute.DELETE("/repositories/:id", hr.deleteRepository)
		helmRoute.GET("/repositories/:id", hr.getRepository)
		helmRoute.GET("/repositories", hr.listRepositories)

		helmRoute.GET("/repositories/:id/charts", hr.getRepoCharts)
		helmRoute.GET("/repositories/charts", hr.getRepoChartsByURL)
		helmRoute.GET("/repositories/values", hr.getChartValues)

		// Helm Release
		helmRoute.POST("/clusters/:cluster/namespaces/:namespace/releases", hr.InstallRelease)
		helmRoute.PUT("/clusters/:cluster/namespaces/:namespace/releases", hr.UpgradeRelease)
		helmRoute.DELETE("/clusters/:cluster/namespaces/:namespace/releases/:name", hr.UninstallRelease)
		helmRoute.GET("/clusters/:cluster/namespaces/:namespace/releases/:name", hr.GetRelease)
		helmRoute.GET("/clusters/:cluster/namespaces/:namespace/releases", hr.ListReleases)

		helmRoute.GET("/clusters/:cluster/namespaces/:namespace/releases/:name/history", hr.GetReleaseHistory)
		helmRoute.POST("/clusters/:cluster/namespaces/:namespace/releases/:name/rollback", hr.RollbackRelease)
	}
}
