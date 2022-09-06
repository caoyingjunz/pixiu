package user

import (
	"context"

	"github.com/gin-gonic/gin"

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
		log.Logger.Errorf(err.Error())
		httputils.SetFailed(c, r, err)
		return
	}
	if _, err := pixiu.CoreV1.Menu().Create(c, &menu); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (*menuRouter) updateMenu(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		menu model.Menu
	)
	if err = c.ShouldBindJSON(&menu); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	menuId, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = pixiu.CoreV1.Menu().Update(c, &menu, menuId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (*menuRouter) deleteMenu(c *gin.Context) {
	r := httputils.NewResponse()
	mid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := pixiu.CoreV1.Menu().Delete(c, mid); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (*menuRouter) getMenu(c *gin.Context) {
	r := httputils.NewResponse()
	mid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Menu().Get(c, mid)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (*menuRouter) listMenus(c *gin.Context) {
	r := httputils.NewResponse()
	var err error
	if r.Result, err = pixiu.CoreV1.Menu().List(context.TODO()); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)

}
