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

// AutoMigrate 自动创建/更新指定模型的数据库表结构（补齐新增字段）
func (m *migrator) AutoMigrate() error {
	db := m.db.Set("gorm:table_options", "AUTO_INCREMENT=20220801 DEFAULT CHARSET=utf8")

	for _, d := range model.GetMigrationModels() {
		if err := db.AutoMigrate(d); err != nil {
			return err
		}
	}

	return m.migrateAPIGroupColumn(db)
}

// migrateAPIGroupColumn 将历史保留字段 group 迁移到 api_group，避免 MySQL 保留字导致读写异常
func (m *migrator) migrateAPIGroupColumn(db *gorm.DB) error {
	api := &model.API{}
	migrator := db.Migrator()

	hasLegacyGroup := migrator.HasColumn(api, "group")
	hasAPIGroup := migrator.HasColumn(api, "api_group")

	if hasLegacyGroup && hasAPIGroup {
		if err := db.Exec(
			"UPDATE apis SET api_group = `group` WHERE (`group` IS NOT NULL AND `group` != '') AND (api_group IS NULL OR api_group = '')",
		).Error; err != nil {
			return err
		}
		return migrator.DropColumn(api, "group")
	}

	if hasLegacyGroup && !hasAPIGroup {
		return migrator.RenameColumn(api, "group", "api_group")
	}

	return nil
}

func newMigrator(db *gorm.DB) *migrator {
	return &migrator{db}
}
