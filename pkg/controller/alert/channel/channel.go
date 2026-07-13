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

package channel

import (
	"context"
	"fmt"
	"net/http"

	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/controller/alert/notify"
	ctrlutil "github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type Interface interface {
	Create(ctx context.Context, req *types.CreateAlertChannelRequest) error
	Update(ctx context.Context, channelId int64, req *types.UpdateAlertChannelRequest) error
	Delete(ctx context.Context, channelId int64) error
	Get(ctx context.Context, channelId int64) (*types.AlertChannel, error)
	List(ctx context.Context, listOption types.ListOptions) (interface{}, error)
	Ping(ctx context.Context, req *types.PingAlertChannelRequest) error
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{cc: cfg, factory: f}
}

func (c *controller) Create(ctx context.Context, req *types.CreateAlertChannelRequest) error {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	_, err := c.factory.Alert().Channel().Create(ctx, &model.AlertChannel{
		Name:        req.Name,
		Description: req.Description,
		ChannelType: req.ChannelType,
		Config:      req.Config,
		Enabled:     enabled,
		CreatedBy:   ctrlutil.CurrentUserName(ctx),
		Extension:   req.Extension,
	})
	if err != nil {
		klog.Errorf("failed to create alert channel: %v", err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) Update(ctx context.Context, channelId int64, req *types.UpdateAlertChannelRequest) error {
	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.ChannelType != nil {
		updates["channel_type"] = *req.ChannelType
	}
	if req.Config != nil {
		updates["config"] = *req.Config
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.Extension != nil {
		updates["extension"] = *req.Extension
	}
	if len(updates) == 0 {
		return apierrors.NewError(fmt.Errorf("no fields to update"), http.StatusBadRequest)
	}
	if err := c.factory.Alert().Channel().Update(ctx, channelId, req.ResourceVersion, updates); err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return apierrors.NewError(fmt.Errorf("alert channel not found"), http.StatusNotFound)
		}
		klog.Errorf("failed to update alert channel(%d): %v", channelId, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) Delete(ctx context.Context, channelId int64) error {
	if err := c.factory.Alert().Channel().Delete(ctx, channelId); err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return apierrors.NewError(fmt.Errorf("alert channel not found"), http.StatusNotFound)
		}
		klog.Errorf("failed to delete alert channel(%d): %v", channelId, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) Get(ctx context.Context, channelId int64) (*types.AlertChannel, error) {
	object, err := c.factory.Alert().Channel().Get(ctx, channelId)
	if err != nil {
		klog.Errorf("failed to get alert channel(%d): %v", channelId, err)
		return nil, apierrors.ErrServerInternal
	}
	if object == nil {
		return nil, apierrors.NewError(fmt.Errorf("alert channel not found"), http.StatusNotFound)
	}
	return modelToType(object), nil
}

func (c *controller) List(ctx context.Context, listOption types.ListOptions) (interface{}, error) {
	listOption.SetDefaultPageOption()

	pageResult := types.PageResult{
		PageRequest: types.PageRequest{
			Page:  listOption.Page,
			Limit: listOption.Limit,
		},
	}

	opts := buildListOpts(listOption)

	var err error
	pageResult.Total, err = c.factory.Alert().Channel().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to count alert channels: %v", err)
		pageResult.Message = err.Error()
	}

	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts, []db.Options{
		db.WithModifyOrderByDesc(),
		db.WithOffset(offset),
		db.WithLimit(listOption.Limit),
	}...)

	objects, err := c.factory.Alert().Channel().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list alert channels: %v", err)
		pageResult.Message = err.Error()
		return nil, apierrors.ErrServerInternal
	}

	items := make([]types.AlertChannel, 0, len(objects))
	for i := range objects {
		items = append(items, *modelToType(&objects[i]))
	}
	pageResult.Items = items

	return pageResult, nil
}

func (c *controller) Ping(ctx context.Context, req *types.PingAlertChannelRequest) error {
	if err := notify.PingChannel(req.ChannelType, req.Config); err != nil {
		klog.Errorf("ping channel connectivity failed (type=%d): %v", req.ChannelType, err)
		return apierrors.NewError(err, http.StatusBadRequest)
	}
	return nil
}

func buildListOpts(opt types.ListOptions) []db.Options {
	opts := []db.Options{
		db.WithNameLike(opt.NameSelector),
		db.WithAlertChannelType(opt.ChannelType),
	}
	if opt.Enabled != nil {
		opts = append(opts, db.WithEnabled(*opt.Enabled))
	}
	return opts
}

func modelToType(object *model.AlertChannel) *types.AlertChannel {
	return &types.AlertChannel{
		PixiuMeta: types.PixiuMeta{Id: object.Id, ResourceVersion: object.ResourceVersion},
		TimeMeta:  types.TimeMeta{GmtCreate: object.GmtCreate, GmtModified: object.GmtModified},
		Name:      object.Name, Description: object.Description, ChannelType: object.ChannelType,
		Config: object.Config, Enabled: object.Enabled, CreatedBy: object.CreatedBy, Extension: object.Extension,
	}
}
