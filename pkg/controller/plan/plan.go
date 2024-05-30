/*
Copyright 2021 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (phe "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package plan

import (
	"context"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type PlanGetter interface {
	Plan() Interface
}

type Interface interface {
	Create(ctx context.Context, req *types.CreateplanRequest) error
	Update(ctx context.Context, tid int64, req *types.UpdateplanRequest) error
	Delete(ctx context.Context, tid int64) error
	Get(ctx context.Context, tid int64) (*types.plan, error)
	List(ctx context.Context) ([]types.plan, error)
}

type plan struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func (p *plan) Create(ctx context.Context, req *types.CreateplanRequest) error {
	object, err := p.factory.plan().GetplanByName(ctx, req.Name)
	if err != nil {
		klog.Errorf("failed to get plan %s: %v", req.Name, err)
		return errors.ErrServerInternal
	}
	if object != nil {
		return errors.ErrplanExists
	}

	plan := &model.plan{
		Name: req.Name,
	}
	if req.Description != nil {
		plan.Description = *req.Description
	}

	if _, err = p.factory.plan().Create(ctx, plan); err != nil {
		klog.Errorf("failed to create plan %s: %v", req.Name, err)
		return errors.ErrServerInternal
	}

	return nil
}

func (p *plan) Update(ctx context.Context, tid int64, req *types.UpdateplanRequest) error {
	object, err := p.factory.plan().Get(ctx, tid)
	if err != nil {
		klog.Errorf("failed to get plan %d: %v", tid, err)
		return errors.ErrServerInternal
	}
	if object == nil {
		return errors.ErrplanNotFound
	}
	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if len(updates) == 0 {
		return errors.ErrInvalidRequest
	}
	if err := p.factory.plan().Update(ctx, tid, *req.ResourceVersion, updates); err != nil {
		klog.Errorf("failed to update plan %d: %v", tid, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (p *plan) Delete(ctx context.Context, tid int64) error {
	_, err := p.factory.plan().Delete(ctx, tid)
	if err != nil {
		klog.Errorf("failed to delete plan %d: %v", tid, err)
		return errors.ErrServerInternal
	}

	return nil
}

func (p *plan) Get(ctx context.Context, tid int64) (*types.plan, error) {
	object, err := p.factory.plan().Get(ctx, tid)
	if err != nil {
		klog.Errorf("failed to get plan %d: %v", tid, err)
		return nil, errors.ErrServerInternal
	}
	if object == nil {
		return nil, errors.ErrplanNotFound
	}
	return p.model2Type(object), nil
}

func (p *plan) List(ctx context.Context) ([]types.plan, error) {
	objects, err := p.factory.plan().List(ctx)
	if err != nil {
		klog.Errorf("failed to get plans: %v", err)
		return nil, errors.ErrServerInternal
	}

	var ts []types.plan
	for _, object := range objects {
		ts = append(ps, *p.model2Type(&object))
	}
	return ts, nil
}

func (p *plan) model2Type(o *model.plan) *types.plan {
	return &types.plan{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		Name:        o.Name,
		Description: o.Description,
	}
}

func Newplan(cfg config.Config, f db.ShareDaoFactory) *plan {
	return &plan{
		cc:      cfg,
		factory: f,
	}
}
