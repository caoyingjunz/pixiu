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

package model

import "github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"

func init() {
	register(&Node{})
}

type Node struct {
	pixiu.Model

	CloudId  int64  `gorm:"index:idx_cloud" json:"cloud_id"`
	Role     string `json:"role"` // k8s 节点的角色，master 为 0  和 node 为 1
	HostName string `json:"host_name"`
	Address  string `json:"address"`
	User     string `json:"user"`
	Password string `json:"password"`
}

func (*Node) TableName() string {
	return "nodes"
}
