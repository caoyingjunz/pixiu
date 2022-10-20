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

package user

import (
	"context"
	"strconv"

	"github.com/casbin/casbin/v2"
	csmodel "github.com/casbin/casbin/v2/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"

	"github.com/caoyingjunz/gopixiu/pkg/db/model"
)

// TODO: 整体优化

var Policy *casbin.Enforcer

type AuthenticationInterface interface {
	GetEnforce() *casbin.Enforcer
	AddRoleForUser(ctx context.Context, userid int64, roleIds []int64) (err error)
	SetRolePermission(ctx context.Context, roleId int64, menus *[]model.Menu) (bool, error)
	DeleteRole(ctx context.Context, roleId int64) error
	DeleteRolePermission(ctx context.Context, resource ...string) error
	DeleteRoleWithUser(ctx context.Context, uid, roleId int64) error
	DeleteRolePermissionWithRole(ctx context.Context, roleId int64, resource ...string) error
}

type authentication struct {
	db       *gorm.DB
	enforcer *casbin.Enforcer
}

func NewAuthentication(db *gorm.DB) *authentication {
	return &authentication{db, Policy}
}

func (c *authentication) GetEnforce() *casbin.Enforcer {
	return c.enforcer
}

// AddRoleForUser 分配用户角色
func (c *authentication) AddRoleForUser(ctx context.Context, userid int64, roleIds []int64) (err error) {
	uidStr := strconv.FormatInt(userid, 10)
	_, err = c.enforcer.DeleteRolesForUser(uidStr)
	if err != nil {
		return
	}
	for _, roleId := range roleIds {
		ok, err := c.enforcer.AddRoleForUser(uidStr, strconv.FormatInt(roleId, 10))
		if err != nil || !ok {
			break
		}
	}
	return
}

// SetRolePermission 设置角色权限
func (c *authentication) SetRolePermission(ctx context.Context, roleId int64, menus *[]model.Menu) (bool, error) {
	_, err := c.enforcer.DeletePermissionsForUser(strconv.FormatInt(roleId, 10))
	if err != nil {
		return false, err
	}
	_, err = c.setRolePermission(roleId, menus)
	if err != nil {
		return false, err
	}
	return true, nil
}

// 设置角色权限
func (c *authentication) setRolePermission(roleId int64, menus *[]model.Menu) (bool, error) {
	for _, menu := range *menus {
		if menu.MenuType == 2 || menu.MenuType == 3 {
			ok, err := c.enforcer.AddPermissionForUser(strconv.FormatInt(roleId, 10), menu.URL, menu.Method)
			if !ok || err != nil {
				return ok, err
			}
		}
	}
	return false, nil
}

// DeleteRole 删除角色
func (c *authentication) DeleteRole(ctx context.Context, roleId int64) error {

	ok, err := c.enforcer.DeletePermissionsForUser(strconv.FormatInt(roleId, 10))
	if err != nil || !ok {
		return err
	}
	_, err = c.enforcer.DeleteRole(strconv.FormatInt(roleId, 10))
	if err != nil {
		return err
	}

	return nil
}

// DeleteRoleWithUser 删除角色
func (c *authentication) DeleteRoleWithUser(ctx context.Context, uid, roleId int64) error {
	ok, err := c.enforcer.DeleteRoleForUser(strconv.FormatInt(uid, 10), strconv.FormatInt(roleId, 10))
	if err != nil || !ok {
		return err
	}

	return nil
}

// DeleteRolePermission 删除角色权限
func (c *authentication) DeleteRolePermission(ctx context.Context, resource ...string) error {
	_, err := c.enforcer.DeletePermission(resource...)
	if err != nil {
		return err
	}
	return nil
}

// DeleteRolePermissionWithRole 删除角色的权限
func (c *authentication) DeleteRolePermissionWithRole(ctx context.Context, roleId int64, resource ...string) error {
	_, err := c.enforcer.DeletePermissionForUser(strconv.FormatInt(roleId, 10), resource...)
	if err != nil {
		return err
	}
	return nil
}

// InitPolicyEnforcer TODO: 整体优化
func InitPolicyEnforcer(db *gorm.DB) (err error) {
	rbacRules :=
		`
	[request_definition]
	r = sub, obj, act
	
	[policy_definition]
	p = sub, obj, act
	
	[role_definition]
	g = _, _

	[policy_effect]
	e = some(where (p.eft == allow))
	
	[matchers]
	m = g(r.sub, p.sub) && keyMatch2(r.obj, p.obj) && regexMatch(r.act, p.act) || r.sub == "21220821"
	`
	// 加载鉴权规则
	m, err := csmodel.NewModelFromString(rbacRules)
	if err != nil {
		return
	}
	// 调用gorm创建casbin_rule表
	adapter, err := gormadapter.NewAdapterByDBWithCustomTable(db, &model.Rule{}, "rules")
	if err != nil {
		return
	}
	// 创建鉴权器enforcer（使用gorm adapter）
	enforcer, err := casbin.NewEnforcer(m, adapter)
	if err != nil {
		return
	}
	// 	加载权限
	err = enforcer.LoadPolicy()
	if err != nil {
		return
	}

	Policy = enforcer
	return
}
