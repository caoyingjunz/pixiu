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
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/caoyingjunz/gopixiu/pkg/log"
)

// TerminalMessage 定义了终端和容器 shell 交互内容的格式 Operation 是操作类型
// Data 是具体数据内容 Rows和Cols 可以理解为终端的行数和列数，也就是宽、高

type TerminalMessage struct {
	Operation string `json:"operation"`
	Data      string `json:"data"`
	Rows      uint16 `json:"rows"`
	Cols      uint16 `json:"cols"`
}

// 初始化 Upgrader 类型的对象，用于http协议升级为 websocket 协议

var upgrader = func() websocket.Upgrader {
	upgrader := websocket.Upgrader{}
	upgrader.HandshakeTimeout = time.Second * 2
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}
	return upgrader
}()

// TerminalSession 定义 TerminalSession 结构体，实现 PtyHandler 接口 // wsConn 是 websocket 连接 // sizeChan 用来定义终端输入和输出的宽和高 // doneChan 用于标记退出终端
type TerminalSession struct {
	wsConn   *websocket.Conn
	sizeChan chan remotecommand.TerminalSize
	doneChan chan struct{}
}

// NewTerminalSession 该方法用于升级 http 协议至 websocket，并new一个 TerminalSession 类型的对象返回
func NewTerminalSession(w http.ResponseWriter, r *http.Request) (*TerminalSession, error) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	session := &TerminalSession{
		wsConn:   conn,
		sizeChan: make(chan remotecommand.TerminalSize),
		doneChan: make(chan struct{}),
	}

	return session, nil
}

// 用于读取web端的输入，接收web端输入的指令内容
func (t *TerminalSession) Read(p []byte) (int, error) {
	_, message, err := t.wsConn.ReadMessage()
	if err != nil {
		log.Logger.Error(errors.New("Failed to read information," + err.Error()))
		return copy(p, "\u0004"), err
	}
	// 反序列化
	var msg TerminalMessage
	if err = json.Unmarshal(message, &msg); err != nil {
		log.Logger.Error(errors.New("json decoding failure," + err.Error()))
		return copy(p, "\u0004"), err
	}
	// 逻辑判断
	switch msg.Operation {
	// 如果是标准输入
	case "stdin":
		return copy(p, msg.Data), nil
	// 窗口调整大小
	case "resize":
		t.sizeChan <- remotecommand.TerminalSize{Width: msg.Cols, Height: msg.Rows}
		return 0, nil
	// ping	无内容交互
	case "ping":
		return 0, nil
	default:
		return copy(p, "\u0004"), errors.New("unknown message type")
	}
}

// 写数据的方法，拿到 api-server 的返回内容，向web端输出
func (t *TerminalSession) Write(p []byte) (int, error) {
	msg, err := json.Marshal(TerminalMessage{
		Operation: "stdout",
		Data:      string(p),
	})
	if err != nil {
		log.Logger.Error("Json Marshal err", err)
		return 0, err
	}
	if err = t.wsConn.WriteMessage(websocket.TextMessage, msg); err != nil {
		log.Logger.Error("Terminal write message err", err)
		return 0, err
	}
	return len(p), nil
}

// Done 标记关闭doneChan,关闭后触发退出终端
func (t *TerminalSession) Done() {
	close(t.doneChan)
}

// Close 用于关闭websocket连接
func (t *TerminalSession) Close() error {
	return t.wsConn.Close()
}

// Next 获取web端是否resize,以及是否退出终端
func (t *TerminalSession) Next() *remotecommand.TerminalSize {
	select {
	case size := <-t.sizeChan:
		return &size
	case <-t.doneChan:
		return nil
	}
}

type IdOptions struct {
	Id int64 `uri:"id" binding:"required"`
}

type CloudOptions struct {
	CloudName string `uri:"cloud_name" binding:"required"`
}

type ObjectOptions struct {
	ObjectName string `uri:"object_name" binding:"required"`
}

type NamespaceOptions struct {
	CloudOptions `json:",inline"`

	ObjectOptions `json:",inline"`
}

type WebShellOptions struct {
	CloudName string `form:"cloud_name"`
	Namespace string `form:"namespace"`
	Pod       string `form:"pod"`
	Container string `form:"container"`
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
