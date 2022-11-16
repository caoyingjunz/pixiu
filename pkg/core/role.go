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

type RoleGetter interface {
	Role() RoleInterface
}

// RoleInterface 角色操作接口
type RoleInterface interface {
	Create(c context.Context, obj *types.RoleReq) (role *model.Role, err error)
	Update(c context.Context, role *types.UpdateRoleReq, rid int64) error
	Delete(c context.Context, rId int64) error
	Get(c context.Context, rid int64) (roles *[]model.Role, err error)
	List(c context.Context, page, limit int) (res *model.PageRole, err error)

	GetMenusByRoleID(c context.Context, rid int64) (*[]model.Menu, error)
	SetRole(ctx context.Context, roleId int64, menuIds []int64) error
	GetRolesByMenuID(ctx context.Context, menuId int64) (roleIds *[]int64, err error)
	GetRoleByRoleName(ctx context.Context, roleName string) (*model.Role, error)
	CheckRoleIsExist(ctx context.Context, name string) bool
	UpdateStatus(c context.Context, roleId, status int64) error
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

func (r *role) Create(c context.Context, obj *types.RoleReq) (role *model.Role, err error) {
	if role, err = r.factory.Role().Create(c, &model.Role{
		Name:     obj.Name,
		Memo:     obj.Memo,
		ParentID: obj.ParentID,
		Sequence: obj.Sequence,
		Status:   obj.Status,
	}); err != nil {
		log.Logger.Error(err)
		return
	}
	return
}

func (r *role) Update(c context.Context, role *types.UpdateRoleReq, rid int64) error {
	if err := r.factory.Role().Update(c, role, rid); err != nil {
		log.Logger.Error(err)
		return err
	}
	return nil
}

func (r *role) Delete(c context.Context, rId int64) error {
	// 1.先清除rule
	err := r.factory.Authentication().DeleteRole(c, rId)
	if err != nil {
		log.Logger.Error(err)
		return err
	}

	// 2.删除user_role
	err = r.factory.Role().Delete(c, rId)
	if err != nil {
		log.Logger.Error(err)
		return err
	}
	return nil
}

func (r *role) Get(c context.Context, rid int64) (roles *[]model.Role, err error) {
	if roles, err = r.factory.Role().Get(c, rid); err != nil {
		log.Logger.Error(err)
		return
	}
	return
}

func (r *role) List(c context.Context, page, limit int) (res *model.PageRole, err error) {
	if res, err = r.factory.Role().List(c, page, limit); err != nil {
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
	// 查询menus信息
	menus, err := r.factory.Menu().GetByIds(ctx, menuIds)
	if err != nil {
		log.Logger.Error(err)
		return err
	}

	// 添加rule规则
	ok, err := r.factory.Authentication().SetRolePermission(ctx, roleId, menus)
	if !ok || err != nil {
		log.Logger.Error(err)
		return err
	}

	// 配置role_menus, 如果操作失败，则将rule表中规则清除
	if err = r.factory.Role().SetRole(ctx, roleId, menuIds); err != nil {
		log.Logger.Error(err)
		//清除rule表中规则
		for _, menu := range *menus {
			err := r.factory.Authentication().DeleteRolePermissionWithRole(ctx, roleId, menu.URL, menu.Method)
			if err != nil {
				log.Logger.Error(err)
				break
			}
		}
		return err
	}

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

func (r *role) GetRoleByRoleName(ctx context.Context, roleName string) (role *model.Role, err error) {
	role, err = r.factory.Role().GetRoleByRoleName(ctx, roleName)
	if err != nil {
		log.Logger.Error(err)
		return
	}
	return
}

func (r *role) UpdateStatus(c context.Context, roleId, status int64) error {
	return r.factory.Role().UpdateStatus(c, roleId, status)
}

// CheckRoleIsExist 判断角色是否存在
func (r *role) CheckRoleIsExist(ctx context.Context, name string) bool {
	_, err := r.factory.Role().GetRoleByRoleName(ctx, name)
	if err != nil {
		log.Logger.Error(err)
		return false
	}

	return true
}
