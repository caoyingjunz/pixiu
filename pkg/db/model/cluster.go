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

import "github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"

func init() {
	register(&Cluster{})
}

// ClusterType Kubernetes 集群的类型
type ClusterType uint8

const (
	ClusterTypeStandard ClusterType = iota // 标准集群
	ClusterTypeCustom                      // 自建集群
)

type ClusterStatus uint8

const (
	ClusterStatusRunning ClusterStatus = iota // 运行中
	ClusterStatusDeploy                       // 部署中
	ClusterStatusUnStart                      // 等待部署
	ClusterStatusFailed                       // 部署失败
	ClusterStatusError                        // 集群失联，API不可用
)

// Cluster kubernetes 集群信息
type Cluster struct {
	pixiu.Model

	// 集群名称，全局唯一
	Name string `gorm:"index:idx_name,unique" json:"name"`
	// 集群别名，可以重复，允许为中文
	AliasName string `json:"alias_name"`

	// 0：标准集群 1: 自建集群
	ClusterType `gorm:"type:tinyint" json:"cluster_type"`
	// 自建集群关联的 PlanId
	PlanId int64

	// 集群运行状态 0: 运行中 1: 部署中 2: 等待部署 3: 部署失败 4: 运行中断 5: 所有的 node 不健康
	ClusterStatus `gorm:"column:status;type:tinyint" json:"status"`

	// 集群的版本
	KubernetesVersion string `gorm:"type:varchar(255)" json:"kubernetes_version,omitempty"`

	// 集群节点健康数，json 字符串
	Nodes string `gorm:"type:text" json:"nodes"`

	// 集群删除保护，开启集群删除保护时不允许删除集群
	// 0: 关闭集群删除保护 1: 开启集群删除保护
	Protected bool `json:"protected"`

	// k8s kubeConfig base64 字段
	KubeConfig string `json:"kube_config"`

	// 集群用途描述，可以为空
	Description string `gorm:"type:text" json:"description"`
}

func (*Cluster) TableName() string {
	return "clusters"
}
