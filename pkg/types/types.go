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

// Role kubernetes 角色定义
type Role string

var (
	MasterRole Role = "master"
	NodeRole   Role = "node"
)

// ResourceType 貔貅的资源类型
type ResourceType string

var (
	CloudResource ResourceType = "cloud"
)

// EventType 审计事件类型
type EventType string

var (
	CreateEvent EventType = "新建"
	UpdateEvent EventType = "更新"
	DeleteEvent EventType = "删除"
	GetEvent    EventType = "查询"
)

type Event struct {
	User     string       `json:"user"`      // 用户名称
	ClientIP string       `json:"client_ip"` // 登陆 ip 地址
	Operator EventType    `json:"operator"`  // 操作类型，新增，更新，删除
	Object   ResourceType `json:"object"`    // 资源类型，比如 cloud，user，kubernetes
	Message  string       `json:"message"`
}
