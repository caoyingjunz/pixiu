package audit

import (
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
	"github.com/gin-gonic/gin"
)

type auditRouter struct {
	c controller.PixiuInterface
}

func NewRouter(o *options.Options) {
	router := &auditRouter{
		c: o.Controller,
	}
	router.initRoutes(o.HttpEngine)
}

func (a *auditRouter) initRoutes(ginEngine *gin.Engine) {
	//get日志
	ginEngine.GET("/:auditId", a.getAudit)
	ginEngine.GET("/", a.listAudits)
	ginEngine.DELETE("/:auditId", a.deleteAudit)
}
