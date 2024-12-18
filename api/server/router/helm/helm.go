package helm

import (
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
	"github.com/gin-gonic/gin"
)

const (
	helmBaseURL = "/pixiu/helms"
)

// clusterRouter is a router to talk with the cluster controller
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
		// Helm Release API 列表
		helmRoute.POST("/clusters/:cluster/namespaces/:namespace/releases", hr.InstallRelease)
		helmRoute.PUT("/clusters/:cluster/namespaces/:namespace/releases", hr.UpgradeRelease)
		helmRoute.DELETE("/clusters/:cluster/namespaces/:namespace/releases/:name", hr.UninstallRelease)
		helmRoute.GET("/clusters/:cluster/namespaces/:namespace/releases/:name", hr.GetRelease)
		helmRoute.GET("/clusters/:cluster/namespaces/:namespace/releases", hr.ListReleases)

		// helm Repository
		helmRoute.POST("/repositories", hr.createRepository)
		helmRoute.PUT("/repositories/:id", hr.updateRepository)
		helmRoute.DELETE("/repositories/:id", hr.deleteRepository)
		helmRoute.GET("/repositories/:id", hr.getRepository)
		helmRoute.GET("/repositories", hr.listRepositories)

		helmRoute.GET("/repositories/:id/charts", hr.getRepoCharts)
		helmRoute.GET("/repositories/charts", hr.getRepoChartsByURL)
		helmRoute.GET("/repositories/values", hr.getChartValues)

	}
}
