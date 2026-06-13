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

// NodeResult 主机节点 API 返回结构（与 model.Node 持久化字段对齐）
type NodeResult struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Name   string `json:"name"`
	UserId int64  `json:"user_id"`
	Ip     string `json:"ip"`
	Auth   string `json:"auth"`
}

// CreateNodeRequest POST /pixiu/nodes
type CreateNodeRequest struct {
	Name   string       `json:"name" binding:"required"`
	UserId int64        `json:"user_id"`
	Ip     string       `json:"ip" binding:"required"`
	Auth   PlanNodeAuth `json:"auth" binding:"required"`
}

// UpdateNodeRequest PUT /pixiu/nodes/:nodeId
// ResourceVersion 使用指针：binding:"required" 在 int64 上会把合法值 0 判为缺失，乐观锁版本 0 必须允许提交。
type UpdateNodeRequest struct {
	ResourceVersion int64 `json:"resource_version"`

	Name *string       `json:"name"`
	Ip   *string       `json:"ip"`
	Auth *PlanNodeAuth `json:"auth"`
}
