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

import "github.com/caoyingjunz/pixiu/pkg/types"

// ToResources 将目录定义转为 API 下发的 MenuResource 列表。
func ToResources(defs []Definition) []types.MenuResource {
	out := make([]types.MenuResource, 0, len(defs))
	for _, d := range defs {
		out = append(out, types.MenuResource{
			Code:         d.Code,
			ParentCode:   d.ParentCode,
			Title:        d.Title,
			Path:         d.Path,
			Kind:         d.Kind,
			Public:       d.Public,
			AdminOnly:    d.AdminOnly,
			RequiredAPIs: d.RequiredAPIs,
		})
	}
	return out
}

// CatalogResources 全量菜单目录（API 形态）。
func CatalogResources() []types.MenuResource {
	return ToResources(Catalog())
}
