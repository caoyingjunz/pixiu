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

package menu

import "sort"

// ResolveOptions 菜单解析选项。
type ResolveOptions struct {
	IsRoot        bool
	IsAdmin       bool     // 内置管理员（RoleAdmin），可获得 AdminOnly 菜单
	Buttons       []string // METHOD:path
	ExplicitMenus []string // role_menus 显式绑定；非空时优先使用
}

// Resolve 计算用户可见菜单码集合（含父目录）。
// 优先级：超管全量 > 显式 role_menus > 由 API buttons 推导（兼容存量角色）。
func Resolve(opt ResolveOptions) []string {
	catalog := Catalog()
	byCode := ByCode()
	granted := make(map[string]struct{})

	if opt.IsRoot {
		for _, d := range catalog {
			granted[d.Code] = struct{}{}
		}
		return sortedKeys(granted)
	}

	// 公共菜单始终可见
	for _, d := range catalog {
		if d.Public {
			granted[d.Code] = struct{}{}
		}
	}

	if len(opt.ExplicitMenus) > 0 {
		valid := ValidCodes()
		for _, code := range opt.ExplicitMenus {
			if _, ok := valid[code]; !ok {
				continue
			}
			granted[code] = struct{}{}
		}
	} else {
		buttonSet := make(map[string]struct{}, len(opt.Buttons))
		for _, b := range opt.Buttons {
			buttonSet[b] = struct{}{}
		}
		for _, d := range catalog {
			if d.Kind == KindDirectory {
				continue
			}
			if d.Public {
				continue
			}
			if d.AdminOnly {
				if opt.IsAdmin {
					granted[d.Code] = struct{}{}
				}
				continue
			}
			if hasAnyAPI(buttonSet, d.RequiredAPIs) {
				granted[d.Code] = struct{}{}
			}
		}
	}

	// 补齐父目录：任一子项可见则父目录可见
	changed := true
	for changed {
		changed = false
		for code := range granted {
			d, ok := byCode[code]
			if !ok || d.ParentCode == "" {
				continue
			}
			if _, exists := granted[d.ParentCode]; !exists {
				granted[d.ParentCode] = struct{}{}
				changed = true
			}
		}
	}

	return sortedKeys(granted)
}

// DeriveFromButtons 仅按 API 推导叶子菜单（不含显式绑定逻辑），供角色管理预览。
func DeriveFromButtons(buttons []string, isAdmin bool) []string {
	return Resolve(ResolveOptions{Buttons: buttons, IsAdmin: isAdmin})
}

func hasAnyAPI(buttonSet map[string]struct{}, required []string) bool {
	if len(required) == 0 {
		return false
	}
	for _, api := range required {
		if _, ok := buttonSet[api]; ok {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
