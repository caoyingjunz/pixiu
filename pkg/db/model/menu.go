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

// Menu 菜单
type Menu struct {
	gopixiu.Model

	Status   int8   `gorm:"column:status;type:tinyint(1);not null;" json:"status" form:"status"`          // 状态(1:启用 2:不启用)
	Memo     string `gorm:"column:memo;size:128;" json:"memo" form:"memo"`                                // 备注
	ParentID int64  `gorm:"column:parent_id;not null;" json:"parent_id" form:"parent_id"`                 // 父级ID
	URL      string `gorm:"column:url;size:128;" json:"url" form:"url"`                                   // 菜单URL
	Name     string `gorm:"column:name;size:128;not null;" json:"name" form:"name"`                       // 菜单名称
	Sequence int    `gorm:"column:sequence;not null;" json:"sequence" form:"sequence"`                    // 排序值
	MenuType int8   `gorm:"column:menu_type;type:tinyint(1);not null;" json:"menu_type" form:"menu_type"` // 菜单类型 1 左侧菜单,2 按钮, 3 非展示权限
	Icon     string `gorm:"column:icon;size:32;" json:"icon" form:"icon"`                                 // icon
	Method   string `gorm:"column:method;size:32;not null;" json:"method" form:"method"`                  // 操作类型 none/GET/POST/PUT/DELETE
	Code     string `gorm:"column:code;size:128;not null;" json:"code"`                                   // 前端鉴权code 例： user:role:add, user:role:delete
	Children []Menu `gorm:"-" json:"children"`
}

// TableName 表名
func (m *Menu) TableName() string {
	return "menus"
}

// BeforeCreate 添加前
func (m *Menu) BeforeCreate(*gorm.DB) error {
	m.GmtCreate = time.Now()
	m.GmtModified = time.Now()
	return nil
}

// BeforeUpdate 更新前
func (m *Menu) BeforeUpdate(tx *gorm.DB) error {
	m.GmtModified = time.Now()
	return nil
}

// RoleMenu 角色-菜单
type RoleMenu struct {
	gopixiu.Model

	RoleID int64 `gorm:"column:role_id;unique_index:uk_role_menu_role_id;not null;" json:"role_id"`  // 角色ID
	MenuID int64 `gorm:"column:menu_id;unique_index:uk_role_menu_role_id;not null;" json:"menu_id'"` // 菜单ID
}

// TableName 表名
func (m *RoleMenu) TableName() string {
	return "role_menus"
}

// BeforeCreate 添加前
func (m *RoleMenu) BeforeCreate(*gorm.DB) error {
	m.GmtCreate = time.Now()
	m.GmtModified = time.Now()
	return nil
}

// BeforeUpdate 更新前
func (m *RoleMenu) BeforeUpdate(*gorm.DB) error {
	m.GmtModified = time.Now()
	return nil
}

// PageMenu 分页菜单
type PageMenu struct {
	Menus []Menu `json:"menus"`
	Total int64  `json:"total"`
}
