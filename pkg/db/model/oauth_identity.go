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

import "github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"

func init() {
	register(&OAuthIdentity{})
}

// OAuthIdentity 将第三方登录标识 (provider, subject_type, subject) 与 Pixiu 用户绑定。
// (provider, subject_type, subject) 联合唯一索引提供数据库级去重，替代仅靠进程内内存锁的
// 单实例约束：多实例并发下同标识建号由 DB 唯一键兜底，避免同一第三方账号绑定到多个用户。
type OAuthIdentity struct {
	pixiu.Model

	UserId      int64  `gorm:"column:user_id;not null;index:idx_oauth_identity_user" json:"user_id"`
	Provider    string `gorm:"column:provider;type:varchar(32);not null;uniqueIndex:uk_oauth_subject,priority:1" json:"provider"`
	SubjectType string `gorm:"column:subject_type;type:varchar(16);not null;uniqueIndex:uk_oauth_subject,priority:2" json:"subject_type"` // union|open
	Subject     string `gorm:"column:subject;type:varchar(128);not null;uniqueIndex:uk_oauth_subject,priority:3" json:"subject"`
}

func (*OAuthIdentity) TableName() string {
	return "oauth_identities"
}
