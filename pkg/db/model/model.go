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

type Cloud struct {
	gopixiu.Model

	Name        string `gorm:"index:idx_name,unique" json:"name"` // 集群名，唯一
	Status      int    `json:"status"`
	CloudType   string `json:"cloud_type"`              // 集群类型，支持自建和标准
	KubeVersion string `json:"kube_version"`            // k8s 的版本
	KubeConfig  string `gorm:"type:text" json:"config"` // 集群 config
	NodeNumber  int    `json:"node_number"`
	Resources   string `json:"resources"`
	Description string `gorm:"type:text" json:"description"`
	Extension   string `gorm:"type:text" json:"extension"`
}

func (*Cloud) TableName() string {
	return "clouds"
}

type User struct {
	gopixiu.Model

	Name        string `gorm:"index:idx_name,unique" json:"name"`
	Password    string `gorm:"type:varchar(256)" json:"password"`
	Status      int8   `gorm:"type:tinyint" json:"status"`
	Role        string `gorm:"type:varchar(128)" json:"role"`
	Email       string `gorm:"type:varchar(128)" json:"email"`
	Description string `gorm:"type:text" json:"description"`
	Extension   string `gorm:"type:text" json:"extension,omitempty"`
}

func (user *User) TableName() string {
	return "users"
}
