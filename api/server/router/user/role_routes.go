package user

import (
	"context"
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/gopixiu/api/server/httpstatus"
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
		log.Logger.Error(err)
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	if _, err := pixiu.CoreV1.Role().Create(context.TODO(), &role); err != nil {
		httputils.SetFailed(c, r, httpstatus.OperateFailed)
		return
	}
	httputils.SetSuccess(c, r)
}

func (*roleRouter) updateRole(c *gin.Context) {
	r := httputils.NewResponse()
	var role model.Role
	if err := c.ShouldBindJSON(&role); err != nil {
		log.Logger.Error(err)
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	roleId, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		log.Logger.Error(err)
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	if err = pixiu.CoreV1.Role().Update(context.TODO(), &role, roleId); err != nil {
		httputils.SetFailed(c, r, httpstatus.OperateFailed)
		return
	}

	httputils.SetSuccess(c, r)
}

// 删除前弹窗提示检查该角色是否已经与用户绑定，如果绑定，删除后用户将没有此角色权限
func (*roleRouter) deleteRole(c *gin.Context) {
	r := httputils.NewResponse()
	var roles model.Roles
	err := c.ShouldBindJSON(&roles)
	if err != nil {
		log.Logger.Error(err)
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	if err = pixiu.CoreV1.Role().Delete(c, roles.RoleIds); err != nil {
		httputils.SetFailed(c, r, httpstatus.OperateFailed)
		return
	}

	httputils.SetSuccess(c, r)
}

func (*roleRouter) getRole(c *gin.Context) {
	r := httputils.NewResponse()
	rid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		log.Logger.Error(err)
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	r.Result, err = pixiu.CoreV1.Role().Get(context.TODO(), rid)
	if err != nil {
		httputils.SetFailed(c, r, httpstatus.OperateFailed)
		return
	}

	httputils.SetSuccess(c, r)
}

func (*roleRouter) listRoles(c *gin.Context) {
	r := httputils.NewResponse()
	var err error
	if r.Result, err = pixiu.CoreV1.Role().List(c); err != nil {
		httputils.SetFailed(c, r, httpstatus.OperateFailed)
		return
	}

	httputils.SetSuccess(c, r)

}

func (*roleRouter) getMenusByRole(c *gin.Context) {
	r := httputils.NewResponse()
	rid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		log.Logger.Error(err)
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	if r.Result, err = pixiu.CoreV1.Role().GetMenusByRoleID(c, rid); err != nil {
		httputils.SetFailed(c, r, httpstatus.OperateFailed)
		return
	}
	httputils.SetSuccess(c, r)
}

func (*roleRouter) setRoleMenus(c *gin.Context) {
	r := httputils.NewResponse()
	rid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		log.Logger.Error(err)
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	var menus model.Menus
	if err := c.ShouldBindJSON(&menus); err != nil {
		log.Logger.Error(err)
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	if err = pixiu.CoreV1.Role().SetRole(c, rid, menus.MenuIDS); err != nil {
		httputils.SetFailed(c, r, httpstatus.OperateFailed)
		return
	}
	httputils.SetSuccess(c, r)
}
