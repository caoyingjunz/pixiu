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
	"fmt"

	"github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"
)

func init() {
	register(&Audit{})
}

type AuditOperationStatus uint8

const (
	AuditOpFail    AuditOperationStatus = iota // 执行失败
	AuditOpSuccess                             // 执行成功
	AuditOpUnknown                             // 获取执行状态失败
)

func (s AuditOperationStatus) String() string {
	switch s {
	case AuditOpFail:
		return "failed"
	case AuditOpSuccess:
		return "succeed"
	default:
		return "unknown"
	}
}

type Audit struct {
	pixiu.Model

	RequestId  string               `gorm:"column:request_id;type:varchar(32);index" json:"request_id"`  // 请求 ID
	Ip         string               `gorm:"type:varchar(128)" json:"ip"`                                 // 客户端 IP
	Action     string               `gorm:"type:varchar(255)" json:"action"`                             // HTTP 方法 [POST/DELETE/PUT/GET]
	Operator   string               `gorm:"type:varchar(255)" json:"operator"`                           // 操作人 ID
	Path       string               `gorm:"type:varchar(255)" json:"path"`                               // HTTP 路径
	ObjectType ObjectType           `gorm:"column:resource_type;type:varchar(128)" json:"resource_type"` // 操作资源类型 [cluster/plan...]
	Status     AuditOperationStatus `gorm:"type:tinyint" json:"status"`                                  // 记录操作运行结果[OperationStatus]
}

func (a *Audit) String() string {
	return fmt.Sprintf("user %s(ip addr: %s) access %s with %s then %s", a.Operator, a.Ip,
		a.Path, a.Action, a.Status.String())
}

func (a *Audit) TableName() string {
	return "audits"
}
