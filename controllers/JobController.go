package controllers

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"pixiu/services"
)

func RunJob(c *gin.Context) {
	c.String(http.StatusOK, "hello World!")
	services.RunJob()
}

func CreateJob(c *gin.Context) {
	c.String(http.StatusOK, "create job")
}
