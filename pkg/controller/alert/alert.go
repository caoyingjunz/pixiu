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

package alert

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

type Getter interface {
	Alert() Interface
}

type Interface interface {
	CreateRule(ctx context.Context, req *types.CreateAlertRuleRequest) error
	UpdateRule(ctx context.Context, ruleId int64, req *types.UpdateAlertRuleRequest) error
	DeleteRule(ctx context.Context, ruleId int64) error
	GetRule(ctx context.Context, ruleId int64) (*types.AlertRule, error)
	ListRules(ctx context.Context, listOption types.AlertListOptions) (interface{}, error)

	GetEvent(ctx context.Context, eventId int64) (*types.AlertEvent, error)
	ListEvents(ctx context.Context, listOption types.AlertListOptions) (interface{}, error)
	UpdateEventStatus(ctx context.Context, eventId int64, req *types.UpdateAlertEventStatusRequest) error

	ListNotifications(ctx context.Context, listOption types.AlertListOptions) (interface{}, error)

	CreateSilence(ctx context.Context, req *types.CreateAlertSilenceRequest) error
	UpdateSilence(ctx context.Context, silenceId int64, req *types.UpdateAlertSilenceRequest) error
	DeleteSilence(ctx context.Context, silenceId int64) error
	GetSilence(ctx context.Context, silenceId int64) (*types.AlertSilence, error)
	ListSilences(ctx context.Context, listOption types.AlertListOptions) (interface{}, error)
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{cc: cfg, factory: f}
}

func (c *controller) CreateRule(ctx context.Context, req *types.CreateAlertRuleRequest) error {
	createdBy := currentUserName(ctx)
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
		Severity:       req.Severity,
		ScopeType:      req.ScopeType,
		ScopeValue:     req.ScopeValue,
		NotifyChannels: req.NotifyChannels,
		NotifyTemplate: req.NotifyTemplate,
		Enabled:        enabled,
		CreatedBy:      createdBy,
		Extension:      req.Extension,
	})
	if err != nil {
		klog.Errorf("failed to create alert rule: %v", err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) UpdateRule(ctx context.Context, ruleId int64, req *types.UpdateAlertRuleRequest) error {
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

func (c *controller) DeleteRule(ctx context.Context, ruleId int64) error {
	if err := c.factory.Alert().Rule().Delete(ctx, ruleId); err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return apierrors.NewError(fmt.Errorf("alert rule not found"), http.StatusNotFound)
		}
		klog.Errorf("failed to delete alert rule(%d): %v", ruleId, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) GetRule(ctx context.Context, ruleId int64) (*types.AlertRule, error) {
	object, err := c.factory.Alert().Rule().Get(ctx, ruleId)
	if err != nil {
		klog.Errorf("failed to get alert rule(%d): %v", ruleId, err)
		return nil, apierrors.ErrServerInternal
	}
	if object == nil {
		return nil, apierrors.NewError(fmt.Errorf("alert rule not found"), http.StatusNotFound)
	}
	return ruleModelToType(object), nil
}

func (c *controller) ListRules(ctx context.Context, listOption types.AlertListOptions) (interface{}, error) {
	listOption.SetDefaultPageOption()
	pageResult := types.PageResult{PageRequest: types.PageRequest{Page: listOption.Page, Limit: listOption.Limit}}

	opts := []db.Options{
		db.WithAlertRuleNameLike(listOption.Name),
		db.WithAlertSeverity(listOption.Severity),
	}

	total, err := c.factory.Alert().Rule().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to count alert rules: %v", err)
		return nil, apierrors.ErrServerInternal
	}
	pageResult.Total = total

	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts, db.WithModifyOrderByDesc(), db.WithOffset(offset), db.WithLimit(listOption.Limit))
	objects, err := c.factory.Alert().Rule().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list alert rules: %v", err)
		return nil, apierrors.ErrServerInternal
	}

	items := make([]types.AlertRule, 0, len(objects))
	for i := range objects {
		items = append(items, *ruleModelToType(&objects[i]))
	}
	pageResult.Items = items
	return pageResult, nil
}

func (c *controller) GetEvent(ctx context.Context, eventId int64) (*types.AlertEvent, error) {
	object, err := c.factory.Alert().Event().Get(ctx, eventId)
	if err != nil {
		klog.Errorf("failed to get alert event(%d): %v", eventId, err)
		return nil, apierrors.ErrServerInternal
	}
	if object == nil {
		return nil, apierrors.NewError(fmt.Errorf("alert event not found"), http.StatusNotFound)
	}
	return eventModelToType(object), nil
}

func (c *controller) ListEvents(ctx context.Context, listOption types.AlertListOptions) (interface{}, error) {
	listOption.SetDefaultPageOption()
	pageResult := types.PageResult{PageRequest: types.PageRequest{Page: listOption.Page, Limit: listOption.Limit}}

	opts := []db.Options{
		db.WithAlertRuleId(listOption.RuleId),
		db.WithAlertEventStatus(listOption.Status),
		db.WithAlertSeverity(listOption.Severity),
		db.WithAlertClusterId(listOption.ClusterId),
	}
	total, err := c.factory.Alert().Event().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to count alert events: %v", err)
		return nil, apierrors.ErrServerInternal
	}
	pageResult.Total = total

	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts, db.WithModifyOrderByDesc(), db.WithOffset(offset), db.WithLimit(listOption.Limit))
	objects, err := c.factory.Alert().Event().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list alert events: %v", err)
		return nil, apierrors.ErrServerInternal
	}

	items := make([]types.AlertEvent, 0, len(objects))
	for i := range objects {
		items = append(items, *eventModelToType(&objects[i]))
	}
	pageResult.Items = items
	return pageResult, nil
}

func (c *controller) UpdateEventStatus(ctx context.Context, eventId int64, req *types.UpdateAlertEventStatusRequest) error {
	updates := map[string]interface{}{"status": req.Status}
	if err := c.factory.Alert().Event().Update(ctx, eventId, req.ResourceVersion, updates); err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return apierrors.NewError(fmt.Errorf("alert event not found"), http.StatusNotFound)
		}
		klog.Errorf("failed to update alert event(%d): %v", eventId, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) ListNotifications(ctx context.Context, listOption types.AlertListOptions) (interface{}, error) {
	listOption.SetDefaultPageOption()
	pageResult := types.PageResult{PageRequest: types.PageRequest{Page: listOption.Page, Limit: listOption.Limit}}

	opts := []db.Options{
		db.WithAlertRuleId(listOption.RuleId),
		db.WithAlertEventId(listOption.EventId),
	}
	total, err := c.factory.Alert().Notification().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to count alert notifications: %v", err)
		return nil, apierrors.ErrServerInternal
	}
	pageResult.Total = total

	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts, db.WithModifyOrderByDesc(), db.WithOffset(offset), db.WithLimit(listOption.Limit))
	objects, err := c.factory.Alert().Notification().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list alert notifications: %v", err)
		return nil, apierrors.ErrServerInternal
	}

	items := make([]types.AlertNotification, 0, len(objects))
	for i := range objects {
		items = append(items, *notificationModelToType(&objects[i]))
	}
	pageResult.Items = items
	return pageResult, nil
}

func (c *controller) CreateSilence(ctx context.Context, req *types.CreateAlertSilenceRequest) error {
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
		CreatedBy:        currentUserName(ctx),
		Comment:          req.Comment,
		Extension:        req.Extension,
	})
	if err != nil {
		klog.Errorf("failed to create alert silence: %v", err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) UpdateSilence(ctx context.Context, silenceId int64, req *types.UpdateAlertSilenceRequest) error {
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

func (c *controller) DeleteSilence(ctx context.Context, silenceId int64) error {
	if err := c.factory.Alert().Silence().Delete(ctx, silenceId); err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return apierrors.NewError(fmt.Errorf("alert silence not found"), http.StatusNotFound)
		}
		klog.Errorf("failed to delete alert silence(%d): %v", silenceId, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) GetSilence(ctx context.Context, silenceId int64) (*types.AlertSilence, error) {
	object, err := c.factory.Alert().Silence().Get(ctx, silenceId)
	if err != nil {
		klog.Errorf("failed to get alert silence(%d): %v", silenceId, err)
		return nil, apierrors.ErrServerInternal
	}
	if object == nil {
		return nil, apierrors.NewError(fmt.Errorf("alert silence not found"), http.StatusNotFound)
	}
	return silenceModelToType(object), nil
}

func (c *controller) ListSilences(ctx context.Context, listOption types.AlertListOptions) (interface{}, error) {
	listOption.SetDefaultPageOption()
	pageResult := types.PageResult{PageRequest: types.PageRequest{Page: listOption.Page, Limit: listOption.Limit}}

	opts := []db.Options{}
	total, err := c.factory.Alert().Silence().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to count alert silences: %v", err)
		return nil, apierrors.ErrServerInternal
	}
	pageResult.Total = total

	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts, db.WithModifyOrderByDesc(), db.WithOffset(offset), db.WithLimit(listOption.Limit))
	objects, err := c.factory.Alert().Silence().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list alert silences: %v", err)
		return nil, apierrors.ErrServerInternal
	}

	items := make([]types.AlertSilence, 0, len(objects))
	for i := range objects {
		items = append(items, *silenceModelToType(&objects[i]))
	}
	pageResult.Items = items
	return pageResult, nil
}

func currentUserName(ctx context.Context) string {
	user, err := httputils.GetUserFromRequest(ctx)
	if err != nil || user == nil {
		return ""
	}
	return user.Name
}

func ruleModelToType(object *model.AlertRule) *types.AlertRule {
	return &types.AlertRule{
		PixiuMeta: types.PixiuMeta{Id: object.Id, ResourceVersion: object.ResourceVersion},
		TimeMeta:  types.TimeMeta{GmtCreate: object.GmtCreate, GmtModified: object.GmtModified},
		Name: object.Name, Description: object.Description, RuleType: object.RuleType,
		MetricName: object.MetricName, ConditionExpr: object.ConditionExpr, Duration: object.Duration,
		Severity: object.Severity, ScopeType: object.ScopeType, ScopeValue: object.ScopeValue,
		NotifyChannels: object.NotifyChannels, NotifyTemplate: object.NotifyTemplate,
		Enabled: object.Enabled, CreatedBy: object.CreatedBy, Extension: object.Extension,
	}
}

func eventModelToType(object *model.AlertEvent) *types.AlertEvent {
	return &types.AlertEvent{
		PixiuMeta: types.PixiuMeta{Id: object.Id, ResourceVersion: object.ResourceVersion},
		TimeMeta:  types.TimeMeta{GmtCreate: object.GmtCreate, GmtModified: object.GmtModified},
		RuleId: object.RuleId, RuleName: object.RuleName, Status: object.Status, Severity: object.Severity,
		TriggerValue: object.TriggerValue, TriggerExpr: object.TriggerExpr,
		ResourceType: object.ResourceType, ResourceName: object.ResourceName,
		ResourceNamespace: object.ResourceNamespace, ClusterId: object.ClusterId, TenantId: object.TenantId,
		RecoverValue: object.RecoverValue, RecoverTime: object.RecoverTime,
		Labels: object.Labels, Annotations: object.Annotations, Extension: object.Extension,
	}
}

func notificationModelToType(object *model.AlertNotification) *types.AlertNotification {
	return &types.AlertNotification{
		PixiuMeta: types.PixiuMeta{Id: object.Id, ResourceVersion: object.ResourceVersion},
		TimeMeta:  types.TimeMeta{GmtCreate: object.GmtCreate, GmtModified: object.GmtModified},
		EventId: object.EventId, RuleId: object.RuleId, Channel: object.Channel, Receiver: object.Receiver,
		Title: object.Title, Content: object.Content, Status: object.Status,
		RetryCount: object.RetryCount, ErrorMsg: object.ErrorMsg, Extension: object.Extension,
	}
}

func silenceModelToType(object *model.AlertSilence) *types.AlertSilence {
	return &types.AlertSilence{
		PixiuMeta: types.PixiuMeta{Id: object.Id, ResourceVersion: object.ResourceVersion},
		TimeMeta:  types.TimeMeta{GmtCreate: object.GmtCreate, GmtModified: object.GmtModified},
		Name: object.Name, MatchLabels: object.MatchLabels, MatchExpressions: object.MatchExpressions,
		StartsAt: object.StartsAt, EndsAt: object.EndsAt, Enabled: object.Enabled,
		CreatedBy: object.CreatedBy, Comment: object.Comment, Extension: object.Extension,
	}
}
