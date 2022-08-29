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

package model

import (
	"github.com/caoyingjunz/gopixiu/pkg/db/gopixiu"
)

type Demo struct {
	gopixiu.Model

	Name string `gorm:"index:idx_name,unique" json:"name"` // 用户名，唯一
}

func (demo *Demo) TableName() string {
	return "demos"
}

type K8sCluster struct {
	gopixiu.Model

	Name   string `gorm:"index:idx_name,unique" json:"name"` // 集群名，唯一
	Config string `json:"config"`                            // 集群config
}

func (*K8sCluster) TableName() string {
	return "k8s_clusters"
}
