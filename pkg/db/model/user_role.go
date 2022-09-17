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
