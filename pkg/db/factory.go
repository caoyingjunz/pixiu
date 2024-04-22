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
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db/dbconn"
	"github.com/caoyingjunz/pixiu/pkg/db/mysql"
	"github.com/caoyingjunz/pixiu/pkg/db/sqlite"
)

type ShareDaoFactory interface {
	Cluster() ClusterInterface
	Tenant() TenantInterface
	User() UserInterface
}

type shareDaoFactory struct {
	db *dbconn.DbConn
}

func (f *shareDaoFactory) Cluster() ClusterInterface {
	return newCluster(f.db)
}
func (f *shareDaoFactory) Tenant() TenantInterface {
	return newTenant(f.db)
}
func (f *shareDaoFactory) User() UserInterface {
	return newUser(f.db)
}

func NewDaoFactory(dbConfig *config.DbConfig, mode string, migrate bool) (ShareDaoFactory, error) {
	var db *dbconn.DbConn
	var err error
	switch dbConfig.Type {
	case "mysql":
		db, err = mysql.NewDbConn(dbConfig.Mysql, mode, migrate)
		if err != nil {
			return nil, err
		}
	case "sqlite":
		db, err = sqlite.NewDbConn(dbConfig.Sqlite, mode, migrate)
	}

	return &shareDaoFactory{
		db: db,
	}, err
}
