package middleware

import (
	"errors"
	"github.com/caoyingjunz/gopixiu/api/server/common"
	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/pkg/log"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func CasbinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		p := c.Request.URL.Path
		if p == "/users/login" {
			return
		}
		// 用户ID
		uid, isExit := c.Get("userId")
		r := httputils.NewResponse()
		if !isExit {
			r.SetCode(common.ErrorCodePermissionDeny)
			httputils.SetFailed(c, r, "无权限")
			return
		}

		m := c.Request.Method
		e := pixiu.CoreV1.Casbin().GetEnforce()
		if e == nil {
			log.Logger.Errorf("cabin初始化失败.")
			return
		}
		uidStr := strconv.FormatInt(uid.(int64), 10)
		ok, err := e.Enforce(uidStr, p, m)
		if err != nil {
			log.Logger.Errorf(err.Error())
			r.SetCode(http.StatusInternalServerError)
			httputils.SetFailed(c, r, "内部错误")
			c.Abort()
			return
		}
		if !ok {
			r.SetCode(common.ErrorCodePermissionDeny)
			httputils.SetFailed(c, r, errors.New("无权限"))
			c.Abort()
			return
		}
		c.Next()
	}
}
