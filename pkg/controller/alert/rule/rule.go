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

package rule

import (
	"context"
	"fmt"
	"net/http"

	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/controller/alert/engine"
	ctrlutil "github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type Interface interface {
	Create(ctx context.Context, req *types.CreateAlertRuleRequest) error
	Update(ctx context.Context, ruleId int64, req *types.UpdateAlertRuleRequest) error
	Delete(ctx context.Context, ruleId int64) error
	Get(ctx context.Context, ruleId int64) (*types.AlertRule, error)
	List(ctx context.Context, listOption types.ListOptions) (interface{}, error)
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{cc: cfg, factory: f}
}

func (c *controller) Create(ctx context.Context, req *types.CreateAlertRuleRequest) error {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	_, err := c.factory.Alert().Rule().Create(ctx, &model.AlertRule{
		Name:           req.Name,
		Description:    req.Description,
		RuleType:       req.RuleType,
		MetricName:     req.MetricName,
		ConditionExpr:  req.ConditionExpr,
		Duration:       req.Duration,
		EvalInterval:   engine.NormalizeEvalInterval(req.EvalInterval),
		Severity:       req.Severity,
		ScopeType:      req.ScopeType,
		ScopeValue:     req.ScopeValue,
		NotifyChannels: req.NotifyChannels,
		NotifyTemplate: req.NotifyTemplate,
		Enabled:        enabled,
		CreatedBy:      ctrlutil.CurrentUserName(ctx),
		Extension:      req.Extension,
	})
	if err != nil {
		klog.Errorf("failed to create alert rule: %v", err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) Update(ctx context.Context, ruleId int64, req *types.UpdateAlertRuleRequest) error {
	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.RuleType != nil {
		updates["rule_type"] = *req.RuleType
	}
	if req.MetricName != nil {
		updates["metric_name"] = *req.MetricName
	}
	if req.ConditionExpr != nil {
		updates["condition_expr"] = *req.ConditionExpr
	}
	if req.Duration != nil {
		updates["duration"] = *req.Duration
	}
	if req.EvalInterval != nil {
		updates["eval_interval"] = engine.NormalizeEvalInterval(*req.EvalInterval)
	}
	if req.Severity != nil {
		updates["severity"] = *req.Severity
	}
	if req.ScopeType != nil {
		updates["scope_type"] = *req.ScopeType
	}
	if req.ScopeValue != nil {
		updates["scope_value"] = *req.ScopeValue
	}
	if req.NotifyChannels != nil {
		updates["notify_channels"] = *req.NotifyChannels
	}
	if req.NotifyTemplate != nil {
		updates["notify_template"] = *req.NotifyTemplate
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

	if err := c.factory.Alert().Rule().Update(ctx, ruleId, req.ResourceVersion, updates); err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return apierrors.NewError(fmt.Errorf("alert rule not found"), http.StatusNotFound)
		}
		klog.Errorf("failed to update alert rule(%d): %v", ruleId, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) Delete(ctx context.Context, ruleId int64) error {
	if err := c.factory.Alert().Rule().Delete(ctx, ruleId); err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return apierrors.NewError(fmt.Errorf("alert rule not found"), http.StatusNotFound)
		}
		klog.Errorf("failed to delete alert rule(%d): %v", ruleId, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) Get(ctx context.Context, ruleId int64) (*types.AlertRule, error) {
	object, err := c.factory.Alert().Rule().Get(ctx, ruleId)
	if err != nil {
		klog.Errorf("failed to get alert rule(%d): %v", ruleId, err)
		return nil, apierrors.ErrServerInternal
	}
	if object == nil {
		return nil, apierrors.NewError(fmt.Errorf("alert rule not found"), http.StatusNotFound)
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

	opts := []db.Options{
		db.WithNameLike(listOption.NameSelector),
		db.WithAlertSeverity(listOption.Severity),
	}

	var err error
	pageResult.Total, err = c.factory.Alert().Rule().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to count alert rules: %v", err)
		pageResult.Message = err.Error()
	}

	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts, []db.Options{
		db.WithModifyOrderByDesc(),
		db.WithOffset(offset),
		db.WithLimit(listOption.Limit),
	}...)

	objects, err := c.factory.Alert().Rule().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list alert rules: %v", err)
		pageResult.Message = err.Error()
		return nil, apierrors.ErrServerInternal
	}

	items := make([]types.AlertRule, 0, len(objects))
	for i := range objects {
		items = append(items, *modelToType(&objects[i]))
	}
	pageResult.Items = items

	return pageResult, nil
}

func modelToType(object *model.AlertRule) *types.AlertRule {
	return &types.AlertRule{
		PixiuMeta: types.PixiuMeta{Id: object.Id, ResourceVersion: object.ResourceVersion},
		TimeMeta:  types.TimeMeta{GmtCreate: object.GmtCreate, GmtModified: object.GmtModified},
		Name:      object.Name, Description: object.Description, RuleType: object.RuleType,
		MetricName: object.MetricName, ConditionExpr: object.ConditionExpr, Duration: object.Duration,
		EvalInterval: engine.NormalizeEvalInterval(object.EvalInterval),
		Severity:     object.Severity, ScopeType: object.ScopeType, ScopeValue: object.ScopeValue,
		NotifyChannels: object.NotifyChannels, NotifyTemplate: object.NotifyTemplate,
		Enabled: object.Enabled, CreatedBy: object.CreatedBy, Extension: object.Extension,
	}
}
