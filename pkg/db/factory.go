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

	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db/dbconn"
	"github.com/caoyingjunz/pixiu/pkg/db/iface"
	"github.com/caoyingjunz/pixiu/pkg/db/mysql"
	"github.com/caoyingjunz/pixiu/pkg/db/sqlite"
)

type ShareDaoFactory interface {
	Cluster() iface.ClusterInterface
	Tenant() iface.TenantInterface
	User() iface.UserInterface
}

func NewDaoFactory(dbConfig *config.DbConfig, mode string, migrate bool) (ShareDaoFactory, error) {
	var db *dbconn.DbConn
	var err error
	if dbConfig.Mysql != nil{
		db, err = mysql.NewDb(dbConfig.Mysql, mode, migrate)
		if err != nil {
			return nil, err
		}
		return mysql.New(db.Conn.(*gorm.DB))
	}
	if dbConfig.Sqlite != nil{
		db, err = sqlite.NewDb(dbConfig.Sqlite, mode, migrate)
		if err != nil {
			return nil, err
		}
		return sqlite.New(db.Conn.(*gorm.DB))
	}

	return nil, err
}
