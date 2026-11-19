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

// ClusterExpansionCapabilities 集群扩容能力探测结果与默认参数。
type ClusterExpansionCapabilities struct {
	Supported bool   `json:"supported"`
	Reason    string `json:"reason,omitempty"`

	KubernetesVersion       string `json:"kubernetes_version,omitempty"`
	ImageRepository         string `json:"image_repository,omitempty"`
	ControlPlaneEndpoint    string `json:"control_plane_endpoint,omitempty"`
	DefaultOSImage          string `json:"default_os_image,omitempty"`
	DefaultNetworkInterface string `json:"default_network_interface,omitempty"`
}

// ClusterExpansionNodeSummary 扩容节点摘要（不含凭据），供节点管理页使用。
type ClusterExpansionNodeSummary struct {
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

// BatchOperationItem 批量操作成功/跳过的对象。
type BatchOperationItem struct {
	Key  int64  `json:"key"`
	Name string `json:"name,omitempty"`
}

// BatchOperationFailure 批量操作失败的明细。
type BatchOperationFailure struct {
	Key    int64  `json:"key"`
	Name   string `json:"name,omitempty"`
	Reason string `json:"reason"`
}

// BatchOperationResult 批量操作结果：单条失败隔离，不整体报错。
type BatchOperationResult struct {
	Succeeded []BatchOperationItem    `json:"succeeded"`
	Skipped   []BatchOperationItem    `json:"skipped"`
	Failed    []BatchOperationFailure `json:"failed"`
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

// ExpansionJoinParams 节点加入参数，对应 POST /pixiu/clusters/:clusterId/expansion-join-params，
// 供扩容页面「自动生成」按钮一键填充。
type ExpansionJoinParams struct {
	// APIServer 新节点可达的 kube-apiserver 地址，格式 host:port
	APIServer string `json:"api_server"`
	// Token kubeadm bootstrap token，格式 id.secret
	Token string `json:"token"`
	// CAHash CA 证书公钥的 sha256（纯 hex，不含 sha256: 前缀）
	CAHash string `json:"ca_cert_hash"`
	// ExpiresAt token 过期时间（TTL 2 小时）
	ExpiresAt time.Time `json:"expires_at"`
}

// CreateOrUpdateClusterExpansionPlanRequest 创建或更新集群扩容计划（plans.type=1）的配置、元数据与节点集合。
// 对应 POST /pixiu/clusters/:clusterId/expansion-plan，属于计划单例的全量保存。
// 计划名称由系统生成（expand-cluster-<id>），请求不接收名称。
type CreateOrUpdateClusterExpansionPlanRequest struct {
	Config      CreatePlanConfigRequest `json:"config"`
	NodeIds     []int64                 `json:"node_ids"`
	Description string                  `json:"description"`
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
	// Status 计划最近一次扩容的执行状态。
	Status model.TaskStatus `json:"status"`
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
