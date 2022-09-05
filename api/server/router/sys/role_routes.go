package sys

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/log"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
	"github.com/caoyingjunz/gopixiu/pkg/util"
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
	var rids []int64
	roleIds := map[string][]int64{"role_ids": rids}
	err := c.ShouldBindJSON(&roleIds)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := pixiu.CoreV1.Role().Delete(c, roleIds["role_ids"]); err != nil {
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

func (*roleRouter) getMenusByRole(c *gin.Context) {
	r := httputils.NewResponse()
	rid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = pixiu.CoreV1.Role().GetMenusByRoleID(c, rid); err != nil {
		r.SetCode(http.StatusBadRequest)
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (*roleRouter) setRoleMenus(c *gin.Context) {
	r := httputils.NewResponse()
	rid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		log.Logger.Errorf(err.Error())
		httputils.SetFailed(c, r, "参数错误")
		return
	}
	var menus []int64
	menusStruct := map[string][]int64{"menu_ids": menus}

	if err := c.ShouldBindJSON(&menusStruct); err != nil {
		log.Logger.Errorf(err.Error())
		httputils.SetFailed(c, r, "参数错误")
		return
	}
	if err = pixiu.CoreV1.Role().SetRole(c, rid, menusStruct["menu_ids"]); err != nil {
		r.SetCode(http.StatusBadRequest)
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}
