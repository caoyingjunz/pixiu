/*
Copyright 2026 The Pixiu Authors.

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
	register(&ResourceLabel{})
}

// ResourceLabel 平台级标签关联表：一个资源一行，标签整体存为 JSON。
// (resource_type, resource_id) 唯一，新增资源类型只需写本表，无需改动原表。
type ResourceLabel struct {
	pixiu.Model

	ResourceType string `gorm:"column:resource_type;type:varchar(32);not null;uniqueIndex:uk_resource_label,priority:1" json:"resource_type"`
	ResourceId   int64  `gorm:"column:resource_id;not null;uniqueIndex:uk_resource_label,priority:2" json:"resource_id"`
	Labels       Labels `gorm:"column:labels;type:json" json:"labels"`
}

func (*ResourceLabel) TableName() string { return "resource_labels" }
