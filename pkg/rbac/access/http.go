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

// Package access 提供平台 HTTP 访问策略（与 Gin/DB 解耦的纯判断）。
package access

import (
	"net/http"

	rbacapi "github.com/caoyingjunz/pixiu/pkg/rbac/api"
)

// Decision HTTP 访问决策。
type Decision int

const (
	// Allow 直接放行（超管、白名单）。
	Allow Decision = iota
	// Proxy 走代理鉴权链路（ValidProxy / kube Permission）。
	Proxy
	// Check 需对照角色 buttons 集合做 METHOD:path 匹配。
	Check
)

const (
	pathUserPermissions = "/pixiu/users/permissions"
	pathProxy           = "/pixiu/proxy/:clusterName/*act"
	pathExternal        = "/pixiu/external/*act"
)

// Classify 根据角色与请求路由模板分类决策（不含 DB 查询）。
// roleId==0 表示超管。
func Classify(roleId int64, method, fullPath string) Decision {
	if roleId == 0 {
		return Allow
	}
	// 任意已登录用户可拉自身权限，避免菜单初始化死锁
	if method == http.MethodGet && fullPath == pathUserPermissions {
		return Allow
	}
	if fullPath == pathProxy || fullPath == pathExternal {
		return Proxy
	}
	return Check
}

// AllowedBySet 在 Check 决策下，用 buttons 集合判断是否放行。
func AllowedBySet(set map[string]bool, method, fullPath string) bool {
	return rbacapi.Match(set, method, fullPath)
}
