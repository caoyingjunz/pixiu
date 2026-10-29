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
	SkipBootstrap    bool        `json:"skip_bootstrap"`
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

// ClusterExpansion 扩容记录 API 返回结构。
type ClusterExpansion struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	ClusterId int64            `json:"cluster_id"`
	NodeIds   []int64          `json:"node_ids"`
	Options   ExpansionOptions `json:"options"`

	Status      model.TaskStatus `json:"status"`
	Step        string           `json:"step"`
	Message     string           `json:"message"`
	StartedAt   *time.Time       `json:"started_at,omitempty"`
	CompletedAt *time.Time       `json:"completed_at,omitempty"`
}
