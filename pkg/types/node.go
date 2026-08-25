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

// NodeAuthResult 节点认证对外返回（仅认证类型与端口，不含密钥/密码）
type NodeAuthResult struct {
	Type AuthType `json:"type"`
	Port int      `json:"port"`
}

// NodeResult 主机节点 API 返回结构（与 model.Node 持久化字段对齐）
type NodeResult struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Name    string         `json:"name"`
	UserId  int64          `json:"user_id"`
	Ip      string         `json:"ip"`
	Cluster string         `json:"cluster"`
	Auth    NodeAuthResult `json:"auth"`
}

// CreateNodeRequest POST /pixiu/nodes
type CreateNodeRequest struct {
	Name    string       `json:"name" binding:"required"`
	UserId  int64        `json:"user_id"`
	Ip      string       `json:"ip" binding:"required"`
	Cluster string       `json:"cluster"`
	Auth    PlanNodeAuth `json:"auth" binding:"required"`
}

// SetUserID 实现 UserIDSetter 接口。
func (r *CreateNodeRequest) SetUserID(id int64) { r.UserId = id }

// UpdateNodeRequest PUT /pixiu/nodes/:nodeId
// ResourceVersion 为值类型 int64：客户端必须显式携带当前版本号（乐观锁），0 也会参与 WHERE 比较，
// 未命中则返回 404，用于并发保护。Name/Ip/Auth 为可空指针，仅非空字段参与更新。
type UpdateNodeRequest struct {
	ResourceVersion int64 `json:"resource_version"`

	Name    *string       `json:"name"`
	Ip      *string       `json:"ip"`
	Cluster *string       `json:"cluster"`
	Auth    *PlanNodeAuth `json:"auth"`
}
