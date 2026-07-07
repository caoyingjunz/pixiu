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
	"github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"
)

func init() {
	register(&Runner{})
}

type RunnerStatus uint8

const (
	RunnerStatusUnstart      RunnerStatus = 0 // 未安装
	RunnerStatusInstalling   RunnerStatus = 1 // 安装中
	RunnerStatusUnInstalling RunnerStatus = 2 // 卸载中
	RunnerStatusInstalled    RunnerStatus = 3 // 已安装
	RunnerStatusUnknown      RunnerStatus = 4 // 异常
)

type Runner struct {
	pixiu.Model

	Name        string       `gorm:"index:idx_runner_name,unique;type:varchar(255)" json:"name"`
	EngineImage string       `gorm:"type:varchar(255)" json:"engine_image"`
	Status      RunnerStatus `gorm:"type:tinyint;default:0" json:"status"`
	Description string       `gorm:"type:text" json:"description"`
}

func (a *Runner) TableName() string {
	return "runners"
}
