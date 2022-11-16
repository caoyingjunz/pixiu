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

	Memo     string `gorm:"column:memo;size:128;" json:"memo" form:"memo"`                                     // 备注
	Name     string `gorm:"column:name;size:128;not null;unique_index:uk_role_name;;" json:"name" form:"name"` // 名称
	Sequence int    `gorm:"column:sequence;not null;" json:"sequence" form:"sequence"`                         // 排序值
	ParentID int64  `gorm:"column:parent_id;not null;" json:"parent_id" form:"parent_id"`                      // 父级ID
	Status   int8   `gorm:"column:status" json:"status" form:"status"`                                         // 0 表示禁用，1 表示启用
	Children []Role `gorm:"-" json:"children"`
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

type Rule struct {
	gopixiu.Model

	PType  string `json:"ptype" gorm:"column:ptype;size:100" description:"策略类型"`
	Role   string `json:"role" gorm:"column:v0;size:100" description:"角色"`
	Path   string `json:"path" gorm:"column:v1;size:100" description:"api路径"`
	Method string `json:"method" gorm:"column:v2;size:100" description:"访问方法"`
	V3     string `gorm:"column:v3;size:100"`
	V4     string `gorm:"column:v4;size:100"`
	V5     string `gorm:"column:v5;size:100"`
}

func (r *Rule) TableName() string {
	return "rules"
}

// BeforeCreate 添加前
func (r *Rule) BeforeCreate(*gorm.DB) error {
	r.GmtCreate = time.Now()
	r.GmtModified = time.Now()
	return nil
}

// BeforeUpdate 更新前
func (r *Rule) BeforeUpdate(*gorm.DB) error {
	r.GmtModified = time.Now()
	return nil
}

type UserRole struct {
	gopixiu.Model

	UserID int64 `gorm:"column:user_id;unique_index:uk_user_role_user_id;not null;" json:"user_id"` // 管理员ID
	RoleID int64 `gorm:"column:role_id;unique_index:uk_user_role_user_id;not null;" json:"role_id"` // 角色ID
}

// TableName 自定义表名
func (u *UserRole) TableName() string {
	return "user_roles"
}

// BeforeCreate 添加前
func (u *UserRole) BeforeCreate(*gorm.DB) error {
	u.GmtCreate = time.Now()
	u.GmtModified = time.Now()
	return nil
}

// BeforeUpdate 更新前
func (u *UserRole) BeforeUpdate(*gorm.DB) error {
	u.GmtModified = time.Now()
	return nil
}

type PageRole struct {
	Roles []Role `json:"roles"`
	Total int64  `json:"total"`
}
