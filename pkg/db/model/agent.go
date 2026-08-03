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
	register(&Agent{}, &Job{})
}

// PlanExecMode 部署计划执行模式
type PlanExecMode string

const (
	// PlanExecModeLocal Pixiu 本机 Docker + 出站 SSH（默认）
	PlanExecModeLocal PlanExecMode = "local"
	// PlanExecModeAgent 单向网络：边缘 Deploy Agent 拉任务执行
	PlanExecModeAgent PlanExecMode = "agent"
)

type AgentStatus = DeployAgentStatus

// DeployAgentStatus Deploy Agent 在线状态
type DeployAgentStatus uint8

const (
	DeployAgentStatusOffline DeployAgentStatus = 0
	DeployAgentStatusOnline  DeployAgentStatus = 1
)

const (
	AgentStatusOffline = DeployAgentStatusOffline
	AgentStatusOnline  = DeployAgentStatusOnline
)

// AgentType 代理类型
type AgentType uint8

const (
	AgentTypeDeploy  AgentType = iota // 部署代理
	AgentTypeCluster                  // 集群代理
)

type Agent struct {
	pixiu.Model

	Name          string            `gorm:"index:idx_deploy_agent_name,unique;type:varchar(255)" json:"name"`
	AgentType     AgentType         `gorm:"type:smallint;default:0" json:"agent_type"`
	UserID        int64             `gorm:"column:user_id;type:bigint;default:0" json:"user_id"`
	Status        DeployAgentStatus `gorm:"type:smallint;default:0" json:"status"`
	Hostname      string            `gorm:"type:varchar(255)" json:"hostname"`
	Version       string            `gorm:"type:varchar(64)" json:"version"`
	LastHeartbeat time.Time         `gorm:"type:timestamp" json:"last_heartbeat"`
	Description   string            `gorm:"type:text" json:"description"`

	Token string `gorm:"type:varchar(128);index:idx_deploy_agent_token" json:"-"`
}

func (Agent) TableName() string { return "agents" }

// JobStatus 作业状态
type JobStatus string

const (
	JobPending JobStatus = "pending"
	JobRunning JobStatus = "running"
	JobSuccess JobStatus = "success"
	JobFailed  JobStatus = "failed"
)

// JobKind 作业类型
type JobKind string

const (
	JobRunContainer    JobKind = "run_container"
	JobPullImage       JobKind = "pull_image"
	JobRenderConfig    JobKind = "render_config"
	JobFetchKubeconfig JobKind = "fetch_kubeconfig"
)

// Job 可被 Deploy Agent claim 的作业
type Job struct {
	pixiu.Model

	PlanId    int64      `gorm:"index:idx_deploy_job_plan" json:"plan_id"`
	AgentId   int64      `gorm:"index:idx_deploy_job_agent" json:"agent_id"`
	TaskName  string     `gorm:"type:varchar(128)" json:"task_name"`
	Kind      JobKind    `gorm:"type:varchar(64)" json:"kind"`
	Action    string     `gorm:"type:varchar(128)" json:"action"` // kubez-ansible COMMAND
	Image     string     `gorm:"type:varchar(512)" json:"image"`
	Payload   string     `gorm:"type:text" json:"payload"` // JSON 附加数据
	Status    JobStatus  `gorm:"type:varchar(32);index:idx_deploy_job_status" json:"status"`
	Message   string     `gorm:"type:text" json:"message"`
	Logs      string     `gorm:"type:text" json:"logs"`
	Result    string     `gorm:"type:text" json:"result"` // 如 base64 kubeconfig
	ClaimedAt *time.Time `gorm:"type:timestamp" json:"claimed_at"`
}

func (Job) TableName() string { return "jobs" }
