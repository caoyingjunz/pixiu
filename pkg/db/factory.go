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

package db

import (
	"gorm.io/gorm"

	"github.com/caoyingjunz/gopixiu/pkg/db/audit"
	"github.com/caoyingjunz/gopixiu/pkg/db/cloud"
	"github.com/caoyingjunz/gopixiu/pkg/db/user"
)

type ShareDaoFactory interface {
	User() user.UserInterface
	Cloud() cloud.CloudInterface
	KubeConfig() cloud.KubeConfigInterface
	Role() user.RoleInterface
	Menu() user.MenuInterface
	Audit() audit.Interface
	Authentication() user.AuthenticationInterface
}

type shareDaoFactory struct {
	db *gorm.DB
}

func (f *shareDaoFactory) Cloud() cloud.CloudInterface           { return cloud.NewCloud(f.db) }
func (f *shareDaoFactory) KubeConfig() cloud.KubeConfigInterface { return cloud.NewKubeConfig(f.db) }
func (f *shareDaoFactory) User() user.UserInterface              { return user.NewUser(f.db) }
func (f *shareDaoFactory) Role() user.RoleInterface              { return user.NewRole(f.db) }
func (f *shareDaoFactory) Menu() user.MenuInterface              { return user.NewMenu(f.db) }
func (f *shareDaoFactory) Audit() audit.Interface                { return audit.NewAudit(f.db) }

// Authentication TODO：优化
func (f *shareDaoFactory) Authentication() user.AuthenticationInterface {
	return user.NewAuthentication(f.db)
}

func NewDaoFactory(db *gorm.DB) ShareDaoFactory {
	return &shareDaoFactory{
		db: db,
	}
}
