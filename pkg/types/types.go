/*
Copyright 2021 The Pixiu Authors.

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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PixiuMeta struct {
	// pixiu 对象 ID
	Id int64 `json:"id"`
	// Pixiu 对象版本号
	ResourceVersion int64 `json:"resource_version"`
}

type TimeMeta struct {
	// pixiu 对象创建时间
	GmtCreate time.Time `json:"gmt_create"`
	// pixiu 对象修改时间
	GmtModified time.Time `json:"gmt_modified"`
}

// ClusterType Kubernetes 集群的类型
// 0：标准集群 1: 自建集群
type ClusterType int

type Cluster struct {
	PixiuMeta `json:",inline"`

	Name      string `json:"name"`
	AliasName string `json:"alias_name"`

	// 0：标准集群 1: 自建集群
	ClusterType ClusterType `json:"cluster_type"`
	// k8s kubeConfig base64 字段
	KubeConfig string `json:"kube_config,omitempty"`

	KubernetesMeta `json:",inline"`

	// 集群用途描述，可以为空
	Description string `json:"description"`

	TimeMeta `json:",inline"`
}

// KubernetesMeta 记录 kubernetes 集群的数据
type KubernetesMeta struct {
	// 集群的版本
	KubernetesVersion string `json:"kubernetes_version,omitempty"`
	// 节点数量
	Nodes int `json:"nodes"`
	// The memory and cpu usage
	Resources Resources `json:"resources"`
}

// Resources kubernetes 的资源信息
// The memory and cpu usage
type Resources struct {
	Cpu    string `json:"cpu"`
	Memory string `json:"memory"`
}

type User struct {
	PixiuMeta `json:",inline"`

	Name        string `json:"name"`               // 用户名称
	Password    string `json:"password,omitempty"` // 用户密码
	Status      int8   `json:"status"`             // 用户状态标识
	Role        string `json:"role"`               // 用户角色，目前只实现管理员，0: 普通用户 1: 管理员 2: 超级管理员
	Email       string `json:"email"`              // 用户注册邮件
	Description string `json:"description"`        // 用户描述信息

	TimeMeta `json:",inline"`
}

type Event struct {
	Type          string      `json:"type"`
	Reason        string      `json:"reason"`
	ObjectName    string      `json:"objectName"`
	Kind          string      `json:"kind"`
	Message       string      `json:"message"`
	LastTimestamp metav1.Time `json:"lastTimestamp,omitempty"`
}

type EventList []Event

func (e EventList) Len() int           { return len(e) }
func (e EventList) Less(i, j int) bool { return e[i].LastTimestamp.After(e[j].LastTimestamp.Time) }
func (e EventList) Swap(i, j int)      { e[i], e[j] = e[j], e[i] }
