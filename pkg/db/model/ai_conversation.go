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
	register(&AIConversation{})
}

// AIConversation stores persisted response-chain context for one user conversation.
type AIConversation struct {
	pixiu.Model

	UserId             int64  `gorm:"not null;index:idx_ai_conversations_user_id" json:"user_id"`
	AIAccountId        int64  `gorm:"column:ai_account_id;not null;index:idx_ai_conversations_account_id" json:"ai_account_id"`
	Provider           string `gorm:"type:varchar(64);not null" json:"provider"`
	ModelName          string `gorm:"column:model;type:varchar(128)" json:"model"`
	Title              string `gorm:"type:varchar(256)" json:"title"`
	PreviousResponseId string `gorm:"column:previous_response_id;type:varchar(256)" json:"previous_response_id"`
	History            string `gorm:"type:longtext" json:"history"`
}

func (AIConversation) TableName() string {
	return "ai_conversations"
}
