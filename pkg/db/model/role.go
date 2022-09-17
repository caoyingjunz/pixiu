/*
Copyright 2021 The Pixiu Authors.

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

	"gorm.io/gorm"

	"github.com/caoyingjunz/gopixiu/pkg/db/gopixiu"
)

// Role 角色
type Role struct {
	gopixiu.Model

	Memo     string `gorm:"column:memo;size:128;" json:"memo" form:"memo"`                // 备注
	Name     string `gorm:"column:name;size:128;not null;" json:"name" form:"name"`       // 名称
	Sequence int    `gorm:"column:sequence;not null;" json:"sequence" form:"sequence"`    // 排序值
	ParentID int64  `gorm:"column:parent_id;not null;" json:"parent_id" form:"parent_id"` // 父级ID
	Status   int8   `gorm:"column:status" json:"status" form:"status"`                    // 0 表示禁用，1 表示启用
	Children []Role `gorm:"-"`
}

// TableName 自定义表名
func (r *Role) TableName() string {
	return "roles"
}

// BeforeCreate 创建前操作
func (r *Role) BeforeCreate(*gorm.DB) error {
	r.GmtCreate = time.Now()
	r.GmtModified = time.Now()
	return nil
}

// BeforeUpdate 更新前操作
func (r *Role) BeforeUpdate(*gorm.DB) error {
	r.GmtModified = time.Now()
	return nil
}
