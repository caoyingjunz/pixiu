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

import "time"

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

type Cluster struct {
	PixiuMeta `json:",inline"`

	Name      string `json:"name"`
	AliasName string `json:"alias_name"`
	// k8s kubeConfig base64 字段
	KubeConfig string `json:"kube_config,omitempty"`
	// 集群用途描述，可以为空
	Description string `json:"description"`

	TimeMeta `json:",inline"`
}

type User struct {
	PixiuMeta `json:",inline"`

	Name        string `json:"name"`               // 用户名称
	Password    string `json:"password,omitempty"` // 用户密码
	Status      int8   `json:"status"`             // 用户状态标识
	Role        string `json:"role"`               // 用户角色，目前只实现管理员，0: 普通用户 1: 管理员 2: 超级管理员
	Email       string `json:"email"`              // 用户注册邮件
	Description string `json:"description"`        // 用户描述信息

	TimeMeta `json:",inline"`
}
