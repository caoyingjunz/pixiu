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

package provider

import (
	"context"
	"fmt"
	"net/http"

	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type Interface interface {
	Create(ctx context.Context, req *types.CreateProviderRequest) error
	Update(ctx context.Context, req *types.UpdateProviderRequest) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*types.AIProvider, error)
	List(ctx context.Context, req *types.ListProviderRequest) (interface{}, error)
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{cc: cfg, factory: f}
}

func (c *controller) Create(ctx context.Context, req *types.CreateProviderRequest) error {
	userId, err := httputils.GetUserIdFromContext(ctx)
	if err != nil {
		return apierrors.ErrUnauthorized
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	_, err = c.factory.Assistant().Provider().Create(ctx, &model.AIProvider{
		APIKey:      req.APIKey,
		BaseURL:     req.BaseURL,
		Enabled:     enabled,
		Description: req.Description,
		MaxTokens:   resolveMaxTokens(req.MaxTokens, 0),
		ModelName:   req.Model,
		UserId:      userId,
		Provider:    req.Provider,
	})
	if err != nil {
		klog.Errorf("failed to create assistant provider for user(%d): %v", userId, err)
		return apierrors.ErrServerInternal
	}

	return nil
}

func (c *controller) Update(ctx context.Context, req *types.UpdateProviderRequest) error {
	userId, err := httputils.GetUserIdFromContext(ctx)
	if err != nil {
		return apierrors.ErrUnauthorized
	}

	old, err := c.factory.Assistant().Provider().Get(ctx, req.Id)
	if err != nil {
		klog.Errorf("failed to get assistant provider(%d): %v", req.Id, err)
		return apierrors.ErrServerInternal
	}
	if old == nil || old.UserId != userId {
		return apierrors.NewError(fmt.Errorf("assistant provider not found"), http.StatusNotFound)
	}

	enabled := old.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	updates := map[string]interface{}{
		"provider":    req.Provider,
		"api_key":     req.APIKey,
		"base_url":    req.BaseURL,
		"model":       req.Model,
		"description": req.Description,
		"enabled":     enabled,
	}
	if req.MaxTokens > 0 {
		updates["max_tokens"] = req.MaxTokens
	}

	if err = c.factory.Assistant().Provider().Update(ctx, req.Id, req.ResourceVersion, updates); err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return apierrors.NewError(fmt.Errorf("assistant provider not found or resource version conflict"), http.StatusConflict)
		}
		klog.Errorf("failed to update assistant provider(%d): %v", req.Id, err)
		return apierrors.ErrServerInternal
	}

	return nil
}

func (c *controller) Delete(ctx context.Context, id int64) error {
	userId, err := httputils.GetUserIdFromContext(ctx)
	if err != nil {
		return apierrors.ErrUnauthorized
	}

	old, err := c.factory.Assistant().Provider().Get(ctx, id)
	if err != nil {
		klog.Errorf("failed to get assistant provider(%d): %v", id, err)
		return apierrors.ErrServerInternal
	}
	if old == nil || old.UserId != userId {
		return apierrors.NewError(fmt.Errorf("assistant provider not found"), http.StatusNotFound)
	}

	if err = c.factory.Assistant().Provider().Delete(ctx, id); err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return apierrors.NewError(fmt.Errorf("assistant provider not found"), http.StatusNotFound)
		}
		klog.Errorf("failed to delete assistant provider(%d): %v", id, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) Get(ctx context.Context, id int64) (*types.AIProvider, error) {
	userId, err := httputils.GetUserIdFromContext(ctx)
	if err != nil {
		return nil, apierrors.ErrUnauthorized
	}

	object, err := c.factory.Assistant().Provider().Get(ctx, id)
	if err != nil {
		klog.Errorf("failed to get assistant provider(%d): %v", id, err)
		return nil, apierrors.ErrServerInternal
	}
	if object == nil || object.UserId != userId {
		return nil, apierrors.NewError(fmt.Errorf("assistant provider not found"), http.StatusNotFound)
	}
	return modelToType(object), nil
}

func (c *controller) List(ctx context.Context, req *types.ListProviderRequest) (interface{}, error) {
	userId, err := httputils.GetUserIdFromContext(ctx)
	if err != nil {
		return nil, apierrors.ErrUnauthorized
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	pageResult := types.PageResult{
		PageRequest: types.PageRequest{
			Page:  req.Page,
			Limit: req.Limit,
		},
	}

	opts := []db.Options{
		db.WithUser(userId),
		db.WithProvider(req.Provider),
	}
	if req.Enabled != nil {
		opts = append(opts, db.WithEnabled(*req.Enabled))
	}

	pageResult.Total, err = c.factory.Assistant().Provider().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to count assistant providers: %v", err)
		return nil, apierrors.ErrServerInternal
	}

	offset := (req.Page - 1) * req.Limit
	opts = append(opts,
		db.WithModifyOrderByDesc(),
		db.WithOffset(offset),
		db.WithLimit(req.Limit),
	)

	objects, err := c.factory.Assistant().Provider().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list assistant providers: %v", err)
		return nil, apierrors.ErrServerInternal
	}

	items := make([]types.AIProvider, 0, len(objects))
	for i := range objects {
		items = append(items, *modelToType(&objects[i]))
	}
	pageResult.Items = items

	return pageResult, nil
}

func modelToType(object *model.AIProvider) *types.AIProvider {
	return &types.AIProvider{
		PixiuMeta: types.PixiuMeta{
			Id:              object.Id,
			ResourceVersion: object.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   object.GmtCreate,
			GmtModified: object.GmtModified,
		},
		UserId:      object.UserId,
		Provider:    object.Provider,
		APIKey:      object.APIKey,
		BaseURL:     object.BaseURL,
		Model:       object.ModelName,
		Description: object.Description,
		Enabled:     object.Enabled,
		MaxTokens:   resolveMaxTokens(object.MaxTokens, 0),
	}
}

func resolveMaxTokens(value, fallback int) int {
	if value > 0 {
		return value
	}
	if fallback > 0 {
		return fallback
	}
	return 4096
}
