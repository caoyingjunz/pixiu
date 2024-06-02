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
	"sync"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model"

	"github.com/gorilla/websocket"
	appv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/remotecommand"
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

type Cluster struct {
	PixiuMeta `json:",inline"`

	Name      string `json:"name"`
	AliasName string `json:"alias_name"`

	// 0: 标准集群 1: 自建集群
	ClusterType model.ClusterType `json:"cluster_type"`

	// 集群删除保护，开启集群删除保护时不允许删除集群
	// 0: 关闭集群删除保护 1: 开启集群删除保护
	Protected bool `json:"protected"`

	// k8s kubeConfig base64 字段
	KubeConfig string `json:"kube_config,omitempty"`

	// 集群用途描述，可以为空
	Description string `json:"description"`

	KubernetesMeta `json:",inline"`
	TimeMeta       `json:",inline"`
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

	Name        string           `json:"name"`                                 // 用户名称
	Password    string           `json:"password" binding:"required,password"` // 用户密码
	Status      model.UserStatus `json:"status"`                               // 用户状态标识
	Role        model.UserRole   `json:"role"`                                 // 用户角色，目前只实现管理员，0: 普通用户 1: 管理员 2: 超级管理员
	Email       string           `json:"email"`                                // 用户注册邮件
	Description string           `json:"description"`                          // 用户描述信息

	TimeMeta `json:",inline"`
}

type Tenant struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Name        string `json:"name"`        // 用户名称
	Description string `json:"description"` // 用户描述信息
}

type Plan struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Name        string         `json:"name"` // 用户名称
	Step        model.PlanStep `json:"step"`
	Description string         `json:"description"` // 用户描述信息
}

type PlanNode struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Name   string         `json:"name"` // required
	PlanId int64          `json:"plan_id"`
	Role   model.KubeRole `json:"role"` // k8s 节点的角色，master 为 1 和 node 为 0
	Ip     string         `json:"ip"`
	Auth   PlanNodeAuth   `json:"auth,omitempty"`
}

type AuthType string

const (
	NoneAuth     AuthType = "none"     // 已开启密码
	KeyAuth      AuthType = "key"      // 密钥
	PasswordAuth AuthType = "password" // 密码
)

type PlanNodeAuth struct {
	Type     AuthType      `json:"type"` // 节点认证模式，支持 key 和 password
	Key      *KeySpec      `json:"key,omitempty"`
	Password *PasswordSpec `json:"password,omitempty"`
}

type KeySpec struct {
	Data string `json:"data,omitempty"`
}

type PasswordSpec struct {
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
}

type PlanConfig struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	PlanId      int64          `json:"plan_id" binding:"required"` // required
	Name        string         `json:"name"  binding:"required"`   // required
	Region      string         `json:"region"`
	Description string         `json:"description"` // optional
	Kubernetes  KubernetesSpec `json:"kubernetes"`
	Network     NetworkSpec    `json:"network"`
	Runtime     RuntimeSpec    `json:"runtime"`
}

// TimeSpec 通用时间规格
type TimeSpec struct {
	GmtCreate   interface{} `json:"gmt_create,omitempty"`
	GmtModified interface{} `json:"gmt_modified,omitempty"`
}

type KubeObject struct {
	lock sync.RWMutex

	ReplicaSets []appv1.ReplicaSet
	Pods        []v1.Pod
}

// WebShellOptions ws API 参数定义
type WebShellOptions struct {
	Cluster   string `form:"cluster"`
	Namespace string `form:"namespace"`
	Pod       string `form:"pod"`
	Container string `form:"container"`
	Command   string `form:"command"`
}

// TerminalMessage 定义了终端和容器 shell 交互内容的格式 Operation 是操作类型
// Data 是具体数据内容 Rows和Cols 可以理解为终端的行数和列数，也就是宽、高
type TerminalMessage struct {
	Operation string `json:"operation"`
	Data      string `json:"data"`
	Rows      uint16 `json:"rows"`
	Cols      uint16 `json:"cols"`
}

// TerminalSession 定义 TerminalSession 结构体，实现 PtyHandler 接口 // wsConn 是 websocket 连接 // sizeChan 用来定义终端输入和输出的宽和高 // doneChan 用于标记退出终端
type TerminalSession struct {
	wsConn   *websocket.Conn
	sizeChan chan remotecommand.TerminalSize
	doneChan chan struct{}
}

// ListOptions is the query options to a standard REST list call.
type ListOptions struct {
	Count bool `form:"count"`
}

type EventOptions struct {
	Uid        string `form:"uid"`
	Namespace  string `form:"namespace"`
	Name       string `form:"name"`
	Kind       string `form:"kind"`
	Namespaced bool   `form:"namespaced"`
	Limit      int64  `form:"limit"`
}

type KubernetesSpec struct {
	ApiServer         string `json:"api_server"`
	KubernetesVersion string `json:"kubernetes_version"`
	EnableHA          bool   `json:"enable_ha"`
}

type NetworkSpec struct {
	Cni            string `json:"cni"`
	PodNetwork     string `json:"pod_network"`
	ServiceNetwork string `json:"service_network"`
	KubeProxy      string `json:"kube_proxy"`
}

type RuntimeSpec struct {
	Runtime string `json:"runtime"`
}
