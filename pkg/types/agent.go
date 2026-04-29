/*
Copyright 2024 The Pixiu Authors.

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

import (
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

type Agent struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Name           string            `json:"name"`
	Status         model.AgentStatus `json:"status"`
	UserId         int64             `json:"user_id"`
	LastReportTime time.Time         `json:"last_report_time"`
	Description    string            `json:"description"`
}

type CreateAgentRequest struct {
	Name        string            `json:"name" binding:"required"`
	Status      model.AgentStatus `json:"status" binding:"omitempty"`
	UserId      int64             `json:"user_id" binding:"required"`
	Description string            `json:"description" binding:"omitempty"`
}

type UpdateAgentRequest struct {
	Name            *string            `json:"name" binding:"omitempty"`
	Status          *model.AgentStatus `json:"status" binding:"omitempty"`
	LastReportTime  *time.Time         `json:"last_report_time" binding:"omitempty"`
	Description     *string            `json:"description" binding:"omitempty"`
	ResourceVersion int64              `json:"resource_version" binding:"required"`
}

type AgentListOptions struct {
	PageRequest  `form:",inline"`
	NameSelector string             `form:"nameSelector" json:"nameSelector"`
	UserId       int64              `form:"userId" json:"userId"`
	Status       *model.AgentStatus `form:"status" json:"status"`
}
