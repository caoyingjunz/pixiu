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
	"github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"
)

func init() {
	register(&Plan{}, &Node{}, &Config{}, &Task{})
}

type Plan struct {
	pixiu.Model

	Name   string     `gorm:"index:idx_name,unique" json:"name"`
	Status TaskStatus `gorm:"type:varchar(32);default:'未开始';index:idx_plan_status" json:"status"` // 部署状态

	// 所属用户
	UserId      int64  `gorm:"index:idx_user_id" json:"user_id"`
	Description string `gorm:"type:text" json:"description"`

	// 执行模式：local（默认）/ agent（单向网络边缘执行）
	ExecMode PlanExecMode `gorm:"type:varchar(32);default:'local'" json:"exec_mode"`
	// agent 模式下绑定的 Deploy Agent
	DeployAgentId     int64  `gorm:"index:idx_plan_deploy_agent;default:0" json:"deploy_agent_id"`
	KubernetesVersion string `gorm:"type:varchar(64)" json:"kubernetes_version"` // k8s 版本（冗余自 configs.kubernetes）
	NodeCount         int    `gorm:"default:0" json:"node_count"`                // 节点总数（冗余自 nodes 表）

	// Type 计划类型：0=普通部署计划（默认），1=集群扩容计划（添加节点时系统自动创建）
	Type int `gorm:"default:0;index:idx_plan_type" json:"type"`
	// ClusterId 扩容计划（Type=1）关联的目标集群
	ClusterId int64 `gorm:"index:idx_plan_cluster_id;default:0" json:"cluster_id"`
}

func (plan *Plan) TableName() string {
	return "plans"
}

type KubeRole string

const (
	MasterRole  string = "master" // kubernetes master role
	NodeRole    string = "node"   // kubernetes node role
	StorageRole string = "storage"
)

type CRI string

const (
	DockerCRI     CRI = "docker"
	ContainerdCRI CRI = "containerd"
)

// NodeStatus 主机节点状态（集群扩容场景）
type NodeStatus uint8

const (
	NodeStatusUnadded NodeStatus = iota // 未添加
	NodeStatusAdding                    // 添加中
	NodeStatusFailed                    // 添加失败
	NodeStatusAdded                     // 已加入
)

type Node struct {
	pixiu.Model

	Name   string `json:"name"`                             // 主机名，相同plan内不允许重复
	UserId int64  `gorm:"index:idx_user_id" json:"user_id"` // 所属用户
	PlanId int64  `json:"plan_id"`
	// ClusterId 直接关联的集群（扩容场景）。
	// 普通计划部署的节点只有 PlanId；集群扩容的节点同时写入扩容计划的 PlanId 与 ClusterId。
	ClusterId int64  `gorm:"index:idx_node_cluster_id;default:0" json:"cluster_id"`
	Role      string `json:"role"` // k8s 节点的角色，master 和 node
	CRI       CRI    `json:"cri"`
	Ip        string `json:"ip"`
	Auth      string `json:"auth"`

	// Status 仅对 ClusterId > 0 的扩容节点有意义
	Status        NodeStatus `gorm:"type:tinyint;default:0;index:idx_node_status" json:"status"`
	StatusMessage string     `gorm:"type:text" json:"status_message"`
}

func (node *Node) TableName() string {
	return "nodes"
}

type Config struct {
	pixiu.Model

	PlanId     int64  `json:"plan_id"`
	Region     string `json:"region"`
	OSImage    string `json:"os_image"`
	Kubernetes string `json:"kubernetes"`
	Network    string `json:"network"`
	Runtime    string `json:"runtime"`
	Component  string `json:"component"`
}

func (config *Config) TableName() string {
	return "configs"
}

type PlanStep int

const (
	UnStartedPlanStep PlanStep = iota
	RunningPlanStep
	FailedPlanStep
	CompletedPlanStep
)

type TaskStatus string

const (
	FailedPlanStatus     TaskStatus = "已失败"
	SuccessPlanStatus    TaskStatus = "已成功"
	UnStartPlanStatus    TaskStatus = "未开始"
	RunningPlanStatus    TaskStatus = "运行中"
	StoppingPlanStatus   TaskStatus = "停止中"
	DestroyingPlanStatus TaskStatus = "销毁中"
)

type Task struct {
	pixiu.Model

	Name    string     `json:"name"`
	PlanId  int64      `json:"plan_id"`
	Step    PlanStep   `json:"step"`
	Action  string     `json:"action"`
	Status  TaskStatus `json:"status"`
	Message string     `json:"message"`
}

func (task *Task) TableName() string {
	return "tasks"
}
