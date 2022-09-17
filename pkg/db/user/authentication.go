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
	"strconv"

	"github.com/casbin/casbin/v2"
	"gorm.io/gorm"

	"github.com/caoyingjunz/gopixiu/pkg/db/model"
)

type AuthenticationInterface interface {
	GetEnforce() *casbin.Enforcer
	AddRoleForUser(userid int64, roleIds []int64) (err error)
	SetRolePermission(roleId int64, menus *[]model.Menu) (bool, error)
	DeleteRole(roleId int64) error
	DeleteRolePermission(...string) error
}

type authentication struct {
	db       *gorm.DB
	enforcer *casbin.Enforcer
}

func NewAuthentication(db *gorm.DB, enforcer *casbin.Enforcer) *authentication {
	return &authentication{db, enforcer}
}

func (c *authentication) GetEnforce() *casbin.Enforcer {
	return c.enforcer
}

// AddRoleForUser 分配用户角色
func (c *authentication) AddRoleForUser(userid int64, roleIds []int64) (err error) {
	uidStr := strconv.FormatInt(userid, 10)
	c.enforcer.DeleteRolesForUser(uidStr)
	for _, roleId := range roleIds {
		c.enforcer.AddRoleForUser(uidStr, strconv.FormatInt(roleId, 10))
	}
	return
}

// SetRolePermission 设置角色权限
func (c *authentication) SetRolePermission(roleId int64, menus *[]model.Menu) (bool, error) {
	c.enforcer.DeletePermissionsForUser(strconv.FormatInt(roleId, 10))
	c.setRolePermission(roleId, menus)
	return false, nil
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
func (c *authentication) DeleteRole(roleId int64) error {

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

// DeleteRolePermission 删除角色权限
func (c *authentication) DeleteRolePermission(resource ...string) error {
	ok, err := c.enforcer.DeletePermission(resource...)
	if !ok || err != nil {
		return err
	}
	return nil
}
