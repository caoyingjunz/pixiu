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

package model

import (
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"
)

func init() {
	register(&Agent{})
}

type AgentStatus uint8

const (
	AgentStatusUnknown AgentStatus = 0
	AgentStatusOnline  AgentStatus = 1
	AgentStatusOffline AgentStatus = 2
	AgentStatusError   AgentStatus = 3
)

type Agent struct {
	pixiu.Model

	Name           string      `gorm:"index:idx_agent_name,unique;type:varchar(255)" json:"name"`
	Status         AgentStatus `gorm:"type:tinyint;default:0" json:"status"`
	UserId         int64       `gorm:"index" json:"user_id"`
	LastReportTime time.Time   `gorm:"type:datetime" json:"last_report_time"`
	Description    string      `gorm:"type:text" json:"description"`
}

func (a *Agent) TableName() string {
	return "agents"
}
