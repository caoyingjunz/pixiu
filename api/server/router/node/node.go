package node

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

type nodeRouter struct {
	c controller.PixiuInterface
}

// NewRouter initializes a new cluster router
func NewRouter(o *options.Options) {
	s := &nodeRouter{
		c: o.Controller,
	}
	s.initRoutes(o.HttpEngine)
}

func (nr *nodeRouter) initRoutes(httpEngine *gin.Engine) {
	nodeRoute := httpEngine.Group("/pixiu/nodes")
	{
		nodeRoute.GET("/webterminal", nr.ServeConn)
	}
}
