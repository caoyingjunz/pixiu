package helm

import (
	"github.com/gin-gonic/gin"
)

type helmRouter struct{}

type Helm struct {
	CloudName string `uri:"cloud_name" binding:"required"`
	Namespace string `uri:"namespace" binding:"required"`
}

func NewRouter(ginEngine *gin.Engine) {
	router := &helmRouter{}
	router.initRoutes(ginEngine)
}

func (h helmRouter) initRoutes(ginEngine *gin.Engine) {
	menuRoute := ginEngine.Group("/pixiu/helm")
	{
		menuRoute.GET("/:cloud_name/apis/apps/v1/namespaces/:namespace/releases", h.ListReleasesHandler)
	}
}
