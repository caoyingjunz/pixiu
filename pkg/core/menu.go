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

package core

import (
	"context"

	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

type MenuGetter interface {
	Menu() MenuInterface
}

// MenuInterface 菜单操作接口
type MenuInterface interface {
	Create(c context.Context, obj *types.MenusReq) (menu *model.Menu, err error)
	Update(c context.Context, menu *types.UpdateMenusReq, mId int64) error
	Delete(c context.Context, mId int64) error
	Get(c context.Context, mId int64) (menu *model.Menu, err error)
	List(c context.Context, page, limit int, menuType []int8) (res *model.PageMenu, err error)

	GetByIds(c context.Context, mIds []int64) (menus *[]model.Menu, err error)
	GetMenuByMenuNameUrl(context.Context, string, string) (*model.Menu, error)
	CheckMenusIsExist(c context.Context, menuId int64) bool
	UpdateStatus(c context.Context, menuId, status int64) error
}

type menu struct {
	app     *pixiu
	factory db.ShareDaoFactory
}

func newMenu(c *pixiu) *menu {
	return &menu{
		app:     c,
		factory: c.factory,
	}
}

func (m *menu) Create(c context.Context, obj *types.MenusReq) (menu *model.Menu, err error) {
	if menu, err = m.factory.Menu().Create(c, &model.Menu{
		Name:     obj.Name,
		Memo:     obj.Memo,
		ParentID: obj.ParentID,
		Status:   obj.Status,
		URL:      obj.URL,
		Icon:     obj.Icon,
		Sequence: obj.Sequence,
		MenuType: obj.MenuType,
		Method:   obj.Method,
		Code:     obj.Code,
	}); err != nil {
		log.Logger.Error(err)
		return
	}
	return
}

func (m *menu) Update(c context.Context, menu *types.UpdateMenusReq, mId int64) error {
	err := m.factory.Menu().Update(c, menu, mId)
	if err != nil {
		log.Logger.Error(err)
		return err
	}
	return nil
}

func (m *menu) Delete(c context.Context, mId int64) error {
	menuInfo, err := m.factory.Menu().Get(c, mId)
	// 如果报错或者未获取到menu信息则返回
	if err != nil || menuInfo == nil {
		log.Logger.Error(err)
		return err
	}

	// 清除rules
	err = m.factory.Authentication().DeleteRolePermission(c, menuInfo.URL, menuInfo.Method)
	if err != nil {
		log.Logger.Error(err)
		return err
	}

	// 清除menus
	err = m.factory.Menu().Delete(c, mId)
	if err != nil {
		log.Logger.Error(err)
		return err
	}

	return nil
}

func (m *menu) Get(c context.Context, mId int64) (menu *model.Menu, err error) {
	if menu, err = m.factory.Menu().Get(c, mId); err != nil {
		log.Logger.Error(err)
		return nil, err
	}
	return
}

func (m *menu) List(c context.Context, page, limit int, menuType []int8) (res *model.PageMenu, err error) {
	if res, err = m.factory.Menu().List(c, page, limit, menuType); err != nil {
		log.Logger.Error(err)
		return
	}
	return
}

func (m *menu) GetByIds(c context.Context, mIds []int64) (menus *[]model.Menu, err error) {
	menus, err = m.factory.Menu().GetByIds(c, mIds)
	if err != nil {
		log.Logger.Error(err)
		return
	}
	return
}

func (m *menu) GetMenuByMenuNameUrl(c context.Context, url, method string) (menu *model.Menu, err error) {
	menu, err = m.factory.Menu().GetMenuByMenuNameUrl(c, url, method)
	return
}

func (m *menu) CheckMenusIsExist(c context.Context, menuId int64) bool {
	_, err := m.factory.Menu().Get(c, menuId)
	if err != nil {
		log.Logger.Error(err)
		return false
	}
	return true
}

func (m *menu) UpdateStatus(c context.Context, menuId, status int64) error {
	return m.factory.Menu().UpdateStatus(c, menuId, status)
}
