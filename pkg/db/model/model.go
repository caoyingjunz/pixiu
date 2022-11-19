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
	AliasName   string `json:"alias_name"`                        // 集群别名，支持中文
	Status      int    `json:"status"`                            // 集群状态
	CloudType   int    `json:"cloud_type"`                        // 集群类型，支持自建和标准
	KubeVersion string `json:"kube_version"`                      // k8s 的版本
	NodeNumber  int    `json:"node_number"`
	Resources   string `json:"resources"`
	Description string `gorm:"type:text" json:"description"`
	Extension   string `gorm:"type:text" json:"extension"`
}

func (*Cloud) TableName() string {
	return "clouds"
}

// Cluster k8s 集群的部署信息
type Cluster struct {
	gopixiu.Model

	CloudId     int64  `gorm:"index:idx_cloud,unique" json:"cloud_id"`
	ApiServer   string `json:"api_server"` // kubernetes 的 apiServer 的 ip 地址
	Version     string `json:"version"`    // k8s 的版本
	Runtime     string `json:"runtime"`    // 容器运行时，目前支持 docker 和 containerd
	Cni         string `json:"cni"`        // 网络 cni，支持 flannel 和 calico
	ServiceCidr string `json:"service_cidr"`
	PodCidr     string `json:"pod_cidr"`
	ProxyMode   string `json:"proxy_mode"` // kubeProxy 的模式，只能是 iptables 和 ipvs
}

func (*Cluster) TableName() string {
	return "clusters"
}

type Node struct {
	gopixiu.Model

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

type User struct {
	gopixiu.Model

	Name        string `gorm:"index:idx_name,unique" json:"name"`
	Password    string `gorm:"type:varchar(256)" json:"-"`
	Status      int8   `gorm:"type:tinyint" json:"status"`
	Role        string `gorm:"type:varchar(128)" json:"role"`
	Email       string `gorm:"type:varchar(128)" json:"email"`
	Description string `gorm:"type:text" json:"description"`
	Extension   string `gorm:"type:text" json:"extension,omitempty"`
}

func (user *User) TableName() string {
	return "users"
}

type KubeConfig struct {
	gopixiu.Model

	ServiceAccount      string `gorm:"unique" json:"service_account"`
	CloudId             int64  `json:"cloud_id"`
	CloudName           string `gorm:"index:idx_cloud_name" json:"cloud_name"`
	ClusterRole         string `json:"cluster_role"`
	Config              string `gorm:"type:text" json:"config"`
	ExpirationTimestamp string `json:"expiration_timestamp"`
}

func (*KubeConfig) TableName() string {
	return "kube_configs"
}

type Event struct {
	gopixiu.Model

	User     string `json:"user"`
	ClientIP string `json:"client_ip"`
	Operator string `json:"operator"`
	Object   string `json:"object"`
	Message  string `json:"message"`
}

func (event *Event) TableName() string {
	return "events"
}
