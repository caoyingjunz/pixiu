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

// UpgradeRecord 系统升级状态（单行记录，Id 固定为 1）。
type UpgradeRecord struct {
	pixiu.Model

	CurrentVersion   string     `gorm:"type:varchar(64);default:''" json:"current_version"`
	LatestVersion    string     `gorm:"type:varchar(64);default:''" json:"latest_version"`
	Image            string     `gorm:"type:varchar(512);default:''" json:"image"`
	UpgradeAvailable bool       `gorm:"default:false" json:"upgrade_available"`
	Upgrading        bool       `gorm:"default:false" json:"upgrading"`
	Requested        bool       `gorm:"default:false" json:"requested"`
	LastError        string     `gorm:"type:text" json:"last_error"`
	LastCheckedAt    *time.Time `json:"last_checked_at,omitempty"`
}

func (u *UpgradeRecord) TableName() string {
	return "upgrade_records"
}
