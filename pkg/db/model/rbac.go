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

type Operation string

const (
	OpRead   Operation = "read"
	OpCreate Operation = "create"
	OpUpdate Operation = "update"
	OpDelete Operation = "delete"
	OpAll    Operation = "*"
)

func (o Operation) String() string {
	return string(o)
}

var OperationMap = map[Operation]struct{}{
	OpRead:   {},
	OpCreate: {},
	OpUpdate: {},
	OpDelete: {},
	OpAll:    {},
}

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

// TODO:
type RBACInterface interface{}

// Casbin RBAC model
// ref: https://github.com/casbin/casbin/blob/master/examples/rbac_model.conf
const RBACModel = `
[request_definition]
r = sub, obj, id, op

[policy_definition]
p = sub, obj, id, op

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && keyMatch(r.obj, p.obj) && keyMatch(r.id, p.id) && keyMatch(r.op, p.op)`

// TODO:
type CasbinRBACImpl struct{}

// MakePolicy returns a policy slice.
// e.g. ["foo", "clusters", "*", "read"]
func MakePolicy(username string, obj ObjectType, sid string, op Operation) []string {
	return []string{username, obj.String(), sid, op.String()}
}
