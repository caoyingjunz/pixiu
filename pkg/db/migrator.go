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
	"github.com/caoyingjunz/pixiu/pkg/db/model"

	"gorm.io/gorm"
)

type migrator struct {
	db *gorm.DB
}

// AutoMigrate 自动创建指定模型的数据库表结构
func (m *migrator) AutoMigrate() error {
	return m.CreateTables(model.GetMigrationModels()...)
}

func (m *migrator) CreateTables(dst ...interface{}) error {
	db := m.db.Set("gorm:table_options", "AUTO_INCREMENT=20220801 DEFAULT CHARSET=utf8")

	for _, d := range dst {
		if db.Migrator().HasTable(d) {
			continue
		}
		if err := db.Migrator().CreateTable(d); err != nil {
			return err
		}
	}
	return nil
}

func newMigrator(db *gorm.DB) *migrator {
	return &migrator{db}
}
