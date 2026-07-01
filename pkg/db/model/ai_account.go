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
	register(&AIAccount{})
}

// AIAccount stores API credentials for a local user.
type AIAccount struct {
	pixiu.Model

	UserId      int64  `gorm:"not null;index:idx_ai_accounts_user_id;uniqueIndex:uk_ai_accounts_user_provider" json:"user_id"`
	Provider    string `gorm:"type:varchar(64);not null;uniqueIndex:uk_ai_accounts_user_provider" json:"provider"`
	APIKey      string `gorm:"column:api_key;type:text;not null" json:"api_key"`
	BaseURL     string `gorm:"column:base_url;type:varchar(512)" json:"base_url"`
	ModelName   string `gorm:"column:model;type:varchar(128)" json:"model"`
	Description string `gorm:"type:text" json:"description"`
	Enabled     bool   `gorm:"not null;default:true" json:"enabled"`
}

func (AIAccount) TableName() string {
	return "ai_accounts"
}
