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

package types

import (
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

// UserIDSetter 创建请求统一实现：设置资源归属用户 id。
// 创建归属一律为当前登录用户（router 层解析），不允许客户端指定。
type UserIDSetter interface {
	SetUserID(id int64)
}

type (
	// LoginRequest is the request body struct for user login.
	LoginRequest struct {
		Name     string `json:"name" binding:"required"`     // required
		Password string `json:"password" binding:"required"` // required
	}

	CreateUserRequest struct {
		Name        string           `json:"name" binding:"required"`              // required
		Password    string           `json:"password" binding:"required,password"` // required
		Role        model.UserLevel  `json:"role" binding:"omitempty"`             // optional
		Status      model.UserStatus `json:"status" binding:"omitempty"`
		Email       string           `json:"email" binding:"omitempty,email"` // optional
		Phone       string           `json:"phone" binding:"omitempty"`       // optional
		Description string           `json:"description" binding:"omitempty"` // optional
	}

	// UpdateUserRequest
	// !Note: if you want to update description only, email also must be provided with current value
	UpdateUserRequest struct {
		Role            model.UserLevel  `json:"role" binding:"omitempty"`               // required
		Status          model.UserStatus `json:"status" binding:"omitempty,oneof=0 1 2"` // required
		Email           string           `json:"email" binding:"omitempty,email"`        // optional
		Phone           string           `json:"phone" binding:"omitempty"`              // optional
		Description     string           `json:"description" binding:"omitempty"`        // optional
		ResourceVersion *int64           `json:"resource_version" binding:"required"`    // required
	}

	UpdateUserPasswordRequest struct {
		New             string `json:"new" binding:"required,password"`     // required
		Old             string `json:"old"`                                 // 修改自己密码时必填，管理员重置时可不填
		ResourceVersion *int64 `json:"resource_version" binding:"required"` // required
		Reset           bool   `json:"reset"`
	}

	CreateClusterRequest struct {
		Name        string            `json:"name" binding:"omitempty"`       // optional
		AliasName   string            `json:"alias_name" binding:"omitempty"` // optional
		UserId      int64             `json:"user_id"`
		Type        model.ClusterType `json:"cluster_type" binding:"omitempty,oneof=0 1"` // optional
		KubeConfig  string            `json:"kube_config" binding:"omitempty"`            // required for direct; required for tunnel too (auth)
		ConnectMode model.ConnectMode `json:"connect_mode" binding:"omitempty,oneof=0 1"` // 0 direct 1 tunnel
		AgentToken  string            `json:"agent_token" binding:"omitempty"`            // optional, only for tunnel
		Description string            `json:"description" binding:"omitempty"`            // optional
		Protected   bool              `json:"protected" binding:"omitempty"`              // optional

		PermissionId   int64
		OwnerReference int64
	}

	UpdateClusterRequest struct {
		ResourceVersion int64 `json:"resource_version"`

		AliasName   *string `json:"alias_name" binding:"omitempty"`  // optional
		Description *string `json:"description" binding:"omitempty"` // optional
	}

	ProtectClusterRequest struct {
		ResourceVersion *int64 `json:"resource_version" binding:"required"` // required
		Protected       bool   `json:"protected" binding:"omitempty"`       // optional
	}

	// CreateProxyKubeconfigRequest 生成指向 Pixiu 的代理 kubeconfig
	CreateProxyKubeconfigRequest struct {
		ClusterId int64 `uri:"clusterId"` // 由 handler 从 URI 注入

		// ExpiresAt 过期时间（RFC3339，如 2026-08-02T22:00:00+08:00）；为空则按配置默认时长
		ExpiresAt *time.Time `json:"expires_at"`
	}

	CreateDatasourceRequest struct {
		Name        string                  `json:"name" binding:"required"`
		ClusterName string                  `json:"cluster_name"`
		Type        model.DatasourceType    `json:"type"`
		SubType     model.DatasourceSubType `json:"sub_type" binding:"required"`
		Config      *DatasourceConfig       `json:"config"`
		IsDefault   bool                    `json:"is_default"`
		External    bool                    `json:"external"`
		Description string                  `json:"description" binding:"omitempty"`
	}

	UpdateDatasourceRequest struct {
		Id              int64 `json:"id"`
		ResourceVersion int64 `json:"resource_version"`

		CreateDatasourceRequest `form:",inline"`
	}

	CreateProviderRequest struct {
		Name        string `json:"name" binding:"required"`
		BaseURL     string `json:"base_url" binding:"required,url"`
		Protocol    string `json:"protocol" binding:"required"`
		Description string `json:"description" binding:"omitempty"`
		MaxTokens   int    `json:"max_tokens" binding:"omitempty"`
	}

	UpdateProviderRequest struct {
		PixiuMeta `json:",inline"`

		Name        string `json:"name" binding:"required"`
		BaseURL     string `json:"base_url" binding:"required,url"`
		Protocol    string `json:"protocol" binding:"required"`
		Description string `json:"description" binding:"omitempty"`
		MaxTokens   int    `json:"max_tokens" binding:"omitempty"`
	}

	CreateAIAccountRequest struct {
		Name       string `json:"name" binding:"required"`
		APIKey     string `json:"api_key" binding:"required"`
		Model      string `json:"model" binding:"required"`
		ProviderId int64  `json:"provider_id" binding:"required"`
	}

	UpdateAIAccountRequest struct {
		PixiuMeta `json:",inline"`

		Name       string `json:"name" binding:"required"`
		APIKey     string `json:"api_key" binding:"omitempty"`
		Model      string `json:"model" binding:"required"`
		ProviderId int64  `json:"provider_id" binding:"required"`
	}

	CreateTenantRequest struct {
		Name        string  `json:"name" binding:"required"`         // required
		Description *string `json:"description" binding:"omitempty"` // optional
	}

	UpdateTenantRequest struct {
		Name            *string `json:"name" binding:"omitempty"`            // optional
		Description     *string `json:"description" binding:"omitempty"`     // optional
		ResourceVersion *int64  `json:"resource_version" binding:"required"` // required
	}

	CreateRoleRequest struct {
		Name        string  `json:"name" binding:"required"`         // required
		TenantId    *int64  `json:"tenant_id"`                       // optional, nil 或 0 表示系统全局角色
		Description *string `json:"description" binding:"omitempty"` // optional
	}

	UpdateRoleRequest struct {
		Name            *string `json:"name" binding:"omitempty"`            // optional
		Description     *string `json:"description" binding:"omitempty"`     // optional
		ResourceVersion *int64  `json:"resource_version" binding:"required"` // required
	}

	UpdateRoleAPIsRequest struct {
		APIIds []int64 `json:"api_ids"` // 已关联的 API 资源 ID 列表，全量替换
	}

	CreateAPIRequest struct {
		Method      string  `json:"method" binding:"required,oneof=GET POST PUT DELETE PATCH"`
		Path        string  `json:"path" binding:"required"`
		Group       *string `json:"group" binding:"omitempty"`
		Description *string `json:"description" binding:"omitempty"`
	}

	UpdateAPIRequest struct {
		Method          *string `json:"method" binding:"omitempty,oneof=GET POST PUT DELETE PATCH"`
		Path            *string `json:"path" binding:"omitempty"`
		Group           *string `json:"group" binding:"omitempty"`
		Description     *string `json:"description" binding:"omitempty"`
		ResourceVersion *int64  `json:"resource_version" binding:"required"`
	}

	CreatePlanRequest struct {
		Name        string `json:"name" binding:"required"`         // required
		Description string `json:"description" binding:"omitempty"` // optional

		UserId int64 `json:"user_id"` // 关联用户（router 层从会话解析填充，不接受客户端指定）

		// 执行模式：local（默认）/ agent（单向网络）
		ExecMode      model.PlanExecMode `json:"exec_mode" binding:"omitempty,oneof=local agent"`
		DeployAgentId int64              `json:"deploy_agent_id" binding:"omitempty"`

		Config CreatePlanConfigRequest `json:"config"`
		Nodes  []CreatePlanNodeRequest `json:"nodes"`
	}

	UpdatePlanRequest struct {
		Name            string `json:"name" binding:"required"`             // required
		ResourceVersion *int64 `json:"resource_version" binding:"required"` // required
		Description     string `json:"description" binding:"omitempty"`     // optional

		// 执行模式：local（默认）/ agent（单向网络）
		ExecMode      model.PlanExecMode `json:"exec_mode" binding:"omitempty,oneof=local agent"`
		DeployAgentId int64              `json:"deploy_agent_id" binding:"omitempty"`

		Config CreatePlanConfigRequest `json:"config"`
		Nodes  []CreatePlanNodeRequest `json:"nodes"`
	}

	CreatePlanNodeRequest struct {
		Name   string       `json:"name" binding:"omitempty"` // required
		UserId int64        `json:"user_id"`
		PlanId int64        `json:"plan_id"`
		Role   []string     `json:"role"` // k8s 节点的角色，master 和 node, storage
		CRI    model.CRI    `json:"cri"`
		Ip     string       `json:"ip"`
		Auth   PlanNodeAuth `json:"auth"`
	}

	UpdatePlanNodeRequest struct {
		ResourceVersion int64        `json:"resource_version" binding:"required"` // required
		Name            string       `json:"name" binding:"omitempty"`            // required
		PlanId          int64        `json:"plan_id"`
		Role            []string     `json:"role"` // k8s 节点的角色，master 为 1 和 node 为 0
		CRI             model.CRI    `json:"cri"`
		Ip              string       `json:"ip"`
		Auth            PlanNodeAuth `json:"auth"`
	}

	CreatePlanConfigRequest struct {
		PlanId      int64  `json:"plan_id"`
		Region      string `json:"region"`
		OSImage     string `json:"os_image" binding:"required"`     // 操作系统
		Description string `json:"description" binding:"omitempty"` // optional

		Kubernetes KubernetesSpec `json:"kubernetes"`
		Network    NetworkSpec    `json:"network"`
		Runtime    RuntimeSpec    `json:"runtime"`
		Component  ComponentSpec  `json:"component"` // 支持的扩展组件配置
	}

	UpdatePlanConfigRequest struct {
		// TODO:
	}

	CreateDistributionRequest struct {
		Family string `json:"family" binding:"required"`
		Name   string `json:"name" binding:"required"`
		Runner string `json:"runner" binding:"required"`
	}

	UpdateDistributionRequest struct {
		Id              int64   `json:"id"`
		Family          *string `json:"family" binding:"omitempty"`
		Name            *string `json:"name" binding:"omitempty"`
		Runner          *string `json:"runner" binding:"omitempty"`
		ResourceVersion *int64  `json:"resource_version" binding:"required"`
	}

	// PageRequest 分页配置
	PageRequest struct {
		Page  int `form:"page" json:"page"`   // 页数，表示第几页
		Limit int `form:"limit" json:"limit"` // 每页数量
	}
	// QueryOption 搜索配置
	QueryOption struct {
		LabelSelector string `form:"labelSelector" json:"labelSelector"` // 标签搜索
		NameSelector  string `form:"nameSelector" json:"nameSelector"`   // 名称搜索
	}

	AIRespondRequest struct {
		ConversationId int64  `json:"conversation_id"`
		AccountId      int64  `json:"account_id" binding:"required"`
		Input          string `json:"input" binding:"required"`
	}

	// WebSSHRequest 主机 ssh 跳转请求
	WebSSHRequest struct {
		Host       string `form:"host" json:"host" binding:"required"`
		Port       int    `form:"port" json:"port"`
		User       string `form:"user" json:"user"`
		Password   string `form:"password" json:"password"`
		PrivateKey string
	}

	ClusterWebRequest struct {
		ClusterName string `form:"cluster_name" json:"cluster_name"`
		ClusterId   int64  `form:"cluster_id" json:"cluster_id"`
		UserId      int64  `form:"user_id" json:"user_id"`
	}
)

// SetUserID 实现 UserIDSetter 接口。
func (r *CreateClusterRequest) SetUserID(id int64) { r.UserId = id }

// SetUserID 实现 UserIDSetter 接口。
func (r *CreatePlanRequest) SetUserID(id int64) { r.UserId = id }

// SetUserID 实现 UserIDSetter 接口。
func (r *CreatePlanNodeRequest) SetUserID(id int64) { r.UserId = id }

type (
	LoginResponse struct {
		UserId      int64           `json:"user_id"`
		UserName    string          `json:"user_name"`
		Token       string          `json:"token"`
		Role        model.UserLevel `json:"role"`
		*model.User `json:"-"`
	}

	// PageResponse 分页查询返回值
	PageResponse struct {
		PageRequest `json:",inline"` // 分页请求属性

		Total int         `json:"total"` // 分页总数
		Items interface{} `json:"items"` // 指定页的元素列表
	}

	PageResult struct {
		PageRequest `json:",inline"`

		Total   int64       `json:"total"`   // 总记录数
		Items   interface{} `json:"items"`   // 数据列表
		Message string      `json:"message"` // 正常或异常信息
	}
)
