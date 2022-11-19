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

// Role k8s 角色定义
type Role string

var (
	MasterRole Role = "master"
	NodeRole   Role = "node"
)

// EventType 审计事件类型
type EventType string

var (
	CreateEvent EventType = "create"
	UpdateEvent EventType = "update"
	DeleteEvent EventType = "delete"
	GetEvent    EventType = "get"
)

type Event struct {
	User      string    `json:"user"`
	EventType EventType `json:"event_type"`
	ClientIP  string    `json:"client_ip"`
	Operator  string    `json:"operator"`
	Object    string    `json:"object"`
	Message   string    `json:"message"`
}
