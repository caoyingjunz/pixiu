/*
Copyright 2026 The Pixiu Authors.

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

	"gorm.io/gorm"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

// upgradeRequiredColumns 是当前代码读写 server_versions 所依赖的列。
var upgradeRequiredColumns = []string{
	"version_tag",
	"commit_id",
	"image",
	"is_current",
	"status",
	"upgrade_id",
	"release_time",
	"error_message",
}

// ValidateUpgradeSchema 校验 server_versions 是否具备当前版本所需的列。
// pixiu-upgrade 不执行 AutoMigrate，启动时显式校验并快速失败，
// 避免每轮轮询都写入失败、只留下重复错误日志。
func ValidateUpgradeSchema(db *gorm.DB) error {
	upgrade := &model.UpgradeRecord{}
	migrator := db.Migrator()

	var missing []string
	for _, column := range upgradeRequiredColumns {
		if !migrator.HasColumn(upgrade, column) {
			missing = append(missing, column)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("server_versions 缺少列 %v；请先启动 pixiu 主服务完成 AutoMigrate 建表", missing)
	}
	return nil
}
