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

func init() {
	register(&User{})
}

type UserLevel int64

const (
	RoleRoot  UserLevel = iota // 超级管理员
	RoleAdmin                  // 管理员
	RoleUser                   // 普通用户
)

type UserStatus uint8 // 0 正常 1 禁用

const (
	UserStatusNormal    UserStatus = iota // 正常
	UserStatusForbidden                   // 禁用
)

type User struct {
	pixiu.Model

	TenantId      int64      `gorm:"not null;uniqueIndex:uk_tenant_username" json:"tenant_id"`
	Name          string     `gorm:"type:varchar(100);not null;uniqueIndex:uk_tenant_username" json:"username"`
	Password      string     `gorm:"type:varchar(256)" json:"-"`
	Status        UserStatus `gorm:"type:tinyint" json:"status"`
	Role          UserLevel  `json:"role"`
	Email         string     `gorm:"type:varchar(128)" json:"email"`
	Phone         string     `gorm:"column:phone;type:varchar(32)" json:"phone"`
	OAuthProvider string     `gorm:"column:oauth_provider;type:varchar(32);index:idx_oauth_provider_open_id,priority:1;index:idx_oauth_provider_union_id,priority:1" json:"oauth_provider"`
	OAuthOpenID   string     `gorm:"column:oauth_open_id;type:varchar(128);index:idx_oauth_provider_open_id,priority:2" json:"oauth_open_id"`
	OAuthUnionID  string     `gorm:"column:oauth_union_id;type:varchar(128);index:idx_oauth_provider_union_id,priority:2" json:"oauth_union_id"`
	OAuthUserID   string     `gorm:"column:oauth_user_id;type:varchar(128)" json:"oauth_user_id"`
	AvatarURL     string     `gorm:"column:avatar_url;type:varchar(512)" json:"avatar_url"`
	Description   string     `gorm:"type:text" json:"description"`
	Extension     string     `gorm:"type:text" json:"extension,omitempty"`
}

func (user *User) TableName() string {
	return "users"
}
