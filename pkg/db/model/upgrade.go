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

package model

import (
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"
)

func init() {
	register(&UpgradeRecord{})
}

// 系统升级状态机（写入方见各常量说明）。
const (
	// UpgradeStatusNotUpgraded 未升级（默认）：检查器发现 latest > current 时写入，等待用户确认。
	UpgradeStatusNotUpgraded = 0
	// UpgradeStatusPending 待升级：用户确认升级后写入，等待 upgrade 进程认领。
	UpgradeStatusPending = 1
	// UpgradeStatusUpgrading 升级中：upgrade 进程原子认领后写入，并记录执行者 upgrade_id。
	UpgradeStatusUpgrading = 2
	// UpgradeStatusCompleted 升级完成：新版 pixiu 启动后确认运行版本与目标版本一致时写入。
	UpgradeStatusCompleted = 3
	// UpgradeStatusFailed 升级失败：upgrade 进程拉取/重建/健康检查失败（含回滚）后写入，允许用户重试（4 -> 1）。
	UpgradeStatusFailed = 4
)

// UpgradeRecord 系统升级状态（单行记录，Id 固定为 1）。
// 状态迁移全部由条件 UPDATE 完成，禁止整行覆盖写入。
type UpgradeRecord struct {
	pixiu.Model

	CurrentVersion   string     `gorm:"type:varchar(64);default:''" json:"current_version"`
	LatestVersion    string     `gorm:"type:varchar(64);default:''" json:"latest_version"`
	Image            string     `gorm:"type:varchar(512);default:''" json:"image"`
	UpgradeAvailable bool       `gorm:"default:false" json:"upgrade_available"`
	Status           int        `gorm:"type:tinyint;default:0;not null" json:"status"`
	UpgradeID        string     `gorm:"column:upgrade_id;type:varchar(128);default:''" json:"upgrade_id"`
	LastError        string     `gorm:"type:text" json:"last_error"`
	LastCheckedAt    *time.Time `json:"last_checked_at,omitempty"`
}

func (u *UpgradeRecord) TableName() string {
	return "upgrade_records"
}
