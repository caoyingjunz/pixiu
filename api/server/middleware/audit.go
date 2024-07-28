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

	emptyString = ""
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
		u := &model.User{}
		if sid == emptyString {
			u.Name = "新增user"
		} else {
			userModel, err := o.Factory.User().Get(context.TODO(), StringToInt64(sid))
			if err != nil {
				u.Name = "未获取操作对象"
			} else {
				u = userModel
			}
		}

		object := &model.Audit{
			Action:       c.Request.Method,
			Content:      buildContent(obj, c.Request.Method, c.Request.RequestURI, u),
			Ip:           ip,
			Operator:     userName,
			Path:         c.Request.RequestURI,
			ResourceType: obj,
		}
		o.Factory.Audit().Create(context.TODO(), object)
	case Cluster:
		cluster := &model.Cluster{}
		if sid == emptyString {
			cluster.Name = "新增cluster"
		} else {
			clusterModel, err := o.Factory.Cluster().Get(context.TODO(), StringToInt64(sid))
			if err != nil {
				cluster.Name = "未获取操作对象"
			} else {
				cluster = clusterModel
			}
		}

		object := &model.Audit{
			Action:       c.Request.Method,
			Content:      buildContent(obj, c.Request.Method, c.Request.RequestURI, cluster),
			Ip:           ip,
			Operator:     userName,
			Path:         c.Request.RequestURI,
			ResourceType: obj,
		}
		o.Factory.Audit().Create(context.TODO(), object)
	case Plan:
		plan := &model.Plan{}
		if sid == emptyString {
			plan.Name = "新增plan"
		} else {
			planModel, err := o.Factory.Plan().Get(context.TODO(), StringToInt64(sid))
			if err != nil {
				plan.Name = "未获取操作对象"
			} else {
				plan = planModel
			}
		}

		object := &model.Audit{
			Action:       c.Request.Method,
			Content:      buildContent(obj, c.Request.Method, c.Request.RequestURI, plan),
			Ip:           ip,
			Operator:     userName,
			Path:         c.Request.RequestURI,
			ResourceType: obj,
		}
		o.Factory.Audit().Create(context.TODO(), object)
	case Tenant:
		tenant := &model.Tenant{}
		if sid == emptyString {
			tenant.Name = "新增plan"
		} else {
			tenantModel, err := o.Factory.Tenant().Get(context.TODO(), StringToInt64(sid))
			if err != nil {
				tenant.Name = "未获取操作对象"
			} else {
				tenant = tenantModel
			}
		}

		object := &model.Audit{
			Action:       c.Request.Method,
			Content:      buildContent(obj, c.Request.Method, c.Request.RequestURI, tenant),
			Ip:           ip,
			Operator:     userName,
			Path:         c.Request.RequestURI,
			ResourceType: obj,
		}
		o.Factory.Audit().Create(context.TODO(), object)
	default:
		object := &model.Audit{
			Action:       c.Request.Method,
			Content:      buildContent(obj, c.Request.Method, c.Request.RequestURI, nil),
			Ip:           ip,
			Operator:     userName,
			Path:         c.Request.RequestURI,
			ResourceType: obj,
		}
		o.Factory.Audit().Create(context.TODO(), object)
	}
}

func buildContent(obj, method, url string, objModel interface{}) string {
	switch objModel.(type) {
	case *model.User:
		return fmt.Sprintf("操作资源[%s];操作请求[%s]:[%s];被操作对象信息[%s]", obj, method, url, objModel.(*model.User).Name)
	case *model.Cluster:
		return fmt.Sprintf("操作资源[%s];操作请求[%s]:[%s];被操作对象信息[%s]", obj, method, url, objModel.(*model.Cluster).Name)
	case *model.Plan:
		return fmt.Sprintf("操作资源[%s];操作请求[%s]:[%s];被操作对象信息[%s]", obj, method, url, objModel.(*model.Plan).Name)
	case *model.Tenant:
		return fmt.Sprintf("操作资源[%s];操作请求[%s]:[%s];被操作对象信息[%s]", obj, method, url, objModel.(*model.Tenant).Name)
	default:
		return fmt.Sprintf("操作资源[%s];操作请求[%s]:[%s];被操作对象信息[%s]", obj, method, url, "新增model请添加审计信息")
	}
}

func StringToInt64(s string) int64 {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return i
}
