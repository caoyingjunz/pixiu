/*
Copyright 2024 The Pixiu Authors.

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

package sqlite

import (
	"fmt"

	sqliteDriver "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db/dbconn"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func NewDb(sqlConfig *config.SqliteOptions, mode string, migrate bool) (*dbconn.DbConn, error) {

	opt := &gorm.Config{}
	if mode == mode {
		opt.Logger = logger.Default.LogMode(logger.Info)
	}

	dsn := fmt.Sprintf("%s?charset=utf8&parseTime=True&loc=Local", sqlConfig.Db)
	DB, err := gorm.Open(sqliteDriver.Open(dsn), opt)
	sqlDB, err := DB.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(types.MaxIdleConns)
	sqlDB.SetMaxOpenConns(types.MaxOpenConns)
	if migrate {
		if err := newMigrator(DB).AutoMigrate(); err != nil {
			return nil, err
		}
	}

	return &dbconn.DbConn{
		Conn: DB,
	}, nil
}
