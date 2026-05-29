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
	"strconv"
	"strings"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const kubernetesAPIGroup = "Kubernetes 资源"

func (r *role) GetAPIScopes(ctx context.Context, rid int64) (*types.RoleAPIScopesResponse, error) {
	object, err := r.factory.Role().Get(ctx, rid)
	if err != nil {
		klog.Errorf("failed to get role %d: %v", rid, err)
		return nil, errors.ErrServerInternal
	}
	if object == nil {
		return nil, errors.ErrRoleNotFound
	}

	scopeModels, err := r.factory.RoleAPIScope().ListByRoleId(ctx, rid)
	if err != nil {
		klog.Errorf("failed to list role api scopes for role %d: %v", rid, err)
		return nil, errors.ErrServerInternal
	}

	apis, err := r.factory.API().List(ctx)
	if err != nil {
		klog.Errorf("failed to list apis: %v", err)
		return nil, errors.ErrServerInternal
	}

	resp := &types.RoleAPIScopesResponse{
		Scopes: make([]types.RoleAPIScopeRecord, 0, len(scopeModels)),
		Apis:   make([]types.APIResource, 0),
	}
	for _, s := range scopeModels {
		resp.Scopes = append(resp.Scopes, scopeModel2Record(&s))
	}
	for i := range apis {
		if apis[i].Group != kubernetesAPIGroup {
			continue
		}
		resp.Apis = append(resp.Apis, *r.apiModel2Type(&apis[i]))
	}

	return resp, nil
}

func (r *role) UpdateAPIScopes(ctx context.Context, rid int64, req *types.UpdateRoleAPIScopesRequest) error {
	object, err := r.factory.Role().Get(ctx, rid)
	if err != nil {
		klog.Errorf("failed to get role %d: %v", rid, err)
		return errors.ErrServerInternal
	}
	if object == nil {
		return errors.ErrRoleNotFound
	}

	scopes := dedupeScopes(req.Scopes)
	records := make([]model.RoleAPIScope, 0, len(scopes))
	for _, item := range scopes {
		api, err := r.factory.API().Get(ctx, item.APIId)
		if err != nil {
			klog.Errorf("failed to get api %d: %v", item.APIId, err)
			return errors.ErrServerInternal
		}
		if api == nil {
			return errors.ErrAPINotFound
		}
		if api.Group != kubernetesAPIGroup {
			return errors.ErrInvalidRequest
		}
		cluster := strings.TrimSpace(item.Cluster)
		namespace := strings.TrimSpace(item.Namespace)
		resourceName := normalizeResourceName(item.ResourceName)
		if cluster == "" || namespace == "" {
			return errors.ErrInvalidRequest
		}
		records = append(records, model.RoleAPIScope{
			RoleId:       rid,
			APIId:        item.APIId,
			Cluster:      cluster,
			Namespace:    namespace,
			ResourceName: resourceName,
		})
	}

	if err := r.factory.RoleAPIScope().ReplaceByRoleId(ctx, rid, records); err != nil {
		klog.Errorf("failed to update role api scopes for role %d: %v", rid, err)
		return errors.ErrServerInternal
	}

	return nil
}

func scopeModel2Record(s *model.RoleAPIScope) types.RoleAPIScopeRecord {
	return types.RoleAPIScopeRecord{
		APIId:        s.APIId,
		Cluster:      s.Cluster,
		Namespace:    s.Namespace,
		ResourceName: s.ResourceName,
	}
}

func normalizeResourceName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "*"
	}
	return name
}

func dedupeScopes(scopes []types.RoleAPIScopeRecord) []types.RoleAPIScopeRecord {
	if len(scopes) == 0 {
		return scopes
	}
	seen := make(map[string]struct{}, len(scopes))
	result := make([]types.RoleAPIScopeRecord, 0, len(scopes))
	for _, item := range scopes {
		cluster := strings.TrimSpace(item.Cluster)
		namespace := strings.TrimSpace(item.Namespace)
		resourceName := normalizeResourceName(item.ResourceName)
		key := scopeKey(item.APIId, cluster, namespace, resourceName)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, types.RoleAPIScopeRecord{
			APIId:        item.APIId,
			Cluster:      cluster,
			Namespace:    namespace,
			ResourceName: resourceName,
		})
	}
	return result
}

func scopeKey(apiId int64, cluster, namespace, resourceName string) string {
	return strconv.FormatInt(apiId, 10) + "\x00" + strings.Join([]string{
		strings.TrimSpace(cluster),
		strings.TrimSpace(namespace),
		resourceName,
	}, "\x00")
}
