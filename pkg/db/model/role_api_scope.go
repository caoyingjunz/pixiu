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

// RoleAPIScope 角色对 Kubernetes 资源的细粒度授权范围。
// 同一角色下 (api_id, cluster, namespace, resource_name) 唯一。
type RoleAPIScope struct {
	pixiu.Model

	RoleId       int64  `gorm:"column:role_id;not null;index:idx_role_api_scope_role;uniqueIndex:uk_role_api_scope,priority:1" json:"role_id"`
	APIId        int64  `gorm:"column:api_id;not null;uniqueIndex:uk_role_api_scope,priority:2" json:"api_id"`
	Cluster      string `gorm:"column:cluster;type:varchar(128);not null;uniqueIndex:uk_role_api_scope,priority:3" json:"cluster"`
	Namespace    string `gorm:"column:namespace;type:varchar(128);not null;uniqueIndex:uk_role_api_scope,priority:4" json:"namespace"`
	ResourceName string `gorm:"column:resource_name;type:varchar(256);not null;default:'*';uniqueIndex:uk_role_api_scope,priority:5" json:"resource_name"`
}

func (*RoleAPIScope) TableName() string {
	return "role_api_scopes"
}
