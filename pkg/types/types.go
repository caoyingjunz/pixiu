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
	"fmt"
	"io"
	"sync"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
	appv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

type PixiuObjectMeta struct {
	Cluster   string `uri:"cluster" binding:"required"`
	Namespace string `uri:"namespace" binding:"required"`
	Name      string `uri:"name"`
}

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

type HTTPHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type DatasourceConfig struct {
	Headers []HTTPHeader       `json:"headers"`
	Log     *LogSourceConfig   `json:"log,omitempty"`
	Alert   *AlertSourceConfig `json:"alert,omitempty"`
}

type LogSourceConfig struct {
	URL      string `json:"url,omitempty"`
	UserName string `json:"user_name,omitempty"`
	Password string `json:"password,omitempty"`
}

type AlertSourceConfig struct {
	URL string `json:"url,omitempty"`

	UserName string `json:"user_name,omitempty"`
	Password string `json:"password,omitempty"`
}

type KubeNode struct {
	Ready    []string `json:"ready"`
	NotReady []string `json:"not_ready"`
}

type Cluster struct {
	PixiuMeta `json:",inline"`

	Name      string              `json:"name"`
	AliasName string              `json:"alias_name"`
	Status    model.ClusterStatus `json:"status"` // 0: 运行中 1: 部署中 2: 等待部署 3: 部署失败 4: 集群失联，API不可用

	UserId int64 `json:"user_id"`

	// 0: 标准集群 1: 自建集群
	ClusterType model.ClusterType `json:"cluster_type"`
	PlanId      int64             `json:"plan_id"` // 自建集群关联的 PlanId，如果是自建的集群，planId 不为 0

	PermissionId int64 `json:"permission_id"`

	// kubernetes 集群的版本和状态
	KubernetesVersion string   `json:"kubernetes_version"`
	Nodes             KubeNode `json:"nodes"`

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

type Datasource struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	ClusterName string                  `json:"cluster_name"`
	Name        string                  `json:"name"`
	Type        model.DatasourceType    `json:"type"`
	SubType     model.DatasourceSubType `json:"sub_type"`
	Config      DatasourceConfig        `json:"config"`
	IsDefault   bool                    `json:"is_default"`
	External    bool                    `json:"external"`
	Description string                  `json:"description"`
}

type AIAccount struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	UserId      int64  `json:"user_id"`
	Provider    string `json:"provider"`
	APIKey      string `json:"api_key"`
	BaseURL     string `json:"base_url"`
	Model       string `json:"model"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

type AIRespondResponse struct {
	ConversationId int64       `json:"conversation_id"`
	ResponseId     string      `json:"response_id"`
	Text           string      `json:"text"`
	Model          string      `json:"model"`
	Raw            interface{} `json:"raw,omitempty"`
}

type AIStreamEvent struct {
	Type           string      `json:"type"`
	Stage          string      `json:"stage,omitempty"`
	Message        string      `json:"message,omitempty"`
	Delta          string      `json:"delta,omitempty"`
	Text           string      `json:"text,omitempty"`
	Model          string      `json:"model,omitempty"`
	ToolName       string      `json:"tool_name,omitempty"`
	ToolCallId     string      `json:"tool_call_id,omitempty"`
	ToolArgs       string      `json:"tool_args,omitempty"`
	ToolOutput     string      `json:"tool_output,omitempty"`
	ConversationId int64       `json:"conversation_id,omitempty"`
	ResponseId     string      `json:"response_id,omitempty"`
	Raw            interface{} `json:"raw,omitempty"`
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
	Role        model.UserLevel  `json:"role"`                                 // 用户角色，目前只实现管理员，0: 普通用户 1: 管理员 2: 超级管理员
	Email       string           `json:"email"`                                // 用户注册邮件
	Phone       string           `json:"phone"`                                // 用户手机号
	Description string           `json:"description"`                          // 用户描述信息

	TimeMeta `json:",inline"`
}

type Tenant struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Name        string `json:"name"`        // 用户名称
	Description string `json:"description"` // 用户描述信息
}

type Role struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	TenantId    int64  `json:"tenant_id"` // 0 表示系统全局角色
	Name        string `json:"name"`
	Description string `json:"description"`
}

type APIResource struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Method      string `json:"method"`
	Path        string `json:"path"`
	Group       string `json:"group"`
	Description string `json:"description"`
}

type RoleAPIsResponse struct {
	Associated   []APIResource `json:"associated"`
	Unassociated []APIResource `json:"unassociated"`
}

type Plan struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Name              string           `json:"name"` // 用户名称
	Step              model.TaskStatus `json:"step"`
	Description       string           `json:"description"`        // 用户描述信息
	KubernetesVersion string           `json:"kubernetes_version"` // k8s 版本
	NodeCount         int              `json:"node_count"`         // 节点总数

	Config PlanConfig `json:"config"`
	Nodes  []PlanNode `json:"nodes"`
}

type PlanNode struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Name   string       `json:"name"` // required
	UserId int64        `json:"user_id,omitempty"`
	PlanId int64        `json:"plan_id,omitempty"`
	Role   []string     `json:"role"` // k8s 节点的角色，master 和 node
	CRI    model.CRI    `json:"cri"`
	Ip     string       `json:"ip"`
	Auth   PlanNodeAuth `json:"auth,omitempty"`
}

type Audit struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Ip                string                     `json:"ip"`
	Action            string                     `json:"action"`             // 操作动作
	Status            model.AuditOperationStatus `json:"status"`             // 操作状态
	Operator          string                     `json:"operator"`           // 操作人
	Path              string                     `json:"path"`               // 操作路径
	ObjectType        model.ObjectType           `json:"resource_type"`      // 资源类型
	Duration          int64                      `json:"duration"`           // 请求耗时 ms
	ResponseCode      int                        `json:"response_code"`      // HTTP 响应码
	Cluster           string                     `json:"cluster"`            // K8s 集群名
	ResourceName      string                     `json:"resource_name"`      // 资源名称
	ResourceNamespace string                     `json:"resource_namespace"` // 资源命名空间
}

type AuthType string

const (
	NoneAuth     AuthType = "none"     // 已开启密码
	KeyAuth      AuthType = "key"      // 密钥
	PasswordAuth AuthType = "password" // 密码
)

const (
	defaultExpiration        = 365 * 24 * time.Hour
	defaultExpirationSeconds = int64(defaultExpiration / time.Second)
	defaultNamespace         = "pixiu-system"
)

type PlanNodeAuth struct {
	Type     AuthType      `json:"type"` // 节点认证模式，支持 key 和 password
	Key      *KeySpec      `json:"key,omitempty"`
	Password *PasswordSpec `json:"password,omitempty"`
}

type PlanTask struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Name    string           `json:"name"`
	PlanId  int64            `json:"plan_id" binding:"required"`
	Action  string           `json:"action"`
	Status  model.TaskStatus `json:"status"`
	Message string           `json:"message"`
}

type KeySpec struct {
	Data string `json:"data,omitempty"`
	File string `json:"-"`
}

type PasswordSpec struct {
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
}

type PlanConfig struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	PlanId  int64  `json:"plan_id,omitempty"` // required
	Region  string `json:"region"`
	OSImage string `json:"os_image"` // 操作系统

	Kubernetes KubernetesSpec `json:"kubernetes"`
	Network    NetworkSpec    `json:"network"`
	Runtime    RuntimeSpec    `json:"runtime"`
	Component  ComponentSpec  `json:"component"` // 支持的扩展组件配置

}

// Distribution 部署支持的操作系统发行版
type Distribution struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Family string `json:"family"`
	Name   string `json:"name"`
	Runner string `json:"runner"`
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
	Cluster     string   `form:"cluster"`
	Namespace   string   `form:"namespace"`
	Pod         string   `form:"pod"`
	Container   string   `form:"container"`
	Command     string   `form:"command"`
	CommandArgs []string `form:"-"`
}

// TerminalMessage 定义了终端和容器 shell 交互内容的格式 Operation 是操作类型
// Data 是具体数据内容 Rows和Cols 可以理解为终端的行数和列数，也就是宽、高
type TerminalMessage struct {
	Operation string `json:"operation"`
	Data      string `json:"data"`
	Rows      uint16 `json:"rows"`
	Cols      uint16 `json:"cols"`
}

// TerminalSession 定义 TerminalSession 结构体，实现 PtyHandler 接口
// wsConn 是 websocket 连接
// sizeChan 用来定义终端输入和输出的宽和高
// doneChan 用于标记退出终端
type TerminalSession struct {
	wsConn   *websocket.Conn
	sizeChan chan remotecommand.TerminalSize
	doneChan chan struct{}
}

type Turn struct {
	StdinPipe io.WriteCloser
	Session   *ssh.Session
	WsConn    *websocket.Conn
}

// ListOptions is the query options to a standard REST list call.
type ListOptions struct {
	UserId int64 `form:"user_id" json:"user_id"` // 用户 id

	CustomMeta  `json:",inline"`
	PageRequest `json:",inline"` // 分页请求属性
	QueryOption `json:",inline"` // 搜索内容
}

type CustomMeta struct {
	Status *int
	Step   string `form:"step" json:"step"` // plan 查询的时候需要 状态过滤，不传则不过滤

	ClusterName    string                `form:"cluster_name" json:"cluster_name"`
	DatasourceType *model.DatasourceType `form:"datasource_type" json:"datasource_type"`
}

func (o *ListOptions) SetDefaultPageOption() {
	// 初始化分页属性
	if o.Page <= 0 {
		o.Page = 1
	}
	if o.Limit <= 0 {
		o.Limit = 10
	}
	if o.Limit > 100 {
		o.Limit = 100
	}
}

type EventOptions struct {
	Uid        string `form:"uid"`
	Namespace  string `form:"namespace"`
	Name       string `form:"name"`
	Kind       string `form:"kind"`
	Namespaced bool   `form:"namespaced"`
	Limit      int64  `form:"limit"`
}

type PodLogOptions struct {
	Container string `form:"container"`
	TailLines int64  `form:"tailLines"`
}

type KubernetesSpec struct {
	EnablePublicIp    bool   `json:"enable_public_ip"`
	ApiServer         string `json:"api_server"`
	ApiPort           string `json:"api_port"`
	KubernetesVersion string `json:"kubernetes_version"`
	EnableHA          bool   `json:"enable_ha"`
	Register          bool   `json:"register"`
	ImageRepository   string `json:"image_repository,omitempty"` // kubernetes 镜像仓库地址
	SetHostname       bool   `json:"set_hostname"`               // 自动修改k8s节点名称, Rocky 系统不生效
	Protect           bool   `json:"protect"`                    // 开启集群保护，防止误删除
	ChangeSelinux     bool   `json:"change_selinux"`             // 关闭 Selinux
}

type NetworkSpec struct {
	NetworkInterface string `json:"network_interface"` // 网口，默认 eth0
	Cni              string `json:"cni"`
	PodNetwork       string `json:"pod_network"`
	ServiceNetwork   string `json:"service_network"`
	KubeProxy        string `json:"kube_proxy"`
}

type RuntimeSpec struct {
	Runtime string `json:"runtime"`
	DataDir string `json:"data_dir"` // 自定义容器运行时数据存放目录
}

type ComponentSpec struct {
	Helm         *Helm         `json:"helm,omitempty"` // 忽略，则使用默认值
	Prometheus   *Prometheus   `json:"prometheus,omitempty"`
	Grafana      *Grafana      `json:"grafana,omitempty"`
	Haproxy      *Haproxy      `json:"haproxy,omitempty"`
	MetricServer *MetricServer `json:"metric_server,omitempty"`
	IngressNginx *IngressNginx `json:"ingress_nginx,omitempty"`
	NFS          *NFS          `json:"nfs,omitempty"`
}

type Helm struct {
	Enable      bool   `json:"enable"`
	HelmRelease string `json:"helm_release"`
}

type NFS struct {
	Enable bool `json:"enable"`

	StorageClassName string `json:"storage_class_name"` // 指定 nfs 存储名称
	StorageDataDir   string `json:"storage_data_dir"`   // 指定 nfs server 存储地址
}

type MetricServer struct {
	Enable bool `json:"enable"`
}

type IngressNginx struct {
	Enable bool `json:"enable"`
}

type Prometheus struct {
	EnablePrometheus string `json:"enable_prometheus"`
	Enable           bool   `json:"enable"`
}

type Grafana struct {
	Enable               bool   `json:"enable"`
	GrafanaAdminUser     string `json:"grafana_admin_user"`
	GrafanaAdminPassword string `json:"grafana_admin_password"`
}

// Haproxy Options
// This configuration is usually enabled when self-created VMs require high availability.
type Haproxy struct {
	Enable                    bool   `json:"enable"`                       // Enable haproxy and keepalived,
	KeepalivedVirtualRouterId string `json:"keepalived_virtual_router_id"` // Arbitrary unique number from 0..255
}

// AuditListOptions 审计列表查询选项，支持过滤
type AuditListOptions struct {
	ListOptions `json:",inline"`
	Operator    string `form:"operator"`    // 模糊匹配操作人
	Action      string `form:"action"`      // 精确匹配 HTTP 方法（POST/PUT/DELETE/PATCH）
	ObjectType  string `form:"object_type"` // 资源类型
	Cluster     string `form:"cluster"`     // 集群名称
	Status      *uint8 `form:"status"`      // 操作状态（0:失败 1:成功 2:未知）
	StartTime   string `form:"start_time"`  // 时间范围起（RFC3339，留空忽略）
	EndTime     string `form:"end_time"`    // 时间范围止（RFC3339，留空忽略）
}

// CreatePermissionRequest 创建 scoped kubeconfig 的请求参数
type CreatePermissionRequest struct {
	ClusterId         int64  `json:"cluster_id" binding:"required"` // 授权k8s的集群ID，主集群
	UserId            int64  `json:"user_id"`
	Name              string `json:"name" binding:"required"`
	ExpirationSeconds int64  `json:"expiration_seconds"` // 默认 1 年
	Description       string `json:"description"`

	PType int                 `json:"p_type"` // 0 只读，1 自定义，2 管理员
	Rules []rbacv1.PolicyRule `json:"rules"`  // p_type=1 时使用

	SAName          string `json:"sa_name"`
	SANamespace     string `json:"sa_namespace"`
	ClusterRoleName string `json:"cluster_role_name"`
	RoleBindingName string `json:"role_binding_name"`

	TargetNamespaces []string `json:"target_namespaces"`
}

// UpdatePermissionRequest 更新权限
type UpdatePermissionRequest struct {
	PixiuMeta `json:",inline"`

	Name              string              `json:"name"`
	ExpirationSeconds int64               `json:"expiration_seconds"` // 默认 1 年
	Description       string              `json:"description"`
	PType             int                 `json:"p_type"` // 0 只读，1 自定义，2 管理员
	Rules             []rbacv1.PolicyRule `json:"rules"`  // p_type=1 时使用
	TargetNamespaces  []string            `json:"target_namespaces"`
	Force             bool                `json:"force"` // 强制下发
}

func (o *CreatePermissionRequest) SetDefaultOptions() {
	if o.ExpirationSeconds <= 0 {
		o.ExpirationSeconds = defaultExpirationSeconds
	}

	if len(o.SANamespace) == 0 {
		o.SANamespace = defaultNamespace
	}

	if len(o.SAName) == 0 {
		o.SAName = fmt.Sprintf("pixiu-sa-%d", o.UserId)
	}

	if len(o.ClusterRoleName) == 0 {
		o.ClusterRoleName = fmt.Sprintf("pixiu-cr-%d", o.UserId)
	}
	if o.PType == 0 {
		o.ClusterRoleName = "pixiu-view"
	}
	if o.PType == 2 {
		o.ClusterRoleName = "cluster-admin"
	}

	if len(o.RoleBindingName) == 0 {
		o.RoleBindingName = fmt.Sprintf("pixiu-rb-%d", o.UserId)
	}
}

// Permission 集群 scoped kubeconfig 授权
type Permission struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	UserId            int64               `json:"user_id"`
	UserName          string              `json:"user_name"`
	ClusterId         int64               `json:"cluster_id"`
	ClusterName       string              `json:"cluster_name"`
	ClusterAliasName  string              `json:"cluster_alias_name"`
	Name              string              `json:"name"`
	ExpirationSeconds int64               `json:"expiration_seconds"`
	PType             int                 `json:"p_type"`
	Rules             []rbacv1.PolicyRule `json:"rules,omitempty"`
	SAName            string              `json:"sa_name"`
	SANamespace       string              `json:"sa_namespace"`
	TargetNamespaces  []string            `json:"target_namespaces"`
	KubeConfig        string              `json:"kube_config,omitempty"`
	Content           string              `json:"content,omitempty"` // 与 kube_config 相同，便于前端展示
	Description       string              `json:"description,omitempty"`
}

// KubeConfigResponse 返回给前端的 kubeconfig 内容
type KubeConfigResponse struct {
	ClusterName string `json:"cluster_name"`
	Content     string `json:"content"`
}
