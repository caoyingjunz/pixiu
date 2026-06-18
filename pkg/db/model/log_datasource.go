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
	register(&ClusterLogDatasource{})
}

type LogDatasourceType string

const (
	LogDatasourceTypeLoki LogDatasourceType = "loki"
)

type ClusterLogDatasource struct {
	pixiu.Model

	ClusterName string            `gorm:"column:cluster_name;type:varchar(128);index;not null" json:"cluster_name"`
	Name        string            `gorm:"column:name;type:varchar(128);not null" json:"name"`
	Type        LogDatasourceType `gorm:"column:type;type:varchar(32);not null" json:"type"`
	URL         string            `gorm:"column:url;type:varchar(1024);not null" json:"url"`
	Username    string            `gorm:"column:username;type:varchar(255)" json:"username"`
	Password    string            `gorm:"column:password;type:text" json:"password"`
	Headers     string            `gorm:"column:headers;type:text" json:"headers"`
	IsDefault   bool              `gorm:"column:is_default;default:false;not null" json:"is_default"`
	Description string            `gorm:"column:description;type:text" json:"description"`
}

func (*ClusterLogDatasource) TableName() string {
	return "cluster_log_datasources"
}
