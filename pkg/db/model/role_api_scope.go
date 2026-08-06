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
	register(&RoleAPIScope{})
}

// RoleAPIScope 角色对 pixiu 自有资源的细粒度授权范围。
// 同一角色下 (api_id, resource_type, resource_id) 唯一。
type RoleAPIScope struct {
	pixiu.Model

	RoleId       int64  `gorm:"column:role_id;not null;uniqueIndex:uk_role_api_scope,priority:1" json:"role_id"`
	APIId        int64  `gorm:"column:api_id;not null;uniqueIndex:uk_role_api_scope,priority:2" json:"api_id"`
	ResourceType string `gorm:"column:resource_type;type:varchar(64);not null;uniqueIndex:uk_role_api_scope,priority:3" json:"resource_type"`
	ResourceId   int64  `gorm:"column:resource_id;not null;uniqueIndex:uk_role_api_scope,priority:4" json:"resource_id"`
}

func (*RoleAPIScope) TableName() string {
	return "role_api_scopes"
}
