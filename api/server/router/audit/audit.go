package audit

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
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

func (a *auditRouter) initRoutes(httpEngine *gin.Engine) {
	auditRoute := httpEngine.Group("/pixiu/audits")
	{
		//get日志
		auditRoute.GET("/:auditId", a.getAudit)
		auditRoute.GET("/", a.listAudits)
		auditRoute.DELETE("/:auditId", a.deleteAudit)
	}
}
