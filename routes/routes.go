package routes

import (
	"github.com/gin-gonic/gin"
	"pixiu/controllers"
)

func RoutesRunJob() *gin.Engine {
	router := gin.Default()
	router.GET("/api/job/run", controllers.RunJob)
	router.GET("/create", controllers.CreateJob)
	return router
}
