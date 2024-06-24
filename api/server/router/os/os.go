package os

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

type OSRouter struct {
	c controller.PixiuInterface
}

func NewRouter(o *options.Options) {
	s := &OSRouter{
		c: o.Controller,
	}
	s.initRoutes(o.HttpEngine)
}

func (o *OSRouter) initRoutes(httpEngine *gin.Engine) {
	osRoute := httpEngine.Group("/pixiu/plan/os")
	{
		osRoute.GET("", o.getOsList)
	}
}
