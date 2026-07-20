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
	register(&AIProvider{}, &AIAccount{}, &Conversation{}, &Message{}, &Execution{})
}

// AIProvider stores an AI vendor endpoint and its wire protocol.
type AIProvider struct {
	pixiu.Model

	Name        string `gorm:"column:name;type:varchar(128);not null;uniqueIndex:uk_ai_providers_name" json:"name"`
	BaseURL     string `gorm:"column:base_url;type:varchar(512);not null" json:"base_url"`
	Protocol    string `gorm:"column:protocol;type:varchar(32);not null" json:"protocol"`
	Description string `gorm:"column:description;type:text" json:"description"`
	MaxTokens   int    `gorm:"column:max_tokens;not null;default:4096" json:"max_tokens"`
	Builtin     bool   `gorm:"column:builtin;not null;default:false" json:"builtin"`
}

func (AIProvider) TableName() string {
	return "ai_providers"
}

// AIAccount stores one model credential belonging to an AI provider.
type AIAccount struct {
	pixiu.Model

	UserId     int64       `gorm:"column:user_id;not null;index:idx_ai_accounts_user_id;uniqueIndex:uk_ai_accounts_user_provider_name,priority:1" json:"user_id"`
	ProviderId int64       `gorm:"column:provider_id;not null;index:idx_ai_accounts_provider_id;uniqueIndex:uk_ai_accounts_user_provider_name,priority:2" json:"provider_id"`
	Name       string      `gorm:"column:name;type:varchar(128);not null;uniqueIndex:uk_ai_accounts_user_provider_name,priority:3" json:"name"`
	APIKey     string      `gorm:"column:api_key;type:text;not null" json:"-"`
	ModelName  string      `gorm:"column:model;type:varchar(128);not null" json:"model"`
	Provider   *AIProvider `gorm:"foreignKey:ProviderId;references:Id" json:"-"`
}

func (AIAccount) TableName() string {
	return "ai_accounts"
}

// Conversation stores persisted response-chain context for one user conversation.
type Conversation struct {
	pixiu.Model

	ProviderId         int64  `gorm:"column:provider_id;not null;index:idx_conversations_provider_id" json:"provider_id"`
	Provider           string `gorm:"type:varchar(64);not null" json:"provider"`
	ModelName          string `gorm:"column:model;type:varchar(128)" json:"model"`
	Title              string `gorm:"type:varchar(256)" json:"title"`
	PreviousResponseId string `gorm:"column:previous_response_id;type:varchar(256)" json:"previous_response_id"`
	History            string `gorm:"type:longtext" json:"history"`
}

func (Conversation) TableName() string {
	return "conversations"
}

type Message struct {
	pixiu.Model

	RequestId       string `gorm:"column:request_id;type:varchar(32);index" json:"request_id"`
	ProviderId      int64  `gorm:"column:provider_id;not null;index:idx_messages_provider_id" json:"provider_id"`
	ConversationId  int64  `gorm:"column:conversation_id;index:idx_messages_conversation_id" json:"conversation_id"`
	Provider        string `gorm:"column:provider;type:varchar(64);index:idx_messages_provider" json:"provider"`
	ModelName       string `gorm:"column:model;type:varchar(128)" json:"model"`
	ResponseId      string `gorm:"column:response_id;type:varchar(128);index:idx_messages_response_id" json:"response_id"`
	InputText       string `gorm:"column:input_text;type:longtext" json:"input_text"`
	OutputText      string `gorm:"column:output_text;type:longtext" json:"output_text"`
	Success         bool   `gorm:"column:success;not null;default:false;index:idx_messages_success" json:"success"`
	ErrorMessage    string `gorm:"column:error_message;type:text" json:"error_message"`
	Duration        int64  `gorm:"column:duration;type:bigint;default:0" json:"duration"`
	InputTokens     int64  `gorm:"column:input_tokens;type:bigint;default:0" json:"input_tokens"`
	OutputTokens    int64  `gorm:"column:output_tokens;type:bigint;default:0" json:"output_tokens"`
	TotalTokens     int64  `gorm:"column:total_tokens;type:bigint;default:0" json:"total_tokens"`
	CachedTokens    int64  `gorm:"column:cached_tokens;type:bigint;default:0" json:"cached_tokens"`
	ReasoningTokens int64  `gorm:"column:reasoning_tokens;type:bigint;default:0" json:"reasoning_tokens"`
}

func (Message) TableName() string {
	return "messages"
}

type Execution struct {
	pixiu.Model

	RequestId      string `gorm:"column:request_id;type:varchar(32);index" json:"request_id"`
	ProviderId     int64  `gorm:"column:provider_id;not null;index:idx_executions_provider_id" json:"provider_id"`
	ConversationId int64  `gorm:"column:conversation_id;index:idx_executions_conversation_id" json:"conversation_id"`
	Provider       string `gorm:"column:provider;type:varchar(64);index:idx_executions_provider" json:"provider"`
	ModelName      string `gorm:"column:model;type:varchar(128)" json:"model"`
	ToolName       string `gorm:"column:tool_name;type:varchar(128);index:idx_executions_tool_name" json:"tool_name"`
	CallId         string `gorm:"column:call_id;type:varchar(128);index:idx_executions_call_id" json:"call_id"`
	Arguments      string `gorm:"column:arguments;type:longtext" json:"arguments"`
	Output         string `gorm:"column:output;type:longtext" json:"output"`
	Success        bool   `gorm:"column:success;not null;default:false;index:idx_executions_success" json:"success"`
	ErrorMessage   string `gorm:"column:error_message;type:text" json:"error_message"`
	Duration       int64  `gorm:"column:duration;type:bigint;default:0" json:"duration"`
}

func (Execution) TableName() string {
	return "executions"
}
