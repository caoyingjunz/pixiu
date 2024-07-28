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

import "github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"

type OperationStatus uint8

const (
	OperationFail    OperationStatus = iota // 执行失败
	OperationSuccess                        // 执行成功
	OperationUnknow                         // 获取执行状态失败
)

type Audit struct {
	pixiu.Model
	Ip           string          `gorm:"type:varchar(128)" json:"ip"`            // 客户端ip
	Action       string          `gorm:"type:varchar(255)" json:"action"`        // HTTP 方法[POST/DELETE/PUT/GET]
	Operator     string          `gorm:"type:varchar(255)" json:"operator"`      // 操作人ID
	Path         string          `gorm:"type:varchar(255)" json:"path"`          // HTTP 路径
	ResourceType string          `gorm:"type:varchar(128)" json:"resource_type"` // 操作资源类型[cluster/plan...]
	Status       OperationStatus `gorm:"type:tinyint" json:"status"`             // 记录操作运行结果[OperationStatus]
}

func (a *Audit) TableName() string {
	return "audit"
}
