package controllers

import (
	"github.com/gin-gonic/gin"
	"pixiu/services"
)

func RunJob(c *gin.Context) {
	services.RunJob()

	c.JSON(200, gin.H{"data": nil, "meta": gin.H{"status": 200, "msg": "Post Success"}})

}

func CreateJob(c *gin.Context) {
	services.CreateJob()
	c.JSON(200, gin.H{"data": nil, "meta": gin.H{"status": 200, "msg": "Post Success"}})
}
func DeleteJob(c *gin.Context) {
	services.DeleteJob()
	c.JSON(200, gin.H{"data": nil, "meta": gin.H{"status": 200, "msg": "Post Success"}})
}

func AddViewJob(c *gin.Context) {
	services.AddViewJob()
	c.JSON(200, gin.H{"data": nil, "meta": gin.H{"status": 200, "msg": "Post Success"}})
}
