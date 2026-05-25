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
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func (r *role) GetAPIs(ctx context.Context, rid int64) (*types.RoleAPIsResponse, error) {
	object, err := r.factory.Role().Get(ctx, rid)
	if err != nil {
		klog.Errorf("failed to get role %d: %v", rid, err)
		return nil, errors.ErrServerInternal
	}
	if object == nil {
		return nil, errors.ErrRoleNotFound
	}

	associatedIds, err := r.factory.RoleAPI().ListAPIIdsByRoleId(ctx, rid)
	if err != nil {
		klog.Errorf("failed to list role apis for role %d: %v", rid, err)
		return nil, errors.ErrServerInternal
	}

	associatedSet := make(map[int64]struct{}, len(associatedIds))
	for _, id := range associatedIds {
		associatedSet[id] = struct{}{}
	}

	apis, err := r.factory.API().List(ctx)
	if err != nil {
		klog.Errorf("failed to list apis: %v", err)
		return nil, errors.ErrServerInternal
	}

	resp := &types.RoleAPIsResponse{
		Associated:   make([]types.APIResource, 0, len(associatedIds)),
		Unassociated: make([]types.APIResource, 0, len(apis)-len(associatedIds)),
	}
	for i := range apis {
		api := r.apiModel2Type(&apis[i])
		if _, ok := associatedSet[apis[i].Id]; ok {
			resp.Associated = append(resp.Associated, *api)
		} else {
			resp.Unassociated = append(resp.Unassociated, *api)
		}
	}

	return resp, nil
}

func (r *role) UpdateAPIs(ctx context.Context, rid int64, req *types.UpdateRoleAPIsRequest) error {
	object, err := r.factory.Role().Get(ctx, rid)
	if err != nil {
		klog.Errorf("failed to get role %d: %v", rid, err)
		return errors.ErrServerInternal
	}
	if object == nil {
		return errors.ErrRoleNotFound
	}

	apiIds := dedupeInt64(req.APIIds)
	for _, apiId := range apiIds {
		api, err := r.factory.API().Get(ctx, apiId)
		if err != nil {
			klog.Errorf("failed to get api %d: %v", apiId, err)
			return errors.ErrServerInternal
		}
		if api == nil {
			return errors.ErrAPINotFound
		}
	}

	if err := r.factory.RoleAPI().ReplaceByRoleId(ctx, rid, apiIds); err != nil {
		klog.Errorf("failed to update role apis for role %d: %v", rid, err)
		return errors.ErrServerInternal
	}

	return nil
}

func (r *role) apiModel2Type(o *model.API) *types.APIResource {
	return &types.APIResource{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		Method:      o.Method,
		Path:        o.Path,
		Group:       o.Group,
		Description: o.Description,
	}
}

func dedupeInt64(ids []int64) []int64 {
	if len(ids) == 0 {
		return ids
	}

	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}

	return result
}
