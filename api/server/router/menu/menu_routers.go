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
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/errors"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
	"github.com/caoyingjunz/gopixiu/pkg/util"
)

// @Summary      Add a menus
// @Description  Add a menus
// @Tags         menus
// @Accept       json
// @Produce      json
// @Param        data body types.MenusReq true "menu info"
// @Success      200  {object}  httputils.HttpOK
// @Failure      400  {object}  httputils.HttpError
// @Router       /menus [get]
func (*menuRouter) addMenu(c *gin.Context) {
	r := httputils.NewResponse()
	var menu types.MenusReq
	if err := c.ShouldBindJSON(&menu); err != nil {
		httputils.SetFailed(c, r, errors.ParamsError)
		return
	}
	// 判断权限是否已存在
	_, err := pixiu.CoreV1.Menu().GetMenuByMenuNameUrl(c, menu.URL, menu.Method)
	if !errors.IsNotFound(err) {
		httputils.SetFailed(c, r, errors.MenusExistError)
		return
	}

	if _, err := pixiu.CoreV1.Menu().Create(c, &menu); err != nil {
		httputils.SetFailed(c, r, errors.OperateFailed)
		return
	}
	httputils.SetSuccess(c, r)
}

// @Summary      Update a menu by menu id
// @Description  Update a menu by menu id
// @Tags         menus
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "menu ID"  Format(int64)
// @Param        data body types.UpdateMenusReq true "menu info"
// @Success      200  {object}  httputils.HttpOK
// @Failure      400  {object}  httputils.HttpError
// @Router       /menus/{id} [put]
func (*menuRouter) updateMenu(c *gin.Context) {
	r := httputils.NewResponse()
	var menu types.UpdateMenusReq

	if err := c.ShouldBindJSON(&menu); err != nil {
		httputils.SetFailed(c, r, errors.ParamsError)
		return
	}

	menuId, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, errors.ParamsError)
		return
	}

	if !pixiu.CoreV1.Menu().CheckMenusIsExist(c, menuId) {
		httputils.SetFailed(c, r, errors.MenusNtoExistError)
		return
	}

	if err = pixiu.CoreV1.Menu().Update(c, &menu, menuId); err != nil {
		httputils.SetFailed(c, r, errors.OperateFailed)
		return
	}

	httputils.SetSuccess(c, r)
}

// @Summary      Delete menu by menu id
// @Description  Delete menu by menu id
// @Tags         menus
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "menu ID"  Format(int64)
// @Success      200  {object}  httputils.HttpOK
// @Failure      400  {object}  httputils.HttpError
// @Router       /menus/{id} [delete]
func (*menuRouter) deleteMenu(c *gin.Context) {
	r := httputils.NewResponse()
	mid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, errors.ParamsError)
		return
	}

	if !pixiu.CoreV1.Menu().CheckMenusIsExist(c, mid) {
		httputils.SetFailed(c, r, errors.MenusNtoExistError)
		return
	}

	if err = pixiu.CoreV1.Menu().Delete(c, mid); err != nil {
		httputils.SetFailed(c, r, errors.OperateFailed)
		return
	}

	httputils.SetSuccess(c, r)
}

// @Summary      Get menu by menu id
// @Description  Get menu by menu id
// @Tags         menus
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "menu ID"  Format(int64)
// @Success      200  {object}  httputils.Response{result=model.Menu}
// @Failure      400  {object}  httputils.HttpError
// @Router       /menus/{id} [get]
func (*menuRouter) getMenu(c *gin.Context) {
	r := httputils.NewResponse()
	mid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, errors.ParamsError)
		return
	}
	r.Result, err = pixiu.CoreV1.Menu().Get(c, mid)
	if err != nil {
		httputils.SetFailed(c, r, errors.OperateFailed)
		return
	}
	httputils.SetSuccess(c, r)
}

// @Summary      List menus
// @Description  List menus
// @Tags         menus
// @Accept       json
// @Produce      json
// @Param        menu_type   query      []int  false  "menu_type 1: 菜单,2： 按钮, 3：API,可填写多个； 默认为： 1,2,3"
// @Param        page   query      int  false  "pageSize"
// @Param        limit   query      int  false  "page limit"
// @Success      200  {object}  httputils.Response{result=[]model.PageMenu}
// @Failure      400  {object}  httputils.HttpError
// @Router       /menus [get]
func (*menuRouter) listMenus(c *gin.Context) {
	r := httputils.NewResponse()

	var menuType []int8
	// menu_type类型为[int]string
	menuTypeStr := c.DefaultQuery("menu_type", "1,2,3")
	menuTypeSlice := strings.Split(menuTypeStr, ",")
	for _, t := range menuTypeSlice {
		res, err := strconv.Atoi(t)
		if err != nil {
			httputils.SetFailed(c, r, errors.ParamsError)
			return
		}
		menuType = append(menuType, int8(res))
	}

	pageStr := c.DefaultQuery("page", "0")
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		httputils.SetFailed(c, r, errors.ParamsError)
		return
	}

	limitStr := c.DefaultQuery("limit", "0")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		httputils.SetFailed(c, r, errors.ParamsError)
		return
	}

	res, err := pixiu.CoreV1.Menu().List(c, page, limit, menuType)
	if err != nil {
		httputils.SetFailed(c, r, errors.OperateFailed)
		return
	}
	r.Result = res

	httputils.SetSuccess(c, r)
}

// @Summary      Update a menu status by menu id
// @Description  Update a menu status by menu id
// @Tags         menus
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "menu ID"  Format(int64)
// @Param        status   path      int  true  "status "  Format(int64)
// @Success      200  {object}  httputils.HttpOK
// @Failure      400  {object}  httputils.HttpError
// @Router       /menus/{id}/status/{status} [put]
func (*menuRouter) updateMenuStatus(c *gin.Context) {
	r := httputils.NewResponse()

	menuId, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, errors.ParamsError)
		return
	}

	status, err := util.ParseInt64(c.Param("status"))
	if err != nil {
		httputils.SetFailed(c, r, errors.ParamsError)
		return
	}

	if !pixiu.CoreV1.Menu().CheckMenusIsExist(c, menuId) {
		httputils.SetFailed(c, r, errors.MenusNtoExistError)
		return
	}
	if err = pixiu.CoreV1.Menu().UpdateStatus(c, menuId, status); err != nil {
		httputils.SetFailed(c, r, errors.OperateFailed)
		return
	}

	httputils.SetSuccess(c, r)
}
