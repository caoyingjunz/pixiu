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
	// Enabled 是否开启升级功能（upgrade.enabled）
	Enabled    bool   `json:"enabled"`
	VersionTag string `json:"version_tag"`
	IsCurrent  bool   `json:"is_current"`
	CommitID   string `json:"commit_id"`
	Image      string `json:"image,omitempty"`
	// Status 升级状态机：0 未升级 / 1 待升级 / 2 升级中 / 3 升级完成 / 4 升级失败
	Status       int        `json:"status"`
	Upgrading    bool       `json:"upgrading"`
	ErrorMessage string     `json:"error_message,omitempty"`
	ReleaseTime  *time.Time `json:"release_time,omitempty"`
}
