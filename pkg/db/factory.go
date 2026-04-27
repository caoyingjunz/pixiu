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
)

type ShareDaoFactory interface {
	Cluster() ClusterInterface
	Tenant() TenantInterface
	User() UserInterface
	Plan() PlanInterface
	Audit() AuditInterface
	Repository() RepositoryInterface
}

type shareDaoFactory struct {
	db *gorm.DB
}

func (f *shareDaoFactory) Cluster() ClusterInterface       { return newCluster(f.db) }
func (f *shareDaoFactory) Tenant() TenantInterface         { return newTenant(f.db) }
func (f *shareDaoFactory) User() UserInterface             { return newUser(f.db) }
func (f *shareDaoFactory) Plan() PlanInterface             { return newPlan(f.db) }
func (f *shareDaoFactory) Audit() AuditInterface           { return newAudit(f.db) }
func (f *shareDaoFactory) Repository() RepositoryInterface { return newRepository(f.db) }

func NewDaoFactory(db *gorm.DB, migrate bool) (ShareDaoFactory, error) {
	if migrate {
		// 自动创建指定模型的数据库表结构
		if err := newMigrator(db).AutoMigrate(); err != nil {
			return nil, err
		}
	}

	return &shareDaoFactory{
		db: db,
	}, nil
}
