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

package silence

import (
	"context"
	"fmt"
	"net/http"

	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	ctrlutil "github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type Interface interface {
	Create(ctx context.Context, req *types.CreateAlertSilenceRequest) error
	Update(ctx context.Context, silenceId int64, req *types.UpdateAlertSilenceRequest) error
	Delete(ctx context.Context, silenceId int64) error
	Get(ctx context.Context, silenceId int64) (*types.AlertSilence, error)
	List(ctx context.Context, listOption types.ListOptions) (interface{}, error)
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{cc: cfg, factory: f}
}

func (c *controller) Create(ctx context.Context, req *types.CreateAlertSilenceRequest) error {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	_, err := c.factory.Alert().Silence().Create(ctx, &model.AlertSilence{
		Name:             req.Name,
		MatchLabels:      req.MatchLabels,
		MatchExpressions: req.MatchExpressions,
		StartsAt:         req.StartsAt,
		EndsAt:           req.EndsAt,
		Enabled:          enabled,
		CreatedBy:        ctrlutil.CurrentUserName(ctx),
		Comment:          req.Comment,
		Extension:        req.Extension,
	})
	if err != nil {
		klog.Errorf("failed to create alert silence(%s): %v", req.Name, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) Update(ctx context.Context, silenceId int64, req *types.UpdateAlertSilenceRequest) error {
	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.MatchLabels != nil {
		updates["match_labels"] = *req.MatchLabels
	}
	if req.MatchExpressions != nil {
		updates["match_expressions"] = *req.MatchExpressions
	}
	if req.StartsAt != nil {
		updates["starts_at"] = *req.StartsAt
	}
	if req.EndsAt != nil {
		updates["ends_at"] = *req.EndsAt
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.Comment != nil {
		updates["comment"] = *req.Comment
	}
	if req.Extension != nil {
		updates["extension"] = *req.Extension
	}
	if len(updates) == 0 {
		klog.V(2).Infof("alert silence(%d): no fields to update", silenceId)
		return apierrors.NewError(fmt.Errorf("no fields to update"), http.StatusBadRequest)
	}
	if err := c.factory.Alert().Silence().Update(ctx, silenceId, req.ResourceVersion, updates); err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return apierrors.NewError(fmt.Errorf("alert silence not found"), http.StatusNotFound)
		}
		klog.Errorf("failed to update alert silence(%d): %v", silenceId, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) Delete(ctx context.Context, silenceId int64) error {
	if err := c.factory.Alert().Silence().Delete(ctx, silenceId); err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return apierrors.NewError(fmt.Errorf("alert silence not found"), http.StatusNotFound)
		}
		klog.Errorf("failed to delete alert silence(%d): %v", silenceId, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) Get(ctx context.Context, silenceId int64) (*types.AlertSilence, error) {
	object, err := c.factory.Alert().Silence().Get(ctx, silenceId)
	if err != nil {
		klog.Errorf("failed to get alert silence(%d): %v", silenceId, err)
		return nil, apierrors.ErrServerInternal
	}
	if object == nil {
		return nil, apierrors.NewError(fmt.Errorf("alert silence not found"), http.StatusNotFound)
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
	pageResult.Total, err = c.factory.Alert().Silence().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to count alert silences: %v", err)
		pageResult.Message = err.Error()
	}

	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts, []db.Options{
		db.WithModifyOrderByDesc(),
		db.WithOffset(offset),
		db.WithLimit(listOption.Limit),
	}...)

	objects, err := c.factory.Alert().Silence().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list alert silences: %v", err)
		pageResult.Message = err.Error()
		return nil, apierrors.ErrServerInternal
	}

	items := make([]types.AlertSilence, 0, len(objects))
	for i := range objects {
		items = append(items, *modelToType(&objects[i]))
	}
	pageResult.Items = items

	return pageResult, nil
}

func buildListOpts(opt types.ListOptions) []db.Options {
	opts := []db.Options{}
	if opt.Enabled != nil {
		opts = append(opts, db.WithEnabled(*opt.Enabled))
	}
	if opt.NameSelector != "" {
		opts = append(opts, db.WithNameLike(opt.NameSelector))
	}
	return opts
}

func modelToType(object *model.AlertSilence) *types.AlertSilence {
	return &types.AlertSilence{
		PixiuMeta: types.PixiuMeta{Id: object.Id, ResourceVersion: object.ResourceVersion},
		TimeMeta:  types.TimeMeta{GmtCreate: object.GmtCreate, GmtModified: object.GmtModified},
		Name:      object.Name, MatchLabels: object.MatchLabels, MatchExpressions: object.MatchExpressions,
		StartsAt: object.StartsAt, EndsAt: object.EndsAt, Enabled: object.Enabled,
		CreatedBy: object.CreatedBy, Comment: object.Comment, Extension: object.Extension,
	}
}
