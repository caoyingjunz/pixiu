package routes

import (
	"github.com/gin-gonic/gin"
	"pixiu/controllers"
)

func RoutesRunJob() *gin.Engine {
	router := gin.Default()
	router.GET("/api/job/runjob", controllers.RunJob)
	router.GET("/api/job/createjob", controllers.CreateJob)
	router.GET("/api/job/deletejob", controllers.DeleteJob)
	router.GET("/api/job/addviewjob", controllers.AddViewJob)
	return router
}
