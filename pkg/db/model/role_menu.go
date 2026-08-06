/*
Copyright 2026 The Pixiu Authors.

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
	register(&RoleMenu{})
}

// RoleMenu 角色显式绑定的菜单权限码。
// 若某角色在本表无任何记录，则运行时按菜单目录的 RequiredAPIs 从 role_apis 推导（兼容存量）。
type RoleMenu struct {
	pixiu.Model

	RoleId   int64  `gorm:"column:role_id;not null;uniqueIndex:uk_role_menu,priority:1" json:"role_id"`
	MenuCode string `gorm:"column:menu_code;type:varchar(128);not null;uniqueIndex:uk_role_menu,priority:2" json:"menu_code"`
}

func (*RoleMenu) TableName() string {
	return "role_menus"
}
