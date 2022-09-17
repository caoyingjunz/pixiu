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

	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

type RoleGetter interface {
	Role() RoleInterface
}

// RoleInterface 角色操作接口
type RoleInterface interface {
	Create(c context.Context, obj *model.Role) (role *model.Role, err error)
	Update(c context.Context, role *model.Role, rid int64) error
	Delete(c context.Context, rId int64) error
	Get(c context.Context, rid int64) (roles *[]model.Role, err error)
	List(c context.Context) (roles *[]model.Role, err error)

	GetMenusByRoleID(c context.Context, rid int64) (*[]model.Menu, error)
	SetRole(ctx context.Context, roleId int64, menuIds []int64) error
	GetRolesByMenuID(ctx context.Context, menuId int64) (roleIds *[]int64, err error)
}

type role struct {
	app     *pixiu
	factory db.ShareDaoFactory
}

func newRole(c *pixiu) *role {
	return &role{
		app:     c,
		factory: c.factory,
	}
}
func (r *role) Create(c context.Context, obj *model.Role) (role *model.Role, err error) {
	if role, err = r.factory.Role().Create(c, obj); err != nil {
		log.Logger.Error(err)
		return
	}
	return
}

func (r *role) Update(c context.Context, role *model.Role, rid int64) error {
	if err := r.factory.Role().Update(c, role, rid); err != nil {
		log.Logger.Error(err)
		return err
	}
	return nil
}

func (r *role) Delete(c context.Context, rId int64) error {
	err := r.factory.Role().Delete(c, rId)
	if err != nil {
		log.Logger.Error(err)
		return err
	}
	go r.factory.Authentication().DeleteRole(rId)
	return nil
}

func (r *role) Get(c context.Context, rid int64) (roles *[]model.Role, err error) {
	if roles, err = r.factory.Role().Get(c, rid); err != nil {
		log.Logger.Error(err)
		return
	}
	return
}

func (r *role) List(c context.Context) (roles *[]model.Role, err error) {
	if roles, err = r.factory.Role().List(c); err != nil {
		log.Logger.Error(err)
		return
	}
	return
}
func (r *role) GetMenusByRoleID(c context.Context, rid int64) (*[]model.Menu, error) {
	menus, err := r.factory.Role().GetMenusByRoleID(c, rid)
	if err != nil {
		log.Logger.Error(err)
		return menus, err
	}
	return menus, err
}

// SetRole 设置角色菜单权限
func (r *role) SetRole(ctx context.Context, roleId int64, menuIds []int64) error {
	if err := r.factory.Role().SetRole(ctx, roleId, menuIds); err != nil {
		log.Logger.Error(err)
		return err
	}
	menus, err := r.factory.Menu().GetByIds(ctx, menuIds)
	if err != nil {
		log.Logger.Error(err)
		return err
	}
	go r.factory.Authentication().SetRolePermission(roleId, menus)
	return nil
}

func (r *role) GetRolesByMenuID(ctx context.Context, menuId int64) (roleIds *[]int64, err error) {
	roleIds, err = r.factory.Role().GetRolesByMenuID(ctx, menuId)
	if err != nil {
		log.Logger.Error(err)
		return
	}
	return
}
