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

package role

import (
	"context"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	rbacapi "github.com/caoyingjunz/pixiu/pkg/rbac/api"
	"github.com/caoyingjunz/pixiu/pkg/rbac/menu"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func (r *roleMenu) GetMenus(ctx context.Context, rid int64) (*types.RoleMenusResponse, error) {
	object, err := r.factory.Role().Get(ctx, rid)
	if err != nil {
		klog.Errorf("failed to get role %d: %v", rid, err)
		return nil, errors.ErrServerInternal
	}
	if object == nil {
		return nil, errors.ErrRoleNotFound
	}

	catalog := menu.Catalog()
	defs := make([]types.MenuResource, 0, len(catalog))
	for _, d := range catalog {
		defs = append(defs, types.MenuResource{
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

	explicit, err := r.factory.Role().Menu().ListMenuCodesByRoleId(ctx, rid)
	if err != nil {
		klog.Errorf("failed to list role menus for role %d: %v", rid, err)
		return nil, errors.ErrServerInternal
	}

	resp := &types.RoleMenusResponse{
		Catalog:    defs,
		Associated: make([]string, 0),
		Derived:    false,
	}

	if len(explicit) > 0 {
		resp.Associated = menu.Resolve(menu.ResolveOptions{ExplicitMenus: explicit})
		return resp, nil
	}

	// 尚无显式菜单绑定时，按 role_apis 推导，便于管理端预览与存量兼容
	apis, err := r.factory.API().GetByRoleId(ctx, rid)
	if err != nil {
		klog.Errorf("failed to list APIs for role %d menus: %v", rid, err)
		return nil, errors.ErrServerInternal
	}
	buttons := make([]string, 0, len(apis))
	for i := range apis {
		buttons = append(buttons, rbacapi.Button(apis[i].Method, apis[i].Path))
	}
	isAdmin := rid == int64(model.RoleAdmin)
	resp.Associated = menu.DeriveFromButtons(buttons, isAdmin)
	resp.Derived = true
	return resp, nil
}

func (r *roleMenu) UpdateMenus(ctx context.Context, rid int64, req *types.UpdateRoleMenusRequest) error {
	if _, err := preUpdateRole(ctx, r.factory, rid); err != nil {
		klog.Errorf("pre-update check failed for role(%d) menus: %v", rid, err)
		return err
	}

	valid := menu.ValidCodes()
	codes := make([]string, 0, len(req.MenuCodes))
	seen := make(map[string]struct{}, len(req.MenuCodes))
	for _, code := range req.MenuCodes {
		if code == "" {
			continue
		}
		if _, ok := valid[code]; !ok {
			return errors.ErrInvalidRequest
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, code)
	}

	if err := r.factory.Role().Menu().ReplaceByRoleId(ctx, rid, codes); err != nil {
		klog.Errorf("failed to update role menus for role %d: %v", rid, err)
		return errors.ErrServerInternal
	}
	return nil
}
