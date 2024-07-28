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
		user, err := o.Factory.User().Get(context.TODO(), StringToInt64(sid))
		if err != nil {
			user = &model.User{}
		}
		object := &model.Audit{
			Action:   c.Request.Method,
			Content:  buildContent(obj, c.Request.Method, c.Request.RequestURI, user),
			Ip:       ip,
			Operator: userName,
		}
		o.Factory.Audit().Create(context.TODO(), object)
	case Cluster:
		cluster, err := o.Factory.Cluster().Get(context.TODO(), StringToInt64(sid))
		if err != nil {
			cluster = &model.Cluster{}
		}
		object := &model.Audit{
			Action:   c.Request.Method,
			Content:  buildContent(obj, c.Request.Method, c.Request.RequestURI, cluster),
			Ip:       ip,
			Operator: userName,
		}
		o.Factory.Audit().Create(context.TODO(), object)
	case Plan:
		plan, err := o.Factory.Plan().Get(context.TODO(), StringToInt64(sid))
		if err != nil {
			plan = &model.Plan{}
		}
		object := &model.Audit{
			Action:   c.Request.Method,
			Content:  buildContent(obj, c.Request.Method, c.Request.RequestURI, plan),
			Ip:       ip,
			Operator: userName,
		}
		o.Factory.Audit().Create(context.TODO(), object)
	case Tenant:
		tenant, err := o.Factory.Tenant().Get(context.TODO(), StringToInt64(sid))
		if err != nil {
			tenant = &model.Tenant{}
		}
		object := &model.Audit{
			Action:   c.Request.Method,
			Content:  buildContent(obj, c.Request.Method, c.Request.RequestURI, tenant),
			Ip:       ip,
			Operator: userName,
		}
		o.Factory.Audit().Create(context.TODO(), object)
	}
}

func buildContent(obj, method, url string, objModel interface{}) string {
	switch objModel.(type) {
	case *model.User:
		return fmt.Sprintf("操作资源[%s];操作请求[%s]:[%s];被操作对象[%s]", obj, method, url, objModel.(*model.User).Name)
	case *model.Cluster:
		return fmt.Sprintf("操作资源[%s];操作请求[%s]:[%s];被操作对象[%s]", obj, method, url, objModel.(*model.Cluster).Name)
	case *model.Plan:
		return fmt.Sprintf("操作资源[%s];操作请求[%s]:[%s];被操作对象[%s]", obj, method, url, objModel.(*model.Plan).Name)
	case *model.Tenant:
		return fmt.Sprintf("操作资源[%s];操作请求[%s]:[%s];被操作对象[%s]", obj, method, url, objModel.(*model.Tenant).Name)
	default:
		return fmt.Sprintf("操作资源[%s];操作请求[%s]:[%s];被操作对象[%s]", obj, method, url, "新增model请添加审计")
	}
}

func StringToInt64(s string) int64 {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return i
}
