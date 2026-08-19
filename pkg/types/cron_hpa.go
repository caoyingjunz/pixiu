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

package types

import "time"

// 定时扩缩容任务状态
const (
	CronHpaJobStateSubmitted = "Submitted" // 已登记，尚未触发
	CronHpaJobStateSucceed   = "Succeed"   // 最近一次执行成功
	CronHpaJobStateFailed    = "Failed"    // 最近一次执行失败
	CronHpaJobStateSkipped   = "Skipped"   // 命中排除日期被跳过
)

// 定时扩缩容执行历史结果
const (
	CronHpaHistoryResultSucceed = "Succeed"
	CronHpaHistoryResultFailed  = "Failed"
	CronHpaHistoryResultSkipped = "Skipped"
)

// 定时扩缩容支持的目标类型
const (
	CronHpaTargetKindDeployment  = "Deployment"
	CronHpaTargetKindStatefulSet = "StatefulSet"
	// CronHpaTargetKindHpa 兼容模式：目标是已存在的 HPA，定时调整其 min/max，
	// 与指标弹性共存（副本数仍由 HPA 按指标决定）
	CronHpaTargetKindHpa = "HorizontalPodAutoscaler"
)

// 定时扩缩容规则启停状态（与 model.CronHpaStatus 取值保持一致）
const (
	CronHpaStatusActive = "active"
	CronHpaStatusPaused = "paused"
)

// CronHpaJob 单条定时任务
type CronHpaJob struct {
	Name string `json:"name"`
	// Schedule 标准 5 段 cron 表达式（分 时 日 月 周），如 "0 9 * * *"
	Schedule   string `json:"schedule"`
	TargetSize int32  `json:"target_size"`
	RunOnce    bool   `json:"run_once"`
	// LastFireTime 最近一次应触发时刻（由调度器推进，用于判定下次触发与补跑）
	LastFireTime *time.Time `json:"last_fire_time,omitempty"`
	State        string     `json:"state,omitempty"`
	Message      string     `json:"message,omitempty"`
}

// CronHpaRequest 创建/更新定时扩缩容规则请求
type CronHpaRequest struct {
	Name        string       `json:"name" binding:"required"`
	ClusterName string       `json:"cluster_name" binding:"required"`
	Namespace   string       `json:"namespace" binding:"required"`
	TargetKind  string       `json:"target_kind" binding:"required"`
	TargetName  string       `json:"target_name" binding:"required"`
	Jobs        []CronHpaJob `json:"jobs" binding:"required"`
	// ExcludeDates cron 表达式集合，当日命中任一则跳过当天执行
	ExcludeDates []string `json:"exclude_dates"`
	Description  string   `json:"description"`
}

// CronHpaStatusRequest 暂停/恢复请求
type CronHpaStatusRequest struct {
	// Status 取值 active / paused
	Status string `json:"status" binding:"required"`
}

// CronHpa 返回给前端的定时扩缩容规则对象
type CronHpa struct {
	Id           int64        `json:"id"`
	GmtCreate    time.Time    `json:"gmt_create"`
	GmtModified  time.Time    `json:"gmt_modified"`
	Name         string       `json:"name"`
	ClusterName  string       `json:"cluster_name"`
	Namespace    string       `json:"namespace"`
	TargetKind   string       `json:"target_kind"`
	TargetName   string       `json:"target_name"`
	Jobs         []CronHpaJob `json:"jobs"`
	ExcludeDates []string     `json:"exclude_dates,omitempty"`
	Status       string       `json:"status"`
	Description  string       `json:"description"`
	CreateUser   string       `json:"create_user"`
}

// CronHpaHistory 定时扩缩容执行历史
type CronHpaHistory struct {
	Id               int64     `json:"id"`
	CronHpaId        int64     `json:"cron_hpa_id"`
	JobName          string    `json:"job_name"`
	ScheduledTime    time.Time `json:"scheduled_time"`
	ExecutedAt       time.Time `json:"executed_at"`
	PreviousReplicas int32     `json:"previous_replicas"`
	DesiredReplicas  int32     `json:"desired_replicas"`
	Result           string    `json:"result"`
	Message          string    `json:"message"`
}

// CronHpaListOptions 列表查询条件
type CronHpaListOptions struct {
	Cluster   string `form:"cluster"`
	Namespace string `form:"namespace"`
}

// CronHpaHistoryOptions 执行历史查询条件
type CronHpaHistoryOptions struct {
	Limit int `form:"limit"`
}
