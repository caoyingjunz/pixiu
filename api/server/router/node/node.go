package node

import (
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
	"github.com/gin-gonic/gin"
)

type nodeRouter struct {
	c controller.PixiuInterface
}

func NewRouter(o *options.Options) {
	router := &nodeRouter{
		c: o.Controller,
	}
	router.initRoutes(o.HttpEngine)
}

func (n *nodeRouter) initRoutes(httpEngine *gin.Engine) {
	nodeRoute := httpEngine.Group("/pixiu/node")
	{
		nodeRoute.GET("", n.serveConn)
	}
}
