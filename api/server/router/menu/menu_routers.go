/*
Copyright 2021 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package menu

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/gopixiu/api/server/httpstatus"
	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
	"github.com/caoyingjunz/gopixiu/pkg/util"
)

func (*menuRouter) addMenu(c *gin.Context) {
	r := httputils.NewResponse()
	var menu model.Menu
	if err := c.ShouldBindJSON(&menu); err != nil {
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
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	menuId, err := util.ParseInt64(c.Param("id"))
	if err != nil {
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
		httputils.SetFailed(c, r, httpstatus.ParamsError)
		return
	}
	if err = pixiu.CoreV1.Menu().Delete(c, mid); err != nil {
		httputils.SetFailed(c, r, httpstatus.OperateFailed)
		return
	}

	httputils.SetSuccess(c, r)
}

func (*menuRouter) getMenu(c *gin.Context) {
	r := httputils.NewResponse()
	mid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
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
