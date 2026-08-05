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

	Name          string                  `json:"name"`
	Type          model.AgentType         `json:"type"`
	UserID        int64                   `json:"user_id"`
	Status        model.DeployAgentStatus `json:"status"`
	Hostname      string                  `json:"hostname"`
	Version       string                  `json:"version"`
	LastHeartbeat time.Time               `json:"last_heartbeat"`
	Description   string                  `json:"description"`
	Token         string                  `json:"token,omitempty"`
}

type CreateAgentRequest struct {
	Name        string          `json:"name" binding:"required"`
	UserID      int64           `json:"user_id"`
	Type        model.AgentType `json:"type" binding:"omitempty"`
	Description string          `json:"description" binding:"omitempty"`
}

// SetUserID 实现 UserIDSetter 接口。
func (r *CreateAgentRequest) SetUserID(id int64) { r.UserID = id }

type UpdateAgentRequest struct {
	Name            *string          `json:"name" binding:"omitempty"`
	ResourceVersion int64            `json:"resource_version"`
	Type            *model.AgentType `json:"type" binding:"omitempty"`
	Description     *string          `json:"description" binding:"omitempty"`
}
