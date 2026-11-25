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
	register(&CICDPipeline{}, &CICDRun{}, &CICDTrigger{})
}

// CICDRunStatus 流水线运行状态，与设计方案 3.5 状态机一致。
type CICDRunStatus string

const (
	CICDRunPending   CICDRunStatus = "pending"
	CICDRunRunning   CICDRunStatus = "running"
	CICDRunSucceeded CICDRunStatus = "succeeded"
	CICDRunFailed    CICDRunStatus = "failed"
	CICDRunCancelled CICDRunStatus = "cancelled"
	CICDRunTimeout   CICDRunStatus = "timeout"
)

// IsTerminal 判断运行状态是否终态，终态记录不再被 worker 领取。
func (s CICDRunStatus) IsTerminal() bool {
	switch s {
	case CICDRunSucceeded, CICDRunFailed, CICDRunCancelled, CICDRunTimeout:
		return true
	default:
		return false
	}
}

// CICDTriggerType 触发类型。
type CICDTriggerType string

const (
	CICDTriggerManual   CICDTriggerType = "manual"
	CICDTriggerWebhook  CICDTriggerType = "webhook"
	CICDTriggerSchedule CICDTriggerType = "schedule"
	CICDTriggerAPI      CICDTriggerType = "api"
)

// CICDPipeline 流水线定义。归属租户 + 目标集群 + 命名空间。
type CICDPipeline struct {
	pixiu.Model

	TenantId       int64  `gorm:"not null;default:0;index:idx_cicd_pipelines_tenant_name,priority:1" json:"tenant_id"`
	Name           string `gorm:"type:varchar(128);not null;index:idx_cicd_pipelines_tenant_name,priority:2" json:"name"`
	Description    string `gorm:"type:text" json:"description"`
	ClusterId      int64  `gorm:"not null;index:idx_cicd_pipelines_cluster_id" json:"cluster_id"`
	Namespace      string `gorm:"type:varchar(128);not null" json:"namespace"`
	Engine         string `gorm:"type:varchar(16);not null;default:'tekton'" json:"engine"`
	SpecMode       string `gorm:"type:varchar(16);not null;default:'template'" json:"spec_mode"`
	Template       string `gorm:"type:varchar(64);not null;default:'source-build'" json:"template"`
	Config         string `gorm:"type:longtext" json:"config"`
	PipelineYaml   string `gorm:"type:longtext" json:"pipeline_yaml"`
	TimeoutSeconds int    `gorm:"default:1200" json:"timeout_seconds"`
	Enabled        bool   `gorm:"default:true" json:"enabled"`
	CreatedBy      int64  `gorm:"not null;default:0;index:idx_cicd_pipelines_created_by" json:"created_by"`
	CreatedByName  string `gorm:"type:varchar(64)" json:"created_by_name"`
}

// CICDRun 一次流水线运行记录，集群事实源为 Tekton PipelineRun，本表做索引与审计。
type CICDRun struct {
	pixiu.Model

	PipelineId     int64           `gorm:"not null;index:idx_cicd_runs_pipeline_id,priority:1" json:"pipeline_id"`
	TenantId       int64           `gorm:"not null;default:0;index:idx_cicd_runs_tenant_id" json:"tenant_id"`
	ClusterId      int64           `gorm:"not null;default:0" json:"cluster_id"`
	Namespace      string          `gorm:"type:varchar(128)" json:"namespace"`
	TriggerType    CICDTriggerType `gorm:"type:varchar(16);not null;default:'manual'" json:"trigger_type"`
	Status         CICDRunStatus   `gorm:"type:varchar(16);not null;index:idx_cicd_runs_status" json:"status"`
	Engine         string          `gorm:"type:varchar(16);not null;default:'tekton'" json:"engine"`
	EngineName     string          `gorm:"type:varchar(191);not null;index:idx_cicd_runs_engine_name" json:"engine_name"`
	GitRef         string          `gorm:"type:varchar(191)" json:"git_ref"`
	CommitSha      string          `gorm:"type:varchar(64)" json:"commit_sha"`
	ImageRepo      string          `gorm:"type:varchar(255)" json:"image_repo"`
	ImageTag       string          `gorm:"type:varchar(128)" json:"image_tag"`
	ImageDigest    string          `gorm:"type:varchar(128)" json:"image_digest"`
	Params         string          `gorm:"type:longtext" json:"params"`
	Message        string          `gorm:"type:text" json:"message"`
	LogKey         string          `gorm:"type:varchar(255)" json:"log_key"`
	StartedAt      *time.Time      `gorm:"column:started_at;type:datetime" json:"started_at"`
	FinishedAt     *time.Time      `gorm:"column:finished_at;type:datetime" json:"finished_at"`
	DurationMs     int64           `gorm:"default:0" json:"duration_ms"`
	CreatedBy      int64           `gorm:"not null;default:0;index:idx_cicd_runs_created_by" json:"created_by"`
	CreatedByName  string          `gorm:"type:varchar(64)" json:"created_by_name"`
}

// CICDTrigger 定时 / Webhook 触发器（设计第 6 节，P2 预留；表结构先行，MVP 不启用）。
type CICDTrigger struct {
	pixiu.Model

	PipelineId      int64  `gorm:"not null;index:idx_cicd_triggers_pipeline_id" json:"pipeline_id"`
	Type            string `gorm:"type:varchar(16);not null" json:"type"`
	SecretHash      string `gorm:"type:varchar(128)" json:"secret_hash"`
	Config          string `gorm:"type:longtext" json:"config"`
	Enabled         bool   `gorm:"default:true" json:"enabled"`
	LastTriggeredAt *time.Time `json:"last_triggered_at"`
}
