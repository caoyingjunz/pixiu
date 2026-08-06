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

// Package api 提供平台 RBAC 的 METHOD:path 按钮码工具。
package api

// Endpoint 表示一条可授权的 HTTP 路由模板。
type Endpoint struct {
	Method string
	Path   string
}

// Button 生成标准权限码：METHOD:path（与 ValidAccess / 前端 hasAuth 对齐）。
func Button(method, path string) string {
	return method + ":" + path
}

// Buttons 将 Endpoint 列表转为 buttons 切片。
func Buttons(eps []Endpoint) []string {
	out := make([]string, 0, len(eps))
	for _, ep := range eps {
		if ep.Method == "" || ep.Path == "" {
			continue
		}
		out = append(out, Button(ep.Method, ep.Path))
	}
	return out
}

// BuildSet 构建 METHOD:path → 是否授权 的集合。
func BuildSet(eps []Endpoint) map[string]bool {
	set := make(map[string]bool, len(eps))
	for _, ep := range eps {
		if ep.Method == "" || ep.Path == "" {
			continue
		}
		set[Button(ep.Method, ep.Path)] = true
	}
	return set
}

// BuildSetFromButtons 从已格式化的 buttons 构建集合。
func BuildSetFromButtons(buttons []string) map[string]bool {
	set := make(map[string]bool, len(buttons))
	for _, b := range buttons {
		if b == "" {
			continue
		}
		set[b] = true
	}
	return set
}

// Match 判断集合是否允许指定 method+path。
func Match(set map[string]bool, method, path string) bool {
	if len(set) == 0 {
		return false
	}
	return set[Button(method, path)]
}
