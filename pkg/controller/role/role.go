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
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type RoleGetter interface {
	Role() Interface
}

type Interface interface {
	Create(ctx context.Context, req *types.CreateRoleRequest) error
	Update(ctx context.Context, rid int64, req *types.UpdateRoleRequest) error
	Delete(ctx context.Context, rid int64) error
	Get(ctx context.Context, rid int64) (*types.Role, error)
	List(ctx context.Context, req *types.ListRoleRequest) (*types.PageResponse, error)
}

type role struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func (r *role) Create(ctx context.Context, req *types.CreateRoleRequest) error {
	tenantId := int64(0)
	if req.TenantId != nil {
		tenantId = *req.TenantId
	}

	object, err := r.factory.Role().GetRoleByTenantAndName(ctx, tenantId, req.Name)
	if err != nil {
		klog.Errorf("failed to get role %s: %v", req.Name, err)
		return errors.ErrServerInternal
	}
	if object != nil {
		return errors.ErrRoleExists
	}

	if tenantId > 0 {
		tenant, err := r.factory.Tenant().Get(ctx, tenantId)
		if err != nil {
			klog.Errorf("failed to get tenant %d: %v", tenantId, err)
			return errors.ErrServerInternal
		}
		if tenant == nil {
			return errors.ErrTenantNotFound
		}
	}

	roleObj := &model.Role{
		TenantId: tenantId,
		Name:     req.Name,
	}
	if _, err = r.factory.Role().Create(ctx, roleObj); err != nil {
		if utilerrors.IsUniqueConstraintError(err) {
			return errors.ErrRoleExists
		}
		klog.Errorf("failed to create role %s: %v", req.Name, err)
		return errors.ErrServerInternal
	}

	return nil
}

func (r *role) Update(ctx context.Context, rid int64, req *types.UpdateRoleRequest) error {
	object, err := r.factory.Role().Get(ctx, rid)
	if err != nil {
		klog.Errorf("failed to get role %d: %v", rid, err)
		return errors.ErrServerInternal
	}
	if object == nil {
		return errors.ErrRoleNotFound
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		if *req.Name != object.Name {
			existing, err := r.factory.Role().GetRoleByTenantAndName(ctx, object.TenantId, *req.Name)
			if err != nil {
				klog.Errorf("failed to get role %s: %v", *req.Name, err)
				return errors.ErrServerInternal
			}
			if existing != nil && existing.Id != rid {
				return errors.ErrRoleExists
			}
		}
		updates["name"] = *req.Name
	}
	if len(updates) == 0 {
		return errors.ErrInvalidRequest
	}

	if err := r.factory.Role().Update(ctx, rid, *req.ResourceVersion, updates); err != nil {
		if utilerrors.IsUniqueConstraintError(err) {
			return errors.ErrRoleExists
		}
		klog.Errorf("failed to update role %d: %v", rid, err)
		return errors.ErrServerInternal
	}

	return nil
}

func (r *role) Delete(ctx context.Context, rid int64) error {
	object, err := r.factory.Role().Delete(ctx, rid)
	if err != nil {
		klog.Errorf("failed to delete role %d: %v", rid, err)
		return errors.ErrServerInternal
	}
	if object == nil {
		return errors.ErrRoleNotFound
	}

	return nil
}

func (r *role) Get(ctx context.Context, rid int64) (*types.Role, error) {
	object, err := r.factory.Role().Get(ctx, rid)
	if err != nil {
		klog.Errorf("failed to get role %d: %v", rid, err)
		return nil, errors.ErrServerInternal
	}
	if object == nil {
		return nil, errors.ErrRoleNotFound
	}

	return r.model2Type(object), nil
}

func (r *role) List(ctx context.Context, req *types.ListRoleRequest) (*types.PageResponse, error) {
	opts := []db.Options{db.WithOrderByDesc()}
	if req != nil {
		if req.NameSelector != "" {
			opts = append(opts, db.WithNameLike(req.NameSelector))
		}
		if req.TenantId != nil {
			opts = append(opts, db.WithTenantId(*req.TenantId))
		}
	}

	total, err := r.factory.Role().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to get role counts: %v", err)
		return nil, errors.ErrServerInternal
	}

	pageReq := types.PageRequest{}
	if req != nil {
		pageReq = req.PageRequest
		if req.Page > 0 && req.Limit > 0 {
			opts = append(opts, db.WithOffset((req.Page-1)*req.Limit), db.WithLimit(req.Limit))
		}
	}

	objects, err := r.factory.Role().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to get roles: %v", err)
		return nil, errors.ErrServerInternal
	}

	rs := make([]types.Role, len(objects))
	for i, object := range objects {
		rs[i] = *r.model2Type(&object)
	}

	return &types.PageResponse{
		PageRequest: pageReq,
		Total:       int(total),
		Items:       rs,
	}, nil
}

func (r *role) model2Type(o *model.Role) *types.Role {
	return &types.Role{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		TenantId: o.TenantId,
		Name:     o.Name,
	}
}

func NewRole(cfg config.Config, f db.ShareDaoFactory) *role {
	return &role{
		cc:      cfg,
		factory: f,
	}
}
