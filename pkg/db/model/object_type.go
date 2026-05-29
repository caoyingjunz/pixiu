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

// ObjectType 审计日志中的资源类型标识（与历史 audits.resource_type 字段兼容）。
type ObjectType string

const (
	ObjectUser    ObjectType = "users"
	ObjectCluster ObjectType = "clusters"
	ObjectTenant  ObjectType = "tenants"
	ObjectPlan    ObjectType = "plans"
	ObjectAuth    ObjectType = "auth"
	ObjectAll     ObjectType = "*"
)

func (o ObjectType) String() string {
	return string(o)
}

var ObjectTypeMap = map[ObjectType]struct{}{
	ObjectUser:    {},
	ObjectCluster: {},
	ObjectTenant:  {},
	ObjectPlan:    {},
	ObjectAuth:    {},
	ObjectAll:     {},
}
