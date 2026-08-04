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
	"strings"

	"github.com/caoyingjunz/pixiu/pkg/db/model"

	"gorm.io/gorm"
)

type migrator struct {
	db *gorm.DB
}

// AutoMigrate 自动创建指定模型的数据库表结构。
//
// 仅当表不存在时 CreateTable，已存在的表跳过。
// 不能调用 db.AutoMigrate：gorm 会通过 ReorderModels 连带处理关联的已存在表，
// 进而执行 ColumnTypes；其 SQL 依赖 information_schema.columns.udt_name，
// 在人大金仓 Oracle 兼容模式下该列不存在，会报：
// "column c.udt_name does not exist (SQLSTATE 42703)"。
// 此行为与「不会更新已存在的数据库表」的既有语义一致，后续模型新增字段需人工迁移。
func (m *migrator) AutoMigrate() error {
	mig := m.db.Migrator()
	for _, d := range model.GetMigrationModels() {
		if mig.HasTable(d) {
			continue
		}
		if err := mig.CreateTable(d); err != nil {
			return err
		}
	}

	return m.migrateAPIGroupColumn(m.db)
}

// migrateAPIGroupColumn 将历史保留字段 group 迁移到 api_group，避免保留字导致读写异常。
// 金仓环境下若列探测失败则跳过（不影响主流程）。
func (m *migrator) migrateAPIGroupColumn(db *gorm.DB) error {
	api := &model.API{}
	mig := db.Migrator()

	hasLegacyGroup := mig.HasColumn(api, "group")
	hasAPIGroup := mig.HasColumn(api, "api_group")

	if hasLegacyGroup && hasAPIGroup {
		if err := db.Exec(
			`UPDATE apis SET api_group = "group" WHERE ("group" IS NOT NULL AND "group" != '') AND (api_group IS NULL OR api_group = '')`,
		).Error; err != nil {
			if isKingbaseUdtNameError(err) {
				return nil
			}
			return err
		}
		if err := mig.DropColumn(api, "group"); err != nil && !isKingbaseUdtNameError(err) {
			return err
		}
		return nil
	}

	if hasLegacyGroup && !hasAPIGroup {
		if err := mig.RenameColumn(api, "group", "api_group"); err != nil && !isKingbaseUdtNameError(err) {
			return err
		}
	}

	return nil
}

func isKingbaseUdtNameError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "udt_name") && strings.Contains(msg, "42703")
}

func newMigrator(db *gorm.DB) *migrator {
	return &migrator{db}
}
