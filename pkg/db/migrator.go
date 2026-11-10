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
	"fmt"

	"github.com/caoyingjunz/pixiu/pkg/db/model"

	"gorm.io/gorm"
)

const (
	defaultTableOptions = "AUTO_INCREMENT=20220801 DEFAULT CHARSET=utf8"
	// upgradeTableOptions 升级状态表固定自增起点为 20261108（2026-11-08）。
	upgradeTableOptions = "AUTO_INCREMENT=20261108 DEFAULT CHARSET=utf8"
)

// tableOptionsFor 返回模型对应的建表参数：升级状态表固定 20261108，其余表沿用全库 20220801。
func tableOptionsFor(d interface{}) string {
	if _, ok := d.(*model.UpgradeRecord); ok {
		return upgradeTableOptions
	}
	return defaultTableOptions
}

type migrator struct {
	db *gorm.DB
}

// AutoMigrate 自动创建/更新指定模型的数据库表结构（补齐新增字段）
func (m *migrator) AutoMigrate() error {
	// 先迁移 upgrade_records 历史列，避免 AutoMigrate 新增列/唯一索引后再回填数据。
	if err := m.migrateUpgradeColumns(m.db); err != nil {
		return err
	}
	for _, d := range model.GetMigrationModels() {
		if err := m.db.Set("gorm:table_options", tableOptionsFor(d)).AutoMigrate(d); err != nil {
			return err
		}
	}
	return m.migrateAPIGroupColumn(m.db)
}

// upgradeColumnRenames 记录 upgrade_records 历史列到当前列的映射。
// 旧版本使用 latest_version/latest_commit_id/last_error，新版本统一为 version_tag/commit_id/error_message。
var upgradeColumnRenames = []struct {
	old string
	new string
}{
	{old: "latest_version", new: "version_tag"},
	{old: "latest_commit_id", new: "commit_id"},
	{old: "last_error", new: "error_message"},
}

// upgradeObsoleteColumns 是 upgrade_records 不再使用、需要从存量表清理的列。
var upgradeObsoleteColumns = []string{
	"last_probed_at",
	"upgrade_available",
	"current_version",
	"last_checked_at",
}

// upgradeColumnAction 描述一次历史列处理动作：rename=true 表示直接改名；
// rename=false 表示新旧列并存，需要先把旧列数据回填到新列，再删除旧列。
type upgradeColumnAction struct {
	rename bool
	old    string
	new    string
}

// planUpgradeColumnActions 根据列存在情况计算历史列处理动作，便于单元测试。
func planUpgradeColumnActions(hasColumn func(string) bool) []upgradeColumnAction {
	actions := make([]upgradeColumnAction, 0, len(upgradeColumnRenames))
	for _, rename := range upgradeColumnRenames {
		hasOld, hasNew := hasColumn(rename.old), hasColumn(rename.new)
		switch {
		case hasOld && hasNew:
			actions = append(actions, upgradeColumnAction{old: rename.old, new: rename.new})
		case hasOld:
			actions = append(actions, upgradeColumnAction{rename: true, old: rename.old, new: rename.new})
		}
	}
	return actions
}

// migrateUpgradeColumns 处理 upgrade_records 的历史列，保证存量库升级后与当前模型一致。
// 必须在 AutoMigrate 之前执行：先回填/改名旧数据，再让 AutoMigrate 基于正确数据补列与建索引。
func (m *migrator) migrateUpgradeColumns(db *gorm.DB) error {
	upgrade := &model.UpgradeRecord{}
	migrator := db.Migrator()

	for _, action := range planUpgradeColumnActions(func(column string) bool {
		return migrator.HasColumn(upgrade, column)
	}) {
		if action.rename {
			if err := migrator.RenameColumn(upgrade, action.old, action.new); err != nil {
				return fmt.Errorf("重命名 upgrade_records.%s 为 %s 失败: %w", action.old, action.new, err)
			}
			continue
		}

		// 新列已存在（可能由旧版 AutoMigrate 或手工迁移创建），仅在目标列为空时回填旧值。
		query := fmt.Sprintf(
			"UPDATE upgrade_records SET %s = %s WHERE (%s IS NULL OR %s = '') AND %s IS NOT NULL AND %s != ''",
			action.new, action.old, action.new, action.new, action.old, action.old,
		)
		if err := db.Exec(query).Error; err != nil {
			return fmt.Errorf("回填 upgrade_records.%s 到 %s 失败: %w", action.old, action.new, err)
		}
		if err := migrator.DropColumn(upgrade, action.old); err != nil {
			return fmt.Errorf("删除 upgrade_records.%s 旧列失败: %w", action.old, err)
		}
	}

	for _, column := range upgradeObsoleteColumns {
		if migrator.HasColumn(upgrade, column) {
			if err := migrator.DropColumn(upgrade, column); err != nil {
				return fmt.Errorf("删除 upgrade_records.%s 废弃列失败: %w", column, err)
			}
		}
	}
	return nil
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
