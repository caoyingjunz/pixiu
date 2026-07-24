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

package event

import (
	"context"
	"fmt"
	"net/http"

	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type Interface interface {
	Get(ctx context.Context, eventId int64) (*types.AlertEvent, error)
	List(ctx context.Context, listOption types.ListOptions) (interface{}, error)
	UpdateStatus(ctx context.Context, eventId int64, req *types.UpdateAlertEventStatusRequest) error
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{cc: cfg, factory: f}
}

func (c *controller) Get(ctx context.Context, eventId int64) (*types.AlertEvent, error) {
	object, err := c.factory.Alert().Event().Get(ctx, eventId)
	if err != nil {
		klog.Errorf("failed to get alert event(%d): %v", eventId, err)
		return nil, apierrors.ErrServerInternal
	}
	if object == nil {
		return nil, apierrors.NewError(fmt.Errorf("alert event not found"), http.StatusNotFound)
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
	pageResult.Total, err = c.factory.Alert().Event().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to count alert events: %v", err)
		pageResult.Message = err.Error()
	}

	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts, []db.Options{
		db.WithModifyOrderByDesc(),
		db.WithOffset(offset),
		db.WithLimit(listOption.Limit),
	}...)

	objects, err := c.factory.Alert().Event().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list alert events: %v", err)
		pageResult.Message = err.Error()
		return nil, apierrors.ErrServerInternal
	}

	items := make([]types.AlertEvent, 0, len(objects))
	for i := range objects {
		items = append(items, *modelToType(&objects[i]))
	}
	pageResult.Items = items

	return pageResult, nil
}

func (c *controller) UpdateStatus(ctx context.Context, eventId int64, req *types.UpdateAlertEventStatusRequest) error {
	if err := c.factory.Alert().Event().Update(ctx, eventId, req.ResourceVersion, map[string]interface{}{"status": req.Status}); err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return apierrors.NewError(fmt.Errorf("alert event not found"), http.StatusNotFound)
		}
		klog.Errorf("failed to update alert event(%d): %v", eventId, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func buildListOpts(opt types.ListOptions) []db.Options {
	opts := []db.Options{
		db.WithAlertRuleId(opt.RuleId),
		db.WithAlertSeverity(opt.Severity),
		db.WithAlertClusterId(opt.ClusterId),
	}
	if opt.Status != nil && *opt.Status != 0 {
		opts = append(opts, db.WithAlertEventStatus(model.AlertEventStatus(*opt.Status)))
	}
	return opts
}

func modelToType(object *model.AlertEvent) *types.AlertEvent {
	return &types.AlertEvent{
		PixiuMeta: types.PixiuMeta{Id: object.Id, ResourceVersion: object.ResourceVersion},
		TimeMeta:  types.TimeMeta{GmtCreate: object.GmtCreate, GmtModified: object.GmtModified},
		RuleId:    object.RuleId, RuleName: object.RuleName, Status: object.Status, Severity: object.Severity,
		TriggerValue: object.TriggerValue, TriggerExpr: object.TriggerExpr,
		ResourceType: object.ResourceType, ResourceName: object.ResourceName,
		ResourceNamespace: object.ResourceNamespace, ClusterId: object.ClusterId, TenantId: object.TenantId,
		RecoverValue: object.RecoverValue, RecoverTime: object.RecoverTime,
		LastSentAt: object.LastSentAt, NotifyCurNumber: object.NotifyCurNumber,
		Labels: object.Labels, Annotations: object.Annotations, Extension: object.Extension,
	}
}
