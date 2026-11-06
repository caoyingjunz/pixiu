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

package types

import "time"

// UpgradeStatus 系统升级状态（由 pixiu-upgrade 同步到数据库）。
type UpgradeStatus struct {
	CurrentVersion   string `json:"current_version"`
	LatestVersion    string `json:"latest_version"`
	Image            string `json:"image,omitempty"`
	UpgradeAvailable bool   `json:"upgrade_available"`
	// Status 升级状态机：0 未升级 / 1 待升级 / 2 升级中 / 3 升级完成 / 4 升级失败
	Status        int        `json:"status"`
	Upgrading     bool       `json:"upgrading"`
	LastError     string     `json:"last_error,omitempty"`
	LastCheckedAt *time.Time `json:"last_checked_at,omitempty"`
}
