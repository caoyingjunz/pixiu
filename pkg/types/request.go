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
		Role        model.UserRole   `json:"role" binding:"omitempty,oneof=0 1 2"` // optional
		Status      model.UserStatus `json:"status" binding:"omitempty"`
		Email       string           `json:"email" binding:"omitempty,email"` // optional
		Description string           `json:"description" binding:"omitempty"` // optional
	}

	// UpdateUserRequest
	// !Note: if you want to update description only, email also must be provided with current value
	UpdateUserRequest struct {
		Role            model.UserRole   `json:"role" binding:"omitempty,oneof=0 1 2"`   // required
		Status          model.UserStatus `json:"status" binding:"omitempty,oneof=0 1 2"` // required
		Email           string           `json:"email" binding:"omitempty,email"`        // optional
		Description     string           `json:"description" binding:"omitempty"`        // optional
		ResourceVersion *int64           `json:"resource_version" binding:"required"`    // required
	}

	UpdateUserPasswordRequest struct {
		New             string `json:"new" binding:"required,password"`     // required
		Old             string `json:"old" binding:"required"`              // required
		ResourceVersion *int64 `json:"resource_version" binding:"required"` // required
		Reset           bool   `json:"reset"`
	}

	CreateClusterRequest struct {
		Name        string            `json:"name" binding:"omitempty"`                   // optional
		AliasName   string            `json:"alias_name" binding:"omitempty"`             // optional
		Type        model.ClusterType `json:"cluster_type" binding:"omitempty,oneof=0 1"` // optional
		KubeConfig  string            `json:"kube_config" binding:"required"`             // required
		Description string            `json:"description" binding:"omitempty"`            // optional
		Protected   bool              `json:"protected" binding:"omitempty"`              // optional
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

	CreateTenantRequest struct {
		Name        string  `json:"name" binding:"required"`         // required
		Description *string `json:"description" binding:"omitempty"` // optional
	}

	UpdateTenantRequest struct {
		Name            *string `json:"name" binding:"omitempty"`            // optional
		Description     *string `json:"description" binding:"omitempty"`     // optional
		ResourceVersion *int64  `json:"resource_version" binding:"required"` // required
	}

	CreatePlanRequest struct {
		Name        string `json:"name" binding:"required"`         // required
		Description string `json:"description" binding:"omitempty"` // optional

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
		PlanId int64        `json:"plan_id"`
		Role   []string     `json:"role"` // k8s 节点的角色，master 和 node
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

	RBACPolicyRequest struct {
		// user ID or group name is required
		UserId     *int64           `json:"user_id" binding:"required_without=GroupName,excluded_with=GroupName"`
		GroupName  *string          `json:"group_name" binding:"required_without=UserId,excluded_with=UserId"`
		ObjectType model.ObjectType `json:"object_type" binding:"required,rbac_object"`
		SID        string           `json:"sid" binding:"omitempty,rbac_sid"`
		Operation  model.Operation  `json:"operation" binding:"required,rbac_operation"`
	}

	ListRBACPolicyRequest struct {
		UserId     int64             `form:"user_id" binding:"required"`
		ObjectType *model.ObjectType `form:"object_type" binding:"omitempty,required_with=UserId,rbac_object"`
		SID        *string           `form:"sid" binding:"omitempty,required_with=ObjectType,rbac_sid"`
		Operation  *model.Operation  `form:"operation" binding:"omitempty,required_with=SID,rbac_operation"`
	}

	GroupBindingRequest struct {
		UserId    int64  `json:"user_id" binding:"required"`
		GroupName string `json:"group_name" binding:"required"`
	}

	ListGroupBindingRequest struct {
		UserId    *int64  `form:"user_id" binding:"omitempty"`
		GroupName *string `form:"group_name" binding:"omitempty"`
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

	// WebSSHRequest 主机 ssh 跳转请求
	WebSSHRequest struct {
		Host     string `form:"host" json:"host" binding:"required"`
		Port     int    `form:"port" json:"port"`
		User     string `form:"user" json:"user" binding:"required"`
		Password string `form:"password" json:"password"`
	}
)

type (
	LoginResponse struct {
		UserId      int64          `json:"user_id"`
		UserName    string         `json:"user_name"`
		Token       string         `json:"token"`
		Role        model.UserRole `json:"role"`
		*model.User `json:"-"`
	}

	// PageResponse 分页查询返回值
	PageResponse struct {
		PageRequest `json:",inline"` // 分页请求属性

		Total int         `json:"total"` // 分页总数
		Items interface{} `json:"items"` // 指定页的元素列表
	}
)
