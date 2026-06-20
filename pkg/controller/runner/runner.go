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

package runner

import (
	"context"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type RunnerGetter interface {
	Runner() Interface
}

type Interface interface {
	Create(ctx context.Context, req *types.CreateRunnerRequest) error
	Update(ctx context.Context, req *types.UpdateRunnerRequest) error
	Delete(ctx context.Context, runnerId int64) error
	Get(ctx context.Context, runnerId int64) (*types.Runner, error)
	List(ctx context.Context, listOption types.ListOptions) (interface{}, error)
}

type runnerController struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func NewRunner(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &runnerController{
		cc:      cfg,
		factory: f,
	}
}

func (r *runnerController) Create(ctx context.Context, req *types.CreateRunnerRequest) error {
	object := &model.Runner{
		Name:        req.Name,
		EngineImage: req.EngineImage,
		Status:      req.Status,
		Description: req.Description,
	}

	if _, err := r.factory.Runner().Create(ctx, object); err != nil {
		klog.Errorf("failed to create runner %s: %v", req.Name, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (r *runnerController) Update(ctx context.Context, req *types.UpdateRunnerRequest) error {
	runnerId := req.Id

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.EngineImage != nil {
		updates["engine_image"] = *req.EngineImage
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}

	if err := r.factory.Runner().Update(ctx, runnerId, req.ResourceVersion, updates); err != nil {
		klog.Errorf("failed to update runner %d: %v", runnerId, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (r *runnerController) Delete(ctx context.Context, runnerId int64) error {
	if _, err := r.factory.Runner().Delete(ctx, runnerId); err != nil {
		klog.Errorf("failed to delete runner %d: %v", runnerId, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (r *runnerController) Get(ctx context.Context, runnerId int64) (*types.Runner, error) {
	object, err := r.factory.Runner().Get(ctx, runnerId)
	if err != nil {
		klog.Errorf("failed to get runner %d: %v", runnerId, err)
		return nil, errors.ErrServerInternal
	}
	if object == nil {
		return nil, errors.ErrRunnerNotFound
	}
	return model2Type(object), nil
}

func (r *runnerController) List(ctx context.Context, listOption types.ListOptions) (interface{}, error) {
	listOption.SetDefaultPageOption()

	pageResult := types.PageResult{
		PageRequest: types.PageRequest{
			Page:  listOption.Page,
			Limit: listOption.Limit,
		},
	}

	opts := []db.Options{
		db.WithNameLike(listOption.NameSelector),
	}

	var err error
	pageResult.Total, err = r.factory.Runner().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to get runners count: %v", err)
		pageResult.Message = err.Error()
		return nil, err
	}

	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts, []db.Options{
		db.WithModifyOrderByDesc(),
		db.WithOffset(offset),
		db.WithLimit(listOption.Limit),
	}...)

	objects, err := r.factory.Runner().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list runners: %v", err)
		pageResult.Message = err.Error()
		return nil, errors.ErrServerInternal
	}

	var ts []types.Runner
	for _, object := range objects {
		ts = append(ts, *model2Type(&object))
	}
	pageResult.Items = ts
	return pageResult, nil
}

func model2Type(o *model.Runner) *types.Runner {
	return &types.Runner{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		Name:        o.Name,
		EngineImage: o.EngineImage,
		Status:      o.Status,
		Description: o.Description,
	}
}
