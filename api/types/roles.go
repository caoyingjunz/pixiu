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

// TODO: 临时参数定义，后续优化

type Menus struct {
	MenuIDS []int64 `json:"menu_ids"`
}

type Roles struct {
	RoleIds []int64 `json:"role_ids"`
}

type RoleReq struct {
	Memo     string `json:"memo" `      // 备注
	Name     string `json:"name"`       // 名称
	Sequence int    `json:"sequence" `  // 排序值
	ParentID int64  `json:"parent_id" ` // 父级ID
	Status   int8   `json:"status" `    // 0 表示禁用，1 表示启用
}

type UpdateRoleReq struct {
	Memo            string `json:"memo" `      // 备注
	Name            string `json:"name"`       // 名称
	Sequence        int    `json:"sequence" `  // 排序值
	ParentID        int64  `json:"parent_id" ` // 父级ID
	Status          int8   `json:"status" `    // 0 表示禁用，1 表示启用
	ResourceVersion int64  `json:"resource_version"`
}
