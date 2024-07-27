package middleware

import (
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/gin-gonic/gin"
)

func Audit() gin.HandlerFunc {
	return func(c *gin.Context) {
		httputils.SetIPToContext(c)
	}

}
