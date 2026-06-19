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
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

type Runner struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Name        string             `json:"name"`
	EngineImage string             `json:"engine_image"`
	Status      model.RunnerStatus `json:"status"`
	Description string             `json:"description"`
}

type CreateRunnerRequest struct {
	Name        string             `json:"name" binding:"required"`
	EngineImage string             `json:"engine_image" binding:"required"`
	Status      model.RunnerStatus `json:"status" binding:"omitempty"`
	Description string             `json:"description" binding:"omitempty"`
}

type UpdateRunnerRequest struct {
	Name            *string             `json:"name" binding:"omitempty"`
	EngineImage     *string             `json:"engine_image" binding:"omitempty"`
	Status          *model.RunnerStatus `json:"status" binding:"omitempty"`
	Description     *string             `json:"description" binding:"omitempty"`
	ResourceVersion int64               `json:"resource_version" binding:"required"`
}

type RunnerListOptions struct {
	PageRequest  `form:",inline"`
	NameSelector string              `form:"nameSelector" json:"nameSelector"`
	Status       *model.RunnerStatus `form:"status" json:"status"`
}
