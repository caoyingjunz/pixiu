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
	"strings"

	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
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
	List(ctx context.Context, listOption types.ListOptions) (interface{}, error)
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{cc: cfg, factory: f}
}

func (c *controller) Create(ctx context.Context, req *types.CreateProviderRequest) error {
	if err := validateProviderFields(req.Name, req.BaseURL, req.Protocol); err != nil {
		return err
	}

	_, err := c.factory.Assistant().Provider().Create(ctx, &model.AIProvider{
		Name:        strings.TrimSpace(req.Name),
		BaseURL:     strings.TrimRight(strings.TrimSpace(req.BaseURL), "/"),
		Protocol:    normalizeProtocol(req.Protocol),
		Description: req.Description,
		MaxTokens:   resolveMaxTokens(req.MaxTokens, 0),
	})
	if err != nil {
		klog.Errorf("failed to create assistant provider: %v", err)
		return apierrors.ErrServerInternal
	}

	return nil
}

func (c *controller) Update(ctx context.Context, req *types.UpdateProviderRequest) error {
	if err := validateProviderFields(req.Name, req.BaseURL, req.Protocol); err != nil {
		return err
	}

	old, err := c.factory.Assistant().Provider().Get(ctx, req.Id)
	if err != nil {
		klog.Errorf("failed to get assistant provider(%d): %v", req.Id, err)
		return apierrors.ErrServerInternal
	}
	if old == nil {
		return apierrors.NewError(fmt.Errorf("assistant provider not found"), http.StatusNotFound)
	}
	if old.Builtin {
		return apierrors.NewError(fmt.Errorf("builtin assistant provider cannot be updated"), http.StatusForbidden)
	}

	updates := map[string]interface{}{
		"name":        strings.TrimSpace(req.Name),
		"base_url":    strings.TrimRight(strings.TrimSpace(req.BaseURL), "/"),
		"protocol":    normalizeProtocol(req.Protocol),
		"description": req.Description,
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
	old, err := c.factory.Assistant().Provider().Get(ctx, id)
	if err != nil {
		klog.Errorf("failed to get assistant provider(%d): %v", id, err)
		return apierrors.ErrServerInternal
	}
	if old == nil {
		return apierrors.NewError(fmt.Errorf("assistant provider not found"), http.StatusNotFound)
	}
	if old.Builtin {
		return apierrors.NewError(fmt.Errorf("builtin assistant provider cannot be deleted"), http.StatusForbidden)
	}
	accountCount, err := c.factory.Assistant().Account().Count(ctx, db.WithAIProviderId(id))
	if err != nil {
		klog.Errorf("failed to count ai accounts for provider(%d): %v", id, err)
		return apierrors.ErrServerInternal
	}
	if accountCount > 0 {
		return apierrors.NewError(fmt.Errorf("assistant provider still has accounts"), http.StatusConflict)
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
	object, err := c.factory.Assistant().Provider().Get(ctx, id)
	if err != nil {
		klog.Errorf("failed to get assistant provider(%d): %v", id, err)
		return nil, apierrors.ErrServerInternal
	}
	if object == nil {
		return nil, apierrors.NewError(fmt.Errorf("assistant provider not found"), http.StatusNotFound)
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

	opts := []db.Options{db.WithNameLike(listOption.NameSelector)}

	var err error
	pageResult.Total, err = c.factory.Assistant().Provider().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to count assistant providers: %v", err)
		return nil, apierrors.ErrServerInternal
	}

	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts,
		db.WithModifyOrderByDesc(),
		db.WithOffset(offset),
		db.WithLimit(listOption.Limit),
	)

	objects, err := c.factory.Assistant().Provider().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list assistant providers: %v", err)
		return nil, apierrors.ErrServerInternal
	}

	items := make([]types.AIProvider, 0)
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
		Name:        object.Name,
		BaseURL:     object.BaseURL,
		Protocol:    object.Protocol,
		Description: object.Description,
		MaxTokens:   resolveMaxTokens(object.MaxTokens, 0),
		Builtin:     object.Builtin,
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

func validateProviderFields(name, baseURL, protocol string) error {
	if strings.TrimSpace(name) == "" {
		return apierrors.NewError(fmt.Errorf("name is required"), http.StatusBadRequest)
	}
	if strings.TrimSpace(baseURL) == "" {
		return apierrors.NewError(fmt.Errorf("base_url is required"), http.StatusBadRequest)
	}
	if strings.TrimSpace(protocol) == "" {
		return apierrors.NewError(fmt.Errorf("protocol is required"), http.StatusBadRequest)
	}
	switch normalizeProtocol(protocol) {
	case "openai_chat", "openai_responses":
	default:
		return apierrors.NewError(fmt.Errorf("unsupported protocol %q", protocol), http.StatusBadRequest)
	}
	return nil
}

func normalizeProtocol(protocol string) string {
	return strings.ToLower(strings.TrimSpace(protocol))
}
