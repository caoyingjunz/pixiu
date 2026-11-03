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

import (
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

// ClusterExpansionConfig 集群扩容能力探测结果。
type ClusterExpansionConfig struct {
	Supported bool   `json:"supported"`
	Reason    string `json:"reason,omitempty"`

	KubernetesVersion       string `json:"kubernetes_version,omitempty"`
	ImageRepository         string `json:"image_repository,omitempty"`
	ControlPlaneEndpoint    string `json:"control_plane_endpoint,omitempty"`
	Runtime                 string `json:"runtime,omitempty"`
	DefaultOSImage          string `json:"default_os_image,omitempty"`
	DefaultNetworkInterface string `json:"default_network_interface,omitempty"`
}

// ClusterNodeSummary 扩容节点摘要（不含凭据），供节点管理页使用。
type ClusterNodeSummary struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Name          string           `json:"name"`
	Ip            string           `json:"ip"`
	Role          string           `json:"role"`
	CRI           model.CRI        `json:"cri"`
	Status        model.NodeStatus `json:"status"`
	StatusMessage string           `json:"status_message,omitempty"`
}

// ClusterNodeSpec 手动新增的扩容节点。
type ClusterNodeSpec struct {
	Name string       `json:"name" binding:"required"`
	Ip   string       `json:"ip" binding:"required"`
	Auth PlanNodeAuth `json:"auth" binding:"required"`
}

// CreateClusterNodesRequest 添加节点到集群：node_ids 关联主机库，nodes 直接创建。
type CreateClusterNodesRequest struct {
	NodeIds []int64           `json:"node_ids"`
	Nodes   []ClusterNodeSpec `json:"nodes"`
}

// ExpansionOptions 扩容执行参数。
type ExpansionOptions struct {
	OSImage          string      `json:"os_image,omitempty"`
	NetworkInterface string      `json:"network_interface,omitempty"`
	ImageRepository  string      `json:"image_repository,omitempty"`
	SetHostname      bool        `json:"set_hostname"`
	RuntimeDataDir   string      `json:"runtime_data_dir,omitempty"`
	CustomRepo       *CustomRepo `json:"custom_repo,omitempty"`

	// join 参数覆盖项：集群 kubeconfig 权限不足以自动创建 token 时使用。
	APIServer string `json:"api_server,omitempty"`
	Token     string `json:"token,omitempty"`
	CAHash    string `json:"ca_cert_hash,omitempty"`
}

// CreateClusterExpansionRequest 创建并启动一次集群扩容。
type CreateClusterExpansionRequest struct {
	NodeIds []int64          `json:"node_ids" binding:"required,min=1"`
	Options ExpansionOptions `json:"options"`
}

// SaveClusterExpansionPlanConfigRequest 保存集群扩容计划（plans.type=1）的配置与元数据。
type SaveClusterExpansionPlanConfigRequest struct {
	// Name 保存扩容计划时忽略名称，避免与部署计划名称唯一索引冲突（计划名称由系统生成）。
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Config      CreatePlanConfigRequest `json:"config"`
	// NodeIds 本次勾选的扩容节点：仅这些节点关联到该计划，未勾选节点解绑。
	NodeIds []int64 `json:"node_ids"`
	// join 参数位于 config.kubernetes 内（KubernetesSpec.bootstrap_token / ca_hash）。
}

// ClusterExpansionPlanResponse 集群扩容计划（plans.type=1）及配置的查询结果。
// 计划不存在时 Exists=false，其余字段省略。
type ClusterExpansionPlanResponse struct {
	Exists      bool                         `json:"exists"`
	PlanId      int64                        `json:"plan_id,omitempty"`
	Name        string                       `json:"name,omitempty"`
	Description string                       `json:"description,omitempty"`
	Config      *ExpansionPlanConfigResponse `json:"config,omitempty"`
	// NodeIds 该计划当前关联的扩容节点。
	NodeIds []int64 `json:"node_ids"`
}

// ExpansionPlanConfigResponse 集群扩容计划配置（configs 表解析结果）。
type ExpansionPlanConfigResponse struct {
	OSImage     string         `json:"os_image"`
	Description string         `json:"description"`
	Kubernetes  KubernetesSpec `json:"kubernetes"`
	Network     NetworkSpec    `json:"network"`
	Runtime     RuntimeSpec    `json:"runtime"`
	Component   ComponentSpec  `json:"component"`
	// join 参数随 Kubernetes 字段返回（kubernetes.bootstrap_token / ca_hash）。
}

// ClusterExpansion 扩容记录 API 返回结构。
type ClusterExpansion struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	ClusterId int64            `json:"cluster_id"`
	PlanId    int64            `json:"plan_id"`
	NodeIds   []int64          `json:"node_ids"`
	Options   ExpansionOptions `json:"options"`

	Status      model.TaskStatus `json:"status"`
	Step        string           `json:"step"`
	Message     string           `json:"message"`
	StartedAt   *time.Time       `json:"started_at,omitempty"`
	CompletedAt *time.Time       `json:"completed_at,omitempty"`
}
