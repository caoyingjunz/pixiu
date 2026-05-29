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
	register(&Tenant{}, &Role{}, &UserRole{}, &API{}, &RoleAPI{}, &RoleAPIScope{})
}

type Tenant struct {
	pixiu.Model

	Name        string `gorm:"index:idx_name,unique" json:"name"`
	Description string `gorm:"type:text" json:"description"`
	Extension   string `gorm:"type:text" json:"extension,omitempty"`
}

func (tenant *Tenant) TableName() string {
	return "tenants"
}

type Role struct {
	pixiu.Model

	TenantId    int64  `gorm:"default:null;uniqueIndex:uk_tenant_role" json:"tenant_id"` // NULL 表示系统全局角色
	Name        string `gorm:"type:varchar(100);not null;uniqueIndex:uk_tenant_role" json:"name"`
	Description string `gorm:"type:text" json:"description"`
}

func (role *Role) TableName() string {
	return "roles"
}

type UserRole struct {
	pixiu.Model

	UserId int64 `gorm:"primaryKey;not null" json:"user_id"`
	RoleId int64 `gorm:"primaryKey;not null" json:"role_id"`
}

func (ur *UserRole) TableName() string {
	return "user_roles"
}

type API struct {
	pixiu.Model

	Method      string `gorm:"type:varchar(10);not null;uniqueIndex:uk_method_path" json:"method"`
	Path        string `gorm:"type:varchar(255);not null;uniqueIndex:uk_method_path" json:"path"`
	Group       string `gorm:"column:api_group;type:varchar(100);index:idx_api_group" json:"group"`
	SubGroup    string `gorm:"column:api_sub_group;type:varchar(100);index:idx_api_sub_group" json:"sub_group"`
	Description string `gorm:"type:varchar(255)" json:"description"`
}

func (api *API) TableName() string {
	return "apis"
}

type RoleAPI struct {
	pixiu.Model

	RoleId int64 `gorm:"primaryKey;not null" json:"role_id"`
	APIId  int64 `gorm:"primaryKey;not null" json:"api_id"`
}

func (api *RoleAPI) TableName() string {
	return "role_apis"
}

type RoleAPIScope struct {
	pixiu.Model

	RoleId int64 `gorm:"not null;index:idx_role" json:"role_id"`
	APIId  int64 `gorm:"not null" json:"api_id"`

	Cluster      string `gorm:"type:varchar(128);not null;index:idx_role_cluster,priority:2" json:"cluster"`
	Namespace    string `gorm:"type:varchar(128);not null" json:"namespace"`
	ResourceName string `gorm:"type:varchar(253);not null;default:*" json:"resource_name"`
}

func (s *RoleAPIScope) TableName() string {
	return "role_api_scopes"
}
