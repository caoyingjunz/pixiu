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
	register(&Permission{})
}

// PermissionPType 授权类型：0 只读 1 自定义 2 管理员
type PermissionPType int8

const (
	PermissionPTypeReadonly PermissionPType = iota
	PermissionPTypeCustom
	PermissionPTypeAdmin
)

// Permission 集群 scoped kubeconfig 授权记录
type Permission struct {
	pixiu.Model

	// 授权名称，同一用户在同一集群下唯一
	Name string `gorm:"type:varchar(128);not null;uniqueIndex:uk_user_cluster_name,priority:3" json:"name"`

	UserId   int64  `gorm:"column:user_id;not null;index:idx_user_id;index:idx_user_cluster,priority:1;uniqueIndex:uk_user_cluster_name,priority:1" json:"user_id"`
	UserName string `gorm:"column:user_name;not null;index:idx_user_name,priority:1" json:"user_name"`

	// 目标集群名称（全局唯一集群名）
	ClusterId   int64  `gorm:"column:cluster_id;type:varchar(128);not null;index:idx_user_cluster,priority:2;uniqueIndex:uk_user_cluster_name,priority:2" json:"cluster_name"`
	ClusterName string `json:"cluster_name"`

	// 所属主集群
	OwnerClusterId        int64  `json:"owner_cluster_id"`
	OwnerClusterName      string `json:"owner_cluster_name"`
	OwnerClusterAliasName string `json:"owner_cluster_alias_name"`

	ExpirationSeconds int64           `gorm:"column:expiration_seconds" json:"expiration_seconds"`
	PType             PermissionPType `gorm:"column:p_type" json:"p_type"`

	// PType=1 时生效，JSON 序列化的 PolicyRule 列表
	Rules string `gorm:"type:text" json:"rules"`

	SAName      string `gorm:"column:sa_name" json:"sa_name"`
	SANamespace string `gorm:"column:sa_namespace" json:"sa_namespace"`

	ClusterRoleName string `gorm:"column:cluster_role_name" json:"cluster_role_name"`
	RoleBindingName string `gorm:"column:role_binding_name" json:"role_binding_name"`

	// JSON 序列化的命名空间列表
	TargetNamespaces string `gorm:"column:target_namespaces" json:"target_namespaces"`

	Description string `gorm:"type:text" json:"description,omitempty"`
}

func (*Permission) TableName() string {
	return "permissions"
}
