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
	register(&AIConversation{})
	register(&AIResponseExecution{})
	register(&AIToolExecution{})
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

type AIResponseExecution struct {
	pixiu.Model

	RequestId       string `gorm:"column:request_id;type:varchar(32);index" json:"request_id"`
	UserId          int64  `gorm:"column:user_id;not null;index:idx_ai_response_executions_user_id" json:"user_id"`
	UserName        string `gorm:"column:user_name;type:varchar(100);index:idx_ai_response_executions_user_name" json:"user_name"`
	AIAccountId     int64  `gorm:"column:ai_account_id;not null;index:idx_ai_response_executions_ai_account_id" json:"ai_account_id"`
	ConversationId  int64  `gorm:"column:conversation_id;index:idx_ai_response_executions_conversation_id" json:"conversation_id"`
	Provider        string `gorm:"column:provider;type:varchar(64);index:idx_ai_response_executions_provider" json:"provider"`
	ModelName       string `gorm:"column:model;type:varchar(128)" json:"model"`
	ResponseId      string `gorm:"column:response_id;type:varchar(128);index:idx_ai_response_executions_response_id" json:"response_id"`
	InputText       string `gorm:"column:input_text;type:longtext" json:"input_text"`
	OutputText      string `gorm:"column:output_text;type:longtext" json:"output_text"`
	Success         bool   `gorm:"column:success;not null;default:false;index:idx_ai_response_executions_success" json:"success"`
	ErrorMessage    string `gorm:"column:error_message;type:text" json:"error_message"`
	Duration        int64  `gorm:"column:duration;type:bigint;default:0" json:"duration"`
	InputTokens     int64  `gorm:"column:input_tokens;type:bigint;default:0" json:"input_tokens"`
	OutputTokens    int64  `gorm:"column:output_tokens;type:bigint;default:0" json:"output_tokens"`
	TotalTokens     int64  `gorm:"column:total_tokens;type:bigint;default:0" json:"total_tokens"`
	CachedTokens    int64  `gorm:"column:cached_tokens;type:bigint;default:0" json:"cached_tokens"`
	ReasoningTokens int64  `gorm:"column:reasoning_tokens;type:bigint;default:0" json:"reasoning_tokens"`
}

func (AIResponseExecution) TableName() string {
	return "ai_response_executions"
}

type AIToolExecution struct {
	pixiu.Model

	RequestId      string `gorm:"column:request_id;type:varchar(32);index" json:"request_id"`
	UserId         int64  `gorm:"column:user_id;not null;index:idx_ai_tool_executions_user_id" json:"user_id"`
	UserName       string `gorm:"column:user_name;type:varchar(100);index:idx_ai_tool_executions_user_name" json:"user_name"`
	AIAccountId    int64  `gorm:"column:ai_account_id;not null;index:idx_ai_tool_executions_ai_account_id" json:"ai_account_id"`
	ConversationId int64  `gorm:"column:conversation_id;index:idx_ai_tool_executions_conversation_id" json:"conversation_id"`
	Provider       string `gorm:"column:provider;type:varchar(64);index:idx_ai_tool_executions_provider" json:"provider"`
	ModelName      string `gorm:"column:model;type:varchar(128)" json:"model"`
	ToolName       string `gorm:"column:tool_name;type:varchar(128);index:idx_ai_tool_executions_tool_name" json:"tool_name"`
	CallId         string `gorm:"column:call_id;type:varchar(128);index:idx_ai_tool_executions_call_id" json:"call_id"`
	Arguments      string `gorm:"column:arguments;type:longtext" json:"arguments"`
	Output         string `gorm:"column:output;type:longtext" json:"output"`
	Success        bool   `gorm:"column:success;not null;default:false;index:idx_ai_tool_executions_success" json:"success"`
	ErrorMessage   string `gorm:"column:error_message;type:text" json:"error_message"`
	Duration       int64  `gorm:"column:duration;type:bigint;default:0" json:"duration"`
}

func (AIToolExecution) TableName() string {
	return "ai_tool_executions"
}
