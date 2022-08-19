package controllers

import (
	"github.com/gin-gonic/gin"
	"pixiu/services"
)

func RunJob(c *gin.Context) {
	c.JSON(200, gin.H{"data": nil, "meta": gin.H{"status": 200, "msg": "Post Success"}})
	services.RunJob()
}

func CreateJob(c *gin.Context) {
	c.JSON(200, gin.H{"data": nil, "meta": gin.H{"status": 200, "msg": "Post Success"}})
	services.CreateJob()
}
func DeleteJob(c *gin.Context) {
	c.JSON(200, gin.H{"data": nil, "meta": gin.H{"status": 200, "msg": "Post Success"}})
	services.DeleteJob()
}

func AddViewJob(c *gin.Context) {
	c.JSON(200, gin.H{"data": nil, "meta": gin.H{"status": 200, "msg": "Post Success"}})
	services.AddViewJob()
}
