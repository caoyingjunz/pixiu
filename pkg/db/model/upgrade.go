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
	// UpgradeStatusFailed 升级失败：upgrade 进程拉取/重建失败后写入，允许用户重试（4 -> 1）。
	UpgradeStatusFailed = 4
)

// UpgradeRecord 系统升级记录
type UpgradeRecord struct {
	pixiu.Model

	VersionTag   string     `gorm:"column:version_tag;type:varchar(64);default:'';uniqueIndex:uk_upgrade_version_commit,priority:1" json:"version_tag"`
	CommitID     string     `gorm:"column:commit_id;type:varchar(64);default:'';uniqueIndex:uk_upgrade_version_commit,priority:2" json:"commit_id"`
	Image        string     `gorm:"type:varchar(512);default:''" json:"image"`
	IsCurrent    bool       `gorm:"column:is_current;default:false;not null" json:"is_current"`
	Status       int        `gorm:"type:tinyint;default:0;not null" json:"status"`
	UpgradeID    string     `gorm:"column:upgrade_id;type:varchar(128);default:''" json:"upgrade_id"`
	ReleaseTime  *time.Time `gorm:"column:release_time" json:"release_time,omitempty"`
	ErrorMessage string     `gorm:"column:error_message;type:text" json:"error_message"`
}

func (u *UpgradeRecord) TableName() string {
	return "upgrade_records"
}
