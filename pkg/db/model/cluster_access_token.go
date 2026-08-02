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

import (
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"
)

func init() {
	register(&ClusterAccessToken{})
}

// ClusterAccessToken 集群代理 kubeconfig 访问令牌（明文仅签发时返回一次）。
type ClusterAccessToken struct {
	pixiu.Model

	JTI         string     `gorm:"column:jti;type:varchar(64);uniqueIndex:uk_jti" json:"jti"`
	UserId      int64      `gorm:"column:user_id;index:idx_user_id" json:"user_id"`
	ClusterId   int64      `gorm:"column:cluster_id;index:idx_cluster_id" json:"cluster_id"`
	ClusterName string     `gorm:"column:cluster_name;type:varchar(128)" json:"cluster_name"`
	Name        string     `gorm:"column:name;type:varchar(128);default:''" json:"name"`
	TokenHash   string     `gorm:"column:token_hash;type:varchar(128);index:idx_token_hash" json:"-"`
	ExpiresAt   *time.Time `gorm:"column:expires_at" json:"expires_at,omitempty"`
	RevokedAt   *time.Time `gorm:"column:revoked_at" json:"revoked_at,omitempty"`
	LastUsedAt  *time.Time `gorm:"column:last_used_at" json:"last_used_at,omitempty"`
}

func (*ClusterAccessToken) TableName() string {
	return "cluster_access_tokens"
}

func (t *ClusterAccessToken) IsRevoked() bool {
	return t != nil && t.RevokedAt != nil
}

func (t *ClusterAccessToken) IsExpired() bool {
	if t == nil || t.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*t.ExpiresAt)
}

func (t *ClusterAccessToken) IsActive() bool {
	return t != nil && !t.IsRevoked() && !t.IsExpired()
}
