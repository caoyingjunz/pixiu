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
	register(&DeployAgent{}, &DeployJob{})
}

// PlanExecMode 部署计划执行模式
type PlanExecMode string

const (
	// PlanExecModeLocal Pixiu 本机 Docker + 出站 SSH（默认）
	PlanExecModeLocal PlanExecMode = "local"
	// PlanExecModeAgent 单向网络：边缘 Deploy Agent 拉任务执行
	PlanExecModeAgent PlanExecMode = "agent"
)

// DeployAgentStatus Deploy Agent 在线状态
type DeployAgentStatus uint8

const (
	DeployAgentStatusOffline DeployAgentStatus = 0
	DeployAgentStatusOnline  DeployAgentStatus = 1
)

// DeployAgent 单向装集群的边缘执行器（HTTPS 出站拉作业）
type DeployAgent struct {
	pixiu.Model

	Name          string            `gorm:"index:idx_deploy_agent_name,unique;type:varchar(255)" json:"name"`
	Token         string            `gorm:"type:varchar(128);index:idx_deploy_agent_token" json:"-"`
	Status        DeployAgentStatus `gorm:"type:tinyint;default:0" json:"status"`
	Hostname      string            `gorm:"type:varchar(255)" json:"hostname"`
	Version       string            `gorm:"type:varchar(64)" json:"version"`
	LastHeartbeat time.Time         `gorm:"type:datetime" json:"last_heartbeat"`
	Description   string            `gorm:"type:text" json:"description"`
}

func (DeployAgent) TableName() string { return "deploy_agents" }

// DeployJobStatus 作业状态
type DeployJobStatus string

const (
	DeployJobPending DeployJobStatus = "pending"
	DeployJobRunning DeployJobStatus = "running"
	DeployJobSuccess DeployJobStatus = "success"
	DeployJobFailed  DeployJobStatus = "failed"
)

// DeployJobKind 作业类型
type DeployJobKind string

const (
	DeployJobRunContainer    DeployJobKind = "run_container"
	DeployJobPullImage       DeployJobKind = "pull_image"
	DeployJobFetchKubeconfig DeployJobKind = "fetch_kubeconfig"
)

// DeployJob 可被 Deploy Agent claim 的作业
type DeployJob struct {
	pixiu.Model

	PlanId    int64           `gorm:"index:idx_deploy_job_plan" json:"plan_id"`
	AgentId   int64           `gorm:"index:idx_deploy_job_agent" json:"agent_id"`
	TaskName  string          `gorm:"type:varchar(128)" json:"task_name"`
	Kind      DeployJobKind   `gorm:"type:varchar(64)" json:"kind"`
	Action    string          `gorm:"type:varchar(128)" json:"action"` // kubez-ansible COMMAND
	Image     string          `gorm:"type:varchar(512)" json:"image"`
	Payload   string          `gorm:"type:mediumtext" json:"payload"` // JSON 附加数据
	Status    DeployJobStatus `gorm:"type:varchar(32);index:idx_deploy_job_status" json:"status"`
	Message   string          `gorm:"type:text" json:"message"`
	Logs      string          `gorm:"type:longtext" json:"logs"`
	Result    string          `gorm:"type:mediumtext" json:"result"` // 如 base64 kubeconfig
	ClaimedAt *time.Time      `gorm:"type:datetime" json:"claimed_at"`
}

func (DeployJob) TableName() string { return "deploy_jobs" }
