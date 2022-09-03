package sys

import (
	"context"
	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
	"github.com/caoyingjunz/gopixiu/pkg/util"
	"github.com/gin-gonic/gin"
)

func (*roleRouter) addRole(c *gin.Context) {
	r := httputils.NewResponse()
	var role model.Role
	if err := c.ShouldBindJSON(&role); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if _, err := pixiu.CoreV1.Role().Create(context.TODO(), &role); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (*roleRouter) updateRole(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		role model.Role
	)
	if err = c.ShouldBindJSON(&role); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	role.Id, err = util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = pixiu.CoreV1.Role().Update(context.TODO(), &role); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (*roleRouter) deleteRole(c *gin.Context) {
	r := httputils.NewResponse()
	rid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := pixiu.CoreV1.Role().Delete(context.TODO(), rid); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (*roleRouter) getRole(c *gin.Context) {
	r := httputils.NewResponse()
	rid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Role().Get(context.TODO(), rid)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (*roleRouter) listRoles(c *gin.Context) {
	r := httputils.NewResponse()
	var err error
	if r.Result, err = pixiu.CoreV1.Role().List(context.TODO()); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)

}
