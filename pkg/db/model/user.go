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

type UserRole uint8

const (
	RoleUser  UserRole = iota // 普通用户
	RoleAdmin                 // 管理员
	RoleRoot                  // 超级管理员
)

type UserStatus uint8 // TODO

type User struct {
	pixiu.Model

	Name        string     `gorm:"index:idx_name,unique" json:"name"`
	Password    string     `gorm:"type:varchar(256)" json:"-"`
	Status      UserStatus `gorm:"type:tinyint" json:"status"`
	Role        UserRole   `gorm:"type:tinyint" json:"role"`
	Email       string     `gorm:"type:varchar(128)" json:"email"`
	Description string     `gorm:"type:text" json:"description"`
	Extension   string     `gorm:"type:text" json:"extension,omitempty"`
}

func (user *User) TableName() string {
	return "users"
}
