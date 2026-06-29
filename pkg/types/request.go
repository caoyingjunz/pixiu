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

import "github.com/caoyingjunz/pixiu/pkg/db/model"

const AllNamespace = "all_namespaces"

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
		KubeConfig  string            `json:"kube_config" binding:"required"`             // required
		Description string            `json:"description" binding:"omitempty"`            // optional
		Protected   bool              `json:"protected" binding:"omitempty"`              // optional

		PermissionId   int64
		OwnerReference int64
	}

	UpdateClusterRequest struct {
		AliasName   *string `json:"alias_name" binding:"omitempty"`  // optional
		Description *string `json:"description" binding:"omitempty"` // optional
		// TODO: put resource version in a common struct for updating request only
		ResourceVersion *int64 `json:"resource_version" binding:"required"` // required
	}

	ProtectClusterRequest struct {
		ResourceVersion *int64 `json:"resource_version" binding:"required"` // required
		Protected       bool   `json:"protected" binding:"omitempty"`       // optional
	}

	CreateDatasourceRequest struct {
		Name        string                  `json:"name" binding:"required"`
		ClusterName string                  `json:"cluster_name"`
		Type        model.DatasourceType    `json:"type"`
		SubType     model.DatasourceSubType `json:"sub_type" binding:"required"`
		Config      *DatasourceConfig       `json:"config"`
		IsDefault   bool                    `json:"is_default"`
		Description string                  `json:"description" binding:"omitempty"`
	}

	UpdateDatasourceRequest struct {
		Id              int64 `json:"id"`
		ResourceVersion int64 `json:"resource_version"`

		CreateDatasourceRequest `form:",inline"`
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

	ListTenantRequest struct {
		PageRequest  `form:",inline"`
		NameSelector string `form:"nameSelector" json:"nameSelector"` // 名称模糊搜索
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

	ListRoleRequest struct {
		PageRequest  `form:",inline"`
		NameSelector string `form:"nameSelector" json:"nameSelector"` // 名称模糊搜索
		TenantId     *int64 `form:"tenant_id" json:"tenant_id"`       // 租户 ID 过滤
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

	ListAPIRequest struct {
		PageRequest  `form:",inline"`
		Method       string `form:"method" json:"method"`
		PathSelector string `form:"pathSelector" json:"pathSelector"`
		Group        string `form:"group" json:"group"`
	}

	CreatePlanRequest struct {
		Name        string `json:"name" binding:"required"`         // required
		Description string `json:"description" binding:"omitempty"` // optional

		UserId int64 `json:"user_id" binding:"required"` // 关联用户

		Config CreatePlanConfigRequest `json:"config"`
		Nodes  []CreatePlanNodeRequest `json:"nodes"`
	}

	UpdatePlanRequest struct {
		Name            string `json:"name" binding:"required"`             // required
		ResourceVersion *int64 `json:"resource_version" binding:"required"` // required
		Description     string `json:"description" binding:"omitempty"`     // optional

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

	ListDistributionRequest struct {
		PageRequest  `form:",inline"`
		Family       string `form:"family" json:"family"`
		NameSelector string `form:"nameSelector" json:"nameSelector"`
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

	// ListUserRequest 用户列表查询参数
	ListUserRequest struct {
		PageRequest `form:",inline"`
		UserName    string `form:"userName" json:"userName"`
		UserPhone   string `form:"userPhone" json:"userPhone"`
		UserEmail   string `form:"userEmail" json:"userEmail"`
		Status      *int   `form:"status" json:"status"`
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
