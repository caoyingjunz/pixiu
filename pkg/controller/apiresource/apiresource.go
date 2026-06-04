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

package apiresource

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

type APIResourceGetter interface {
	APIResource() Interface
}

type Interface interface {
	Create(ctx context.Context, req *types.CreateAPIRequest) error
	Update(ctx context.Context, aid int64, req *types.UpdateAPIRequest) error
	Delete(ctx context.Context, aid int64) error
	Get(ctx context.Context, aid int64) (*types.APIResource, error)
	List(ctx context.Context, req *types.ListAPIRequest) (*types.PageResponse, error)

	Register(ctx context.Context, req *types.CreateAPIRequest) error
}

type apiResource struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func (a *apiResource) Create(ctx context.Context, req *types.CreateAPIRequest) error {
	object, err := a.factory.API().GetByMethodAndPath(ctx, req.Method, req.Path)
	if err != nil {
		klog.Errorf("failed to get api %s %s: %v", req.Method, req.Path, err)
		return errors.ErrServerInternal
	}
	if object != nil {
		return errors.ErrAPIExists
	}

	apiObj := &model.API{
		Method: req.Method,
		Path:   req.Path,
	}
	if req.Group != nil {
		apiObj.Group = *req.Group
	}
	if req.Description != nil {
		apiObj.Description = *req.Description
	}

	if _, err = a.factory.API().Create(ctx, apiObj); err != nil {
		if utilerrors.IsUniqueConstraintError(err) {
			return errors.ErrAPIExists
		}
		klog.Errorf("failed to create api %s %s: %v", req.Method, req.Path, err)
		return errors.ErrServerInternal
	}

	return nil
}

// Register 幂等同步 API 资源：存在则更新分组和描述，不存在则创建
func (a *apiResource) Register(ctx context.Context, req *types.CreateAPIRequest) error {
	object, err := a.factory.API().GetByMethodAndPath(ctx, req.Method, req.Path)
	if err != nil {
		klog.Errorf("failed to get api %s %s for sync: %v", req.Method, req.Path, err)
		return err
	}

	if object != nil {
		updates := make(map[string]interface{})
		if req.Group != nil && *req.Group != object.Group {
			updates["api_group"] = *req.Group
		}
		if req.Description != nil && *req.Description != object.Description {
			updates["description"] = *req.Description
		}
		if len(updates) > 0 {
			if err := a.factory.API().Update(ctx, object.Id, object.ResourceVersion, updates); err != nil {
				klog.Errorf("failed to sync api %s %s: %v", req.Method, req.Path, err)
				return err
			}
		}
		return nil
	}

	return a.Create(ctx, req)
}

func (a *apiResource) Update(ctx context.Context, aid int64, req *types.UpdateAPIRequest) error {
	object, err := a.factory.API().Get(ctx, aid)
	if err != nil {
		klog.Errorf("failed to get api %d: %v", aid, err)
		return errors.ErrServerInternal
	}
	if object == nil {
		return errors.ErrAPINotFound
	}

	method := object.Method
	path := object.Path
	if req.Method != nil {
		method = *req.Method
	}
	if req.Path != nil {
		path = *req.Path
	}
	if method != object.Method || path != object.Path {
		existing, err := a.factory.API().GetByMethodAndPath(ctx, method, path)
		if err != nil {
			klog.Errorf("failed to get api %s %s: %v", method, path, err)
			return errors.ErrServerInternal
		}
		if existing != nil && existing.Id != aid {
			return errors.ErrAPIExists
		}
	}

	updates := make(map[string]interface{})
	if req.Method != nil {
		updates["method"] = *req.Method
	}
	if req.Path != nil {
		updates["path"] = *req.Path
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Group != nil {
		updates["api_group"] = *req.Group
	}
	if len(updates) == 0 {
		return errors.ErrInvalidRequest
	}

	if err := a.factory.API().Update(ctx, aid, *req.ResourceVersion, updates); err != nil {
		if utilerrors.IsUniqueConstraintError(err) {
			return errors.ErrAPIExists
		}
		klog.Errorf("failed to update api %d: %v", aid, err)
		return errors.ErrServerInternal
	}

	return nil
}

func (a *apiResource) Delete(ctx context.Context, aid int64) error {
	object, err := a.factory.API().Delete(ctx, aid)
	if err != nil {
		klog.Errorf("failed to delete api %d: %v", aid, err)
		return errors.ErrServerInternal
	}
	if object == nil {
		return errors.ErrAPINotFound
	}

	return nil
}

func (a *apiResource) Get(ctx context.Context, aid int64) (*types.APIResource, error) {
	object, err := a.factory.API().Get(ctx, aid)
	if err != nil {
		klog.Errorf("failed to get api %d: %v", aid, err)
		return nil, errors.ErrServerInternal
	}
	if object == nil {
		return nil, errors.ErrAPINotFound
	}

	return a.model2Type(object), nil
}

func (a *apiResource) List(ctx context.Context, req *types.ListAPIRequest) (*types.PageResponse, error) {
	opts := []db.Options{db.WithOrderByDesc()}
	if req != nil {
		opts = append(opts, db.WithMethod(req.Method), db.WithPathLike(req.PathSelector), db.WithGroup(req.Group))
	}

	total, err := a.factory.API().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to get api counts: %v", err)
		return nil, errors.ErrServerInternal
	}

	pageReq := types.PageRequest{}
	if req != nil {
		pageReq = req.PageRequest
		if req.Page > 0 && req.Limit > 0 {
			opts = append(opts, db.WithOffset((req.Page-1)*req.Limit), db.WithLimit(req.Limit))
		}
	}

	objects, err := a.factory.API().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to get apis: %v", err)
		return nil, errors.ErrServerInternal
	}

	apis := make([]types.APIResource, len(objects))
	for i, object := range objects {
		apis[i] = *a.model2Type(&object)
	}

	return &types.PageResponse{
		PageRequest: pageReq,
		Total:       int(total),
		Items:       apis,
	}, nil
}

func (a *apiResource) model2Type(o *model.API) *types.APIResource {
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

func NewAPIResource(cfg config.Config, f db.ShareDaoFactory) *apiResource {
	return &apiResource{
		cc:      cfg,
		factory: f,
	}
}
