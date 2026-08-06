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

// Package scope 提供平台数据权限（资源实例）纯策略判断。
package scope

// CanAccess 数据权限决策：超管 → 资源 owner → 角色 scope 命中。
// hasScope 由调用方查库后传入，本函数不做 I/O。
func CanAccess(isRoot, isOwner, hasScope bool) bool {
	return isRoot || isOwner || hasScope
}

// NeedFilter 非超管列表查询需叠加角色 scope 授权的资源 ID。
func NeedFilter(isRoot bool) bool {
	return !isRoot
}
