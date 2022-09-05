package user

import "github.com/gin-gonic/gin"

type menuRouter struct{}

func NewMenuRouter(ginEngine *gin.Engine) {
	u := &menuRouter{}
	u.initRoutes(ginEngine)
}

func (m *menuRouter) initRoutes(ginEngine *gin.Engine) {
	userRoute := ginEngine.Group("/menu")
	{
		userRoute.POST("", m.addMenu)
		userRoute.DELETE("/:id", m.deleteMenu)
		userRoute.PUT("/:id", m.updateMenu)
		userRoute.GET("/:id", m.getMenu)
		userRoute.GET("", m.listMenus)
	}
}
