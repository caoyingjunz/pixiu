package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/gopixiu/api/server/httpstatus"
	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/pkg/log"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

func authentication() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/users/login" {
			return
		}
		// 用户ID
		uid, exist := c.Get("userId")
		r := httputils.NewResponse()
		if !exist {
			r.SetCode(http.StatusUnauthorized)
			httputils.SetFailed(c, r, httpstatus.NoPermission)
			return
		}

		method := c.Request.Method
		enforcer := pixiu.CoreV1.Authentication().GetEnforce()
		if enforcer == nil {
			log.Logger.Errorf("init casbin failed.")
			return
		}
		uidStr := strconv.FormatInt(uid.(int64), 10)
		ok, err := enforcer.Enforce(uidStr, path, method)
		if err != nil {
			log.Logger.Errorf(err.Error())
			r.SetCode(http.StatusInternalServerError)
			httputils.SetFailed(c, r, httpstatus.InnerError)
			c.Abort()
			return
		}
		if !ok {
			r.SetCode(http.StatusUnauthorized)
			httputils.SetFailed(c, r, httpstatus.NoPermission)
			c.Abort()
			return
		}
		c.Next()
	}
}
