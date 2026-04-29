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

package agent

import (
	"context"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type AgentGetter interface {
	Agent() Interface
}

type Interface interface {
	Create(ctx context.Context, req *types.CreateAgentRequest) error
	Update(ctx context.Context, agentId int64, req *types.UpdateAgentRequest) error
	Delete(ctx context.Context, agentId int64) error
	Get(ctx context.Context, agentId int64) (*types.Agent, error)
	List(ctx context.Context, listOption types.AgentListOptions) (interface{}, error)
}

type agentController struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func NewAgent(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &agentController{
		cc:      cfg,
		factory: f,
	}
}

func (a *agentController) Create(ctx context.Context, req *types.CreateAgentRequest) error {
	object := &model.Agent{
		Name:        req.Name,
		Status:      req.Status,
		UserId:      req.UserId,
		Description: req.Description,
	}

	if _, err := a.factory.Agent().Create(ctx, object); err != nil {
		klog.Errorf("failed to create agent %s: %v", req.Name, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (a *agentController) Update(ctx context.Context, agentId int64, req *types.UpdateAgentRequest) error {
	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.LastReportTime != nil {
		updates["last_report_time"] = *req.LastReportTime
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}

	if err := a.factory.Agent().Update(ctx, agentId, req.ResourceVersion, updates); err != nil {
		klog.Errorf("failed to update agent %d: %v", agentId, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (a *agentController) Delete(ctx context.Context, agentId int64) error {
	if err := a.factory.Agent().Delete(ctx, agentId); err != nil {
		klog.Errorf("failed to delete agent %d: %v", agentId, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (a *agentController) Get(ctx context.Context, agentId int64) (*types.Agent, error) {
	object, err := a.factory.Agent().Get(ctx, agentId)
	if err != nil {
		klog.Errorf("failed to get agent %d: %v", agentId, err)
		return nil, errors.ErrServerInternal
	}
	if object == nil {
		return nil, errors.ErrAgentNotFound
	}
	return model2Type(object), nil
}

func (a *agentController) List(ctx context.Context, listOption types.AgentListOptions) (interface{}, error) {
	filterOpts := buildFilterOpts(listOption)

	total, err := a.factory.Agent().Count(ctx, filterOpts...)
	if err != nil {
		klog.Errorf("failed to get agents count: %v", err)
		return nil, err
	}

	page := listOption.Page
	if page <= 0 {
		page = 1
	}
	limit := listOption.Limit
	if limit <= 0 {
		limit = 20
	}

	paginationOpts := append(filterOpts,
		db.WithOffset((page-1)*limit),
		db.WithLimit(limit),
		db.WithOrderByDesc(),
	)

	objects, err := a.factory.Agent().List(ctx, paginationOpts...)
	if err != nil {
		klog.Errorf("failed to list agents: %v", err)
		return nil, errors.ErrServerInternal
	}

	var ts []types.Agent
	for _, object := range objects {
		ts = append(ts, *model2Type(&object))
	}
	return types.PageResponse{
		PageRequest: listOption.PageRequest,
		Total:       int(total),
		Items:       ts,
	}, nil
}

func buildFilterOpts(opt types.AgentListOptions) []db.Options {
	var opts []db.Options
	if opt.NameSelector != "" {
		opts = append(opts, db.WithNameLike(opt.NameSelector))
	}
	if opt.UserId != 0 {
		opts = append(opts, db.WithUserId(opt.UserId))
	}
	if opt.Status != nil {
		opts = append(opts, db.WithStatus(*opt.Status))
	}
	return opts
}

func model2Type(o *model.Agent) *types.Agent {
	return &types.Agent{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		Name:           o.Name,
		Status:         o.Status,
		UserId:         o.UserId,
		LastReportTime: o.LastReportTime,
		Description:    o.Description,
	}
}
