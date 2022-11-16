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
	"github.com/gorilla/websocket"
	"k8s.io/client-go/tools/remotecommand"
)

type IdOptions struct {
	Id int64 `uri:"id" binding:"required"`
}

type CloudOptions struct {
	CloudName string `uri:"cloud_name" binding:"required"`
}

type ObjectOptions struct {
	ObjectName string `uri:"object_name" binding:"required"`
}

// NodeOptions todo: 后续整合优化
type NodeOptions struct {
	CloudOptions `json:",inline"`

	ObjectOptions `json:",inline"`
}

type ListOptions struct {
	CloudName string `uri:"cloud_name" binding:"required"`
	Namespace string `uri:"namespace" binding:"required"`
}

type GetOrDeleteOptions struct {
	ListOptions `json:",inline"`

	ObjectName string `uri:"object_name" binding:"required"`
}

type GetOrCreateOptions struct {
	ListOptions `json:",inline,omitempty"`

	ObjectName string `uri:"object_name" binding:"required"`
}

type CreateOptions struct {
	ListOptions `json:",inline,omitempty"`
}

// LogsOptions 日志
type LogsOptions struct {
	GetOrCreateOptions
	ContainerName string `form:"container_name"`
}

type Git struct {
	GitUrl        string `json:"gitUrl,omitempty"`
	Branch        string `json:"branch,omitempty"`
	CredentialsId string `json:"credentialsId,omitempty"`
	ScriptPath    string `json:"scriptPath,omitempty"`
}

type Cicd struct {
	Name     string `json:"name,omitempty"`
	OldName  string `json:"oldName,omitempty"`
	NewName  string `json:"newName,omitempty"`
	ViewName string `json:"viewname,omitempty"`
	Version  string `json:"version,omitempty"`
	Type     string `json:"type,omitempty"`

	Git
}

type User struct {
	Id              int64  `json:"id"`
	ResourceVersion int64  `json:"resource_version"`
	Name            string `json:"name"`
	Password        string `json:"password"`
	Status          int8   `json:"status"`
	Role            string `json:"role"`
	Email           string `json:"email"`
	Description     string `json:"description"`

	TimeOption `json:",inline"`
}

type Password struct {
	UserId          int64  `json:"user_id"`
	OriginPassword  string `json:"origin_password"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

type Cloud struct {
	IdMeta     `json:",inline"`
	TimeOption `json:",inline"`

	Name        string `json:"name"`
	AliasName   string `json:"alias_name"`
	Status      int    `json:"status"`     // 0: 运行中 1: 集群异常 2: 构建中 3: 删除中 4: 等待构建
	CloudType   int    `json:"cloud_type"` // 1：导入集群（前端又名标准集群） 2: 自建集群
	KubeVersion string `json:"kube_version"`
	KubeConfig  []byte `json:"kube_config"`
	NodeNumber  int    `json:"node_number"`
	Resources   string `json:"resources"`
	Description string `json:"description"`
}

// BuildCloud 自建 kubernetes 属性
type BuildCloud struct {
	Name            string          `json:"name"`       // 名称，系统自动生成，只能为字符串
	AliasName       string          `json:"alias_name"` // 可读性的名称，支持中午
	Immediate       bool            `json:"immediate"`  // 立刻部署
	CloudType       int             `json:"cloud_type"` // cloud 的类型，支持标准类型和自建类型
	Region          string          `json:"region"`     // 城市区域
	Kubernetes      *KubernetesSpec `json:"kubernetes"` // k8s 全部信息
	CreateNamespace bool            `json:"create_namespace"`
	Description     string          `json:"description"`
}

// NodeSpec 构造 kubernetes 集群的节点
type NodeSpec struct {
	HostName string `json:"host_name"`
	Address  string `json:"address"`
	User     string `json:"user"`
	Password string `json:"password"`
}

type KubernetesSpec struct {
	ApiServer   string     `json:"api_server"` // kubernetes 的 apiServer 的 ip 地址
	Version     string     `json:"version"`    // k8s 的版本
	Runtime     string     `json:"runtime"`    // 容器运行时，目前支持 docker 和 containerd
	Cni         string     `json:"cni"`        // 网络 cni，支持 flannel 和 calico
	ServiceCidr string     `json:"service_cidr"`
	PodCidr     string     `json:"pod_cidr"`
	ProxyMode   string     `json:"proxy_mode"` // kubeProxy 的模式，只能是 iptables 和 ipvs
	Masters     []NodeSpec `json:"masters"`    // 集群的 master 节点
	Nodes       []NodeSpec `json:"nodes"`      // 集群的 node 节点
}

// Node k8s node属性
type Node struct {
	Name             string `json:"name"`
	Status           string `json:"status"`
	Roles            string `json:"roles"`
	CreateAt         string `json:"create_at"`
	Version          string `json:"version"`
	InternalIP       string `json:"internal_ip"`
	OsImage          string `json:"osImage"`
	KernelVersion    string `json:"kernel_version"`
	ContainerRuntime string `json:"container_runtime"`
}

type KubeConfigOptions struct {
	Id                  int64  `json:"id"`
	CloudName           string `json:"cloud_name"`
	ServiceAccount      string `json:"service_account"`
	ClusterRole         string `json:"cluster_role"`
	Config              string `json:"config"`
	ExpirationTimestamp string `json:"expiration_timestamp"`
}

// WebShellOptions ws API 参数定义
type WebShellOptions struct {
	CloudName string `form:"cloud"` // 需要连接的 k8s 唯一名称
	Namespace string `form:"namespace"`
	Pod       string `form:"pod"`
	Container string `form:"container"`
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
