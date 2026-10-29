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

import (
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"
)

func init() {
	register(&ClusterExpansion{})
}

// ClusterExpansion 集群节点扩容记录。每次扩容一条记录，保留历史与日志定位信息。
type ClusterExpansion struct {
	pixiu.Model

	ClusterId int64 `gorm:"index:idx_expansion_cluster_id" json:"cluster_id"`
	UserId    int64 `gorm:"index:idx_expansion_user_id" json:"user_id"`

	// PlanId 关联的扩容计划（plans.type=1，系统在添加节点时自动创建）
	PlanId int64 `gorm:"index:idx_expansion_plan_id;default:0" json:"plan_id"`

	// NodeIds 本次扩容的目标节点 ID 列表（JSON 数组）
	NodeIds string `gorm:"type:text" json:"node_ids"`
	// Options 本次扩容的执行参数（JSON）
	Options string `gorm:"type:text" json:"options"`

	Status TaskStatus `gorm:"type:varchar(32);default:'未开始';index:idx_expansion_status" json:"status"`
	// Step 当前步骤：初始化 / 加入集群 / 完成
	Step    string `gorm:"type:varchar(64)" json:"step"`
	Message string `gorm:"type:text" json:"message"`

	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

func (e *ClusterExpansion) TableName() string {
	return "cluster_expansions"
}
