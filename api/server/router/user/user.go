package user

import "github.com/gin-gonic/gin"

type userRouter struct{}

func (u *userRouter) initRoutes(ginEngine *gin.Engine) {
	userRoute := ginEngine.Group("/users")
	{
		userRoute.POST("/", u.create)
		userRoute.DELETE("/:id", u.delete)
		userRoute.PUT("/:id", u.update)
		userRoute.GET("/:id", u.get)
		userRoute.GET("/", u.list)
		userRoute.POST("/login", u.login)
		userRoute.POST("/:id/logout", u.logout)
	}
}

func NewRouter(ginEngine *gin.Engine) {
	u := &userRouter{}
	u.initRoutes(ginEngine)
}
