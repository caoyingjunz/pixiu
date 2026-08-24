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
	register(&CronHpa{})
	register(&CronHpaHistory{})
}

// CronHpaStatus 定时扩缩容规则的启停状态
type CronHpaStatus string

const (
	// CronHpaStatusActive 启用：调度器按周期扫描并执行到期任务
	CronHpaStatusActive CronHpaStatus = "active"
	// CronHpaStatusPaused 暂停：调度器扫描时跳过
	CronHpaStatusPaused CronHpaStatus = "paused"
)

// CronHpa 定时扩缩容规则（由 pixiu 后端调度执行，无需在集群侧安装控制器）
type CronHpa struct {
	pixiu.Model

	Name        string `gorm:"column:name;type:varchar(128);not null" json:"name"`
	ClusterName string `gorm:"column:cluster_name;type:varchar(128);not null;index:idx_cluster_name" json:"cluster_name"`
	Namespace   string `gorm:"column:namespace;type:varchar(128);not null" json:"namespace"`
	// TargetKind 支持 Deployment / StatefulSet / HorizontalPodAutoscaler（兼容模式：调整 HPA 的 min/max）
	TargetKind string `gorm:"column:target_kind;type:varchar(64);not null" json:"target_kind"`
	TargetName string `gorm:"column:target_name;type:varchar(128);not null" json:"target_name"`
	// Jobs 定时任务列表，JSON 数组，元素结构见 types.CronHpaJob
	Jobs string `gorm:"column:jobs;type:text" json:"jobs"`
	// ExcludeDates 可选，JSON 数组：cron 表达式集合，当日命中任一则跳过当天执行
	ExcludeDates string        `gorm:"column:exclude_dates;type:text" json:"exclude_dates"`
	Status       CronHpaStatus `gorm:"column:status;type:varchar(16);not null" json:"status"`
	Description  string        `gorm:"column:description;type:varchar(255)" json:"description"`
	CreateUser   string        `gorm:"column:create_user;type:varchar(128)" json:"create_user"`
}

func (*CronHpa) TableName() string {
	return "cron_hpas"
}

// CronHpaHistory 定时扩缩容执行历史
type CronHpaHistory struct {
	pixiu.Model

	CronHpaId        int64     `gorm:"column:cron_hpa_id;not null;index:idx_cron_hpa_id" json:"cron_hpa_id"`
	JobName          string    `gorm:"column:job_name;type:varchar(128)" json:"job_name"`
	ScheduledTime    time.Time `gorm:"column:scheduled_time;type:datetime" json:"scheduled_time"`
	ExecutedAt       time.Time `gorm:"column:executed_at;type:datetime" json:"executed_at"`
	PreviousReplicas int32     `gorm:"column:previous_replicas" json:"previous_replicas"`
	DesiredReplicas  int32     `gorm:"column:desired_replicas" json:"desired_replicas"`
	// Result 取值 Succeed / Failed / Skipped
	Result  string `gorm:"column:result;type:varchar(16);not null" json:"result"`
	Message string `gorm:"column:message;type:varchar(512)" json:"message"`
}

func (*CronHpaHistory) TableName() string {
	return "cron_hpa_histories"
}
