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

	normalizedConfig, normalizedSeverity, normalizedDuration, ok := engine.NormalizeRuleConfig(req.RuleConfig, req.Severity)
	if !ok {
		return apierrors.NewError(
			fmt.Errorf("rule_config must contain prom_ql and at least one valid trigger condition (operators: > >= < <= = == != <>, followed by a number)"),
			http.StatusBadRequest,
		)
	}

	_, err := c.factory.Alert().Rule().Create(ctx, &model.AlertRule{
		Name:             req.Name,
		Description:      req.Description,
		RuleType:         req.RuleType,
		Duration:         normalizedDuration,
		EvalInterval:     engine.NormalizeEvalInterval(req.EvalInterval),
		NotifyRepeatStep: engine.ResolveNotifyRepeatStep(req.NotifyRepeatStep),
		NotifyMaxNumber:  engine.ResolveNotifyMaxNumber(req.NotifyMaxNumber),
		Severity:         normalizedSeverity,
		ScopeType:        req.ScopeType,
		ScopeValue:       req.ScopeValue,
		NotifyChannels:   req.NotifyChannels,
		NotifyTemplate:   req.NotifyTemplate,
		RuleConfig:       normalizedConfig,
		EnableDaysOfWeek: engine.NormalizeEnableDaysOfWeek(req.EnableDaysOfWeek),
		EnableStime:      engine.NormalizeEnableTime(req.EnableStime),
		EnableEtime:      engine.NormalizeEnableTime(req.EnableEtime),
		DatasourceId:     req.DatasourceId,
		Enabled:          enabled,
		CreatedBy:        ctrlutil.CurrentUserName(ctx),
		Extension:        req.Extension,
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
	if req.Duration != nil {
		updates["duration"] = *req.Duration
	}
	if req.EvalInterval != nil {
		updates["eval_interval"] = engine.NormalizeEvalInterval(*req.EvalInterval)
	}
	if req.NotifyRepeatStep != nil {
		updates["notify_repeat_step"] = engine.NormalizeNotifyRepeatStep(*req.NotifyRepeatStep)
	}
	if req.NotifyMaxNumber != nil {
		updates["notify_max_number"] = engine.NormalizeNotifyMaxNumber(*req.NotifyMaxNumber)
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
	if req.EnableDaysOfWeek != nil {
		updates["enable_days_of_week"] = engine.NormalizeEnableDaysOfWeek(*req.EnableDaysOfWeek)
	}
	if req.EnableStime != nil {
		updates["enable_stime"] = engine.NormalizeEnableTime(*req.EnableStime)
	}
	if req.EnableEtime != nil {
		updates["enable_etime"] = engine.NormalizeEnableTime(*req.EnableEtime)
	}
	if req.DatasourceId != nil {
		updates["datasource_id"] = *req.DatasourceId
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.Extension != nil {
		updates["extension"] = *req.Extension
	}

	if req.RuleConfig != nil || req.Severity != nil {
		current, err := c.factory.Alert().Rule().Get(ctx, ruleId)
		if err != nil {
			klog.Errorf("failed to get alert rule(%d) before update: %v", ruleId, err)
			return apierrors.ErrServerInternal
		}
		if current == nil {
			return apierrors.NewError(fmt.Errorf("alert rule not found"), http.StatusNotFound)
		}

		ruleConfig := current.RuleConfig
		severity := current.Severity
		if req.RuleConfig != nil {
			ruleConfig = *req.RuleConfig
		}
		if req.Severity != nil {
			severity = *req.Severity
		}

		normalizedConfig, normalizedSeverity, normalizedDuration, ok := engine.NormalizeRuleConfig(ruleConfig, severity)
		if !ok {
			return apierrors.NewError(
				fmt.Errorf("rule_config must contain prom_ql and at least one valid trigger condition (operators: > >= < <= = == != <>, followed by a number)"),
				http.StatusBadRequest,
			)
		}
		updates["rule_config"] = normalizedConfig
		updates["severity"] = normalizedSeverity
		if req.Duration == nil {
			updates["duration"] = normalizedDuration
		}
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
		Duration:         object.Duration,
		EvalInterval:     engine.NormalizeEvalInterval(object.EvalInterval),
		NotifyRepeatStep: engine.NormalizeNotifyRepeatStep(object.NotifyRepeatStep),
		NotifyMaxNumber:  engine.NormalizeNotifyMaxNumber(object.NotifyMaxNumber),
		Severity:         object.Severity, ScopeType: object.ScopeType, ScopeValue: object.ScopeValue,
		NotifyChannels:   object.NotifyChannels,
		NotifyTemplate:   object.NotifyTemplate,
		RuleConfig:       object.RuleConfig,
		EnableDaysOfWeek: object.EnableDaysOfWeek,
		EnableStime:      engine.NormalizeEnableTime(object.EnableStime),
		EnableEtime:      engine.NormalizeEnableTime(object.EnableEtime),
		DatasourceId:     object.DatasourceId,
		Enabled:          object.Enabled, CreatedBy: object.CreatedBy, Extension: object.Extension,
	}
}
