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

package role

import (
	"context"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	rbacapi "github.com/caoyingjunz/pixiu/pkg/rbac/api"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func (r *roleAPIScope) GetAPIScopes(ctx context.Context, rid int64) (*types.RoleAPIScopesResponse, error) {
	object, err := r.factory.Role().Get(ctx, rid)
	if err != nil {
		klog.Errorf("failed to get role %d: %v", rid, err)
		return nil, errors.ErrServerInternal
	}
	if object == nil {
		return nil, errors.ErrRoleNotFound
	}

	scopes, err := r.factory.Role().Scope().ListScopes(ctx, rid)
	if err != nil {
		klog.Errorf("failed to list role api scopes for role %d: %v", rid, err)
		return nil, errors.ErrServerInternal
	}

	apis, err := r.factory.API().List(ctx)
	if err != nil {
		klog.Errorf("failed to list role apis for role %d: %v", rid, err)
		return nil, errors.ErrServerInternal
	}

	resp := &types.RoleAPIScopesResponse{
		Scopes: make([]types.RoleAPIScope, 0, len(scopes)),
		APIs:   make([]types.APIResource, 0),
	}
	for i := range scopes {
		resp.Scopes = append(resp.Scopes, types.RoleAPIScope{
			APIId:        scopes[i].APIId,
			ResourceType: scopes[i].ResourceType,
			ResourceId:   scopes[i].ResourceId,
		})
	}

	// 可选资源类型清单：暂返回全部持久化 API，前端 UI 后续阶段再细化
	for i := range apis {
		resp.APIs = append(resp.APIs, *apiModel2Type(&apis[i]))
	}
	return resp, nil
}

func (r *roleAPIScope) UpdateAPIScopes(ctx context.Context, rid int64, req *types.UpdateRoleAPIScopesRequest) error {
	if _, err := preUpdateRole(ctx, r.factory, rid); err != nil {
		klog.Errorf("pre-update check failed for role %d: %v", rid, err)
		return err
	}

	toModels := func(items []types.RoleAPIScope) []model.RoleAPIScope {
		out := make([]model.RoleAPIScope, 0, len(items))
		for i := range items {
			out = append(out, model.RoleAPIScope{
				APIId:        items[i].APIId,
				ResourceType: items[i].ResourceType,
				ResourceId:   items[i].ResourceId,
			})
		}
		return out
	}

	var err error
	if req.Scopes != nil {
		err = r.factory.Role().Scope().ReplaceScopes(ctx, rid, toModels(req.Scopes))
	} else {
		if len(req.RemoveScopes) > 0 {
			if err = r.factory.Role().Scope().RemoveScopes(ctx, rid, toModels(req.RemoveScopes)); err != nil {
				klog.Errorf("failed to remove role api scopes for role %d: %v", rid, err)
				return errors.ErrServerInternal
			}
		}
		if len(req.AddScopes) > 0 {
			if err = r.factory.Role().Scope().AddScopes(ctx, rid, toModels(req.AddScopes)); err != nil {
				klog.Errorf("failed to add role api scopes for role %d: %v", rid, err)
				return errors.ErrServerInternal
			}
		}
	}
	if err != nil {
		klog.Errorf("failed to update role api scopes for role %d: %v", rid, err)
		return errors.ErrServerInternal
	}

	// 确保 scopes 中的 api_id 同步进 role_apis，否则 ValidAccess 无法放行对应 HTTP API
	if err = r.syncRoleAPIsFromScopes(ctx, rid); err != nil {
		return err
	}
	return nil
}

func (r *roleAPIScope) syncRoleAPIsFromScopes(ctx context.Context, rid int64) error {
	scopes, err := r.factory.Role().Scope().ListScopes(ctx, rid)
	if err != nil {
		klog.Errorf("failed to list scopes when syncing role apis for role %d: %v", rid, err)
		return errors.ErrServerInternal
	}
	existing, err := r.factory.Role().API().ListAPIIdsByRoleId(ctx, rid)
	if err != nil {
		klog.Errorf("failed to list role apis when syncing for role %d: %v", rid, err)
		return errors.ErrServerInternal
	}

	scopeAPIIds := make([]int64, 0, len(scopes))
	for i := range scopes {
		scopeAPIIds = append(scopeAPIIds, scopes[i].APIId)
	}
	apiIds := rbacapi.MergeIDs(existing, scopeAPIIds)
	if err = r.factory.Role().API().ReplaceByRoleId(ctx, rid, apiIds); err != nil {
		klog.Errorf("failed to sync role apis from scopes for role %d: %v", rid, err)
		return errors.ErrServerInternal
	}
	return nil
}
