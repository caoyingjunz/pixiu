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
	"github.com/casbin/casbin/v2"

	"github.com/caoyingjunz/gopixiu/cmd/app/config"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	auth "github.com/caoyingjunz/gopixiu/pkg/db/user"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

type AuthenticationInterface interface {
	auth.AuthenticationInterface
}

type AuthenticationGetter interface {
	Authentication() AuthenticationInterface
}

type authentication struct {
	ComponentConfig config.Config
	app             *pixiu
	factory         db.ShareDaoFactory
}

func newAuthentication(c *pixiu) *authentication {
	return &authentication{
		c.cfg,
		c,
		c.factory,
	}
}

// GetEnforce 获取全局enforcer
func (c *authentication) GetEnforce() *casbin.Enforcer {
	return c.factory.Authentication().GetEnforce()
}

// AddRoleForUser 添加用户角色权限
func (c *authentication) AddRoleForUser(userid int64, roleIds []int64) (err error) {
	err = c.factory.Authentication().AddRoleForUser(userid, roleIds)
	if err != nil {
		log.Logger.Error(err)
		return
	}
	return
}

// SetRolePermission 设置角色权限
func (c *authentication) SetRolePermission(roleId int64, menus *[]model.Menu) (bool, error) {
	ok, err := c.factory.Authentication().SetRolePermission(roleId, menus)
	if err != nil {
		log.Logger.Error(err)
		return ok, err
	}
	return ok, err
}

// DeleteRole 删除角色
func (c *authentication) DeleteRole(roleId int64) error {
	err := c.factory.Authentication().DeleteRole(roleId)
	if err != nil {
		log.Logger.Error(err)
		return err
	}
	return err
}

// DeleteRolePermission 删除角色权限
func (c *authentication) DeleteRolePermission(resource ...string) error {
	err := c.factory.Authentication().DeleteRolePermission(resource...)
	if err != nil {
		log.Logger.Error(err)
		return err
	}
	return err
}
