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

func (*menuRouter) addMenu(c *gin.Context) {
	r := httputils.NewResponse()
	var menu model.Menu
	if err := c.ShouldBindJSON(&menu); err != nil {
		// 系统记录异常日志
		log.Logger.Errorf(err.Error())
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	if _, err := pixiu.CoreV1.Menu().Create(c, &menu); err != nil {
		httputils.SetFailed(c, r, httpstatus.OperateFailed)
		return
	}
	httputils.SetSuccess(c, r)
}

func (*menuRouter) updateMenu(c *gin.Context) {
	r := httputils.NewResponse()
	var menu model.Menu

	if err := c.ShouldBindJSON(&menu); err != nil {
		log.Logger.Error(err)
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	menuId, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		log.Logger.Error(err)
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	if err = pixiu.CoreV1.Menu().Update(c, &menu, menuId); err != nil {
		httputils.SetFailed(c, r, httpstatus.OperateFailed)
		return
	}

	httputils.SetSuccess(c, r)
}

func (*menuRouter) deleteMenu(c *gin.Context) {
	r := httputils.NewResponse()
	mid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		log.Logger.Error(err)
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	if err := pixiu.CoreV1.Menu().Delete(c, mid); err != nil {
		httputils.SetFailed(c, r, httpstatus.OperateFailed)
		return
	}

	httputils.SetSuccess(c, r)
}

func (*menuRouter) getMenu(c *gin.Context) {
	r := httputils.NewResponse()
	mid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		log.Logger.Error(err)
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	r.Result, err = pixiu.CoreV1.Menu().Get(c, mid)
	if err != nil {
		httputils.SetFailed(c, r, httpstatus.OperateFailed)
		return
	}
	httputils.SetSuccess(c, r)
}

func (*menuRouter) listMenus(c *gin.Context) {
	r := httputils.NewResponse()
	res, err := pixiu.CoreV1.Menu().List(context.TODO())
	if err != nil {
		httputils.SetFailed(c, r, httpstatus.OperateFailed)
		return
	}
	r.Result = res

	httputils.SetSuccess(c, r)

}
