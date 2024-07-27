package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

const (
	//操作对象
	User    = "users"
	Cluster = "clusters"
	Tenant  = "tenants"
	Plan    = "plans"
)

func Audit(o *options.Options) gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		if method == http.MethodGet {
			return
		}
		obj, sid, ok := httputils.GetObjectFromRequest(c)
		if !ok {
			return
		}
		saveAudit(o, c, obj, sid)

	}

}

func saveAudit(o *options.Options, c *gin.Context, obj, sid string) {
	var userName string
	ip := c.ClientIP()
	user := c.Value("user")
	if _, ok := user.(model.User); !ok {
		userName = "unknown"
	} else {
		userName = user.(model.User).Name
	}

	switch obj {
	case User:
		o.Factory.User().Get(context.TODO(), StringToInt64(sid))
		object := &model.Audit{
			Action:   c.Request.Method,
			Content:  buildContent(obj, c.Request.Method, c.Request.RequestURI),
			Ip:       ip,
			Operator: userName,
		}
		o.Factory.Audit().Create(context.TODO(), object)
	case Cluster:
		o.Factory.Cluster().Get(context.TODO(), StringToInt64(sid))
		object := &model.Audit{
			Action:   c.Request.Method,
			Content:  buildContent(obj, c.Request.Method, c.Request.RequestURI),
			Ip:       ip,
			Operator: userName,
		}
		o.Factory.Audit().Create(context.TODO(), object)
	case Plan:
		o.Factory.Plan().Get(context.TODO(), StringToInt64(sid))
		object := &model.Audit{
			Action:   c.Request.Method,
			Content:  buildContent(obj, c.Request.Method, c.Request.RequestURI),
			Ip:       ip,
			Operator: userName,
		}
		o.Factory.Audit().Create(context.TODO(), object)
	case Tenant:
		o.Factory.Tenant().Get(context.TODO(), StringToInt64(sid))
		object := &model.Audit{
			Action:   c.Request.Method,
			Content:  buildContent(obj, c.Request.Method, c.Request.RequestURI),
			Ip:       ip,
			Operator: userName,
		}
		o.Factory.Audit().Create(context.TODO(), object)
	}
}

func buildContent(obj, method, url string) string {
	return fmt.Sprintf("操作资源[%s];操作请求[%s]:[%s]", obj, method, url)
}

func StringToInt64(s string) int64 {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return i
}
