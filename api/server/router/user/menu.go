package user

import "github.com/gin-gonic/gin"

type menuRouter struct{}

func NewMenuRouter(ginEngine *gin.Engine) {
	u := &menuRouter{}
	u.initRoutes(ginEngine)
}

func (m *menuRouter) initRoutes(ginEngine *gin.Engine) {
	menuRoute := ginEngine.Group("/menus")
	{
		menuRoute.POST("", m.addMenu)
		menuRoute.PUT("/:id", m.updateMenu)
		menuRoute.DELETE("/:id", m.deleteMenu)
		menuRoute.GET("/:id", m.getMenu)
		menuRoute.GET("", m.listMenus)
	}
}
