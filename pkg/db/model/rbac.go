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
)

func (o Operation) String() string {
	return string(o)
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
