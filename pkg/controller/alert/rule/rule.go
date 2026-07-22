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
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

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
	Export(ctx context.Context, ids []int64) ([]byte, error)
	Import(ctx context.Context, data []byte) (*types.ImportAlertRulesResult, error)
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

	normalizedConfig, _, normalizedDuration, ok := engine.NormalizeRuleConfig(req.RuleConfig)
	if !ok {
		return apierrors.NewError(
			fmt.Errorf("rule_config must contain prom_ql and at least one valid trigger condition (operators: > >= < <= = == != <>, followed by a number)"),
			http.StatusBadRequest,
		)
	}
	labels, err := engine.NormalizeLabelsJSON(req.Labels)
	if err != nil {
		return apierrors.NewError(err, http.StatusBadRequest)
	}

	_, err = c.factory.Alert().Rule().Create(ctx, &model.AlertRule{
		Name:             req.Name,
		Description:      req.Description,
		RuleType:         req.RuleType,
		Duration:         normalizedDuration,
		EvalInterval:     engine.NormalizeEvalInterval(req.EvalInterval),
		NotifyRepeatStep: engine.ResolveNotifyRepeatStep(req.NotifyRepeatStep),
		NotifyMaxNumber:  engine.ResolveNotifyMaxNumber(req.NotifyMaxNumber),
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
		Labels:           labels,
	})
	if err != nil {
		klog.Errorf("failed to create alert rule(%s): %v", req.Name, err)
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
	if req.Labels != nil {
		labels, err := engine.NormalizeLabelsJSON(*req.Labels)
		if err != nil {
			return apierrors.NewError(err, http.StatusBadRequest)
		}
		updates["labels"] = labels
	}

	if req.RuleConfig != nil {
		current, err := c.factory.Alert().Rule().Get(ctx, ruleId)
		if err != nil {
			klog.Errorf("failed to get alert rule(%d) before update: %v", ruleId, err)
			return apierrors.ErrServerInternal
		}
		if current == nil {
			return apierrors.NewError(fmt.Errorf("alert rule not found"), http.StatusNotFound)
		}

		ruleConfig := current.RuleConfig
		if req.RuleConfig != nil {
			ruleConfig = *req.RuleConfig
		}

		normalizedConfig, _, normalizedDuration, ok := engine.NormalizeRuleConfig(ruleConfig)
		if !ok {
			return apierrors.NewError(
				fmt.Errorf("rule_config must contain prom_ql and at least one valid trigger condition (operators: > >= < <= = == != <>, followed by a number)"),
				http.StatusBadRequest,
			)
		}
		updates["rule_config"] = normalizedConfig
		if req.Duration == nil {
			updates["duration"] = normalizedDuration
		}
	}

	if len(updates) == 0 {
		klog.V(2).Infof("alert rule(%d): no fields to update", ruleId)
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

const (
	alertRuleExportAPIVersion = "pixiu.io/v1"
	alertRuleExportKind       = "AlertRuleList"
)

func (c *controller) Export(ctx context.Context, ids []int64) ([]byte, error) {
	if len(ids) == 0 {
		return nil, apierrors.NewError(fmt.Errorf("ids is required"), http.StatusBadRequest)
	}

	objects, err := c.factory.Alert().Rule().List(ctx, db.WithIDIn(ids...))
	if err != nil {
		klog.Errorf("failed to list alert rules for export: %v", err)
		return nil, apierrors.ErrServerInternal
	}
	if len(objects) == 0 {
		return nil, apierrors.NewError(fmt.Errorf("no alert rules found for export"), http.StatusNotFound)
	}

	// Batch collect datasource IDs and channel IDs across all rules.
	var dsIDs, channelIDs []int64
	for i := range objects {
		if objects[i].DatasourceId > 0 {
			dsIDs = append(dsIDs, objects[i].DatasourceId)
		}
		channelIDs = append(channelIDs, parseNotifyChannelIDs(objects[i].NotifyChannels)...)
	}

	// Batch lookup datasource names.
	dsNameByID := make(map[int64]string, len(dsIDs))
	if len(dsIDs) > 0 {
		dss, err := c.factory.Datasource().List(ctx, db.WithIDIn(dsIDs...))
		if err == nil {
			for _, ds := range dss {
				dsNameByID[ds.Id] = ds.Name
			}
		}
	}

	// Batch lookup channel names.
	chNameByID := make(map[int64]string, len(channelIDs))
	if len(channelIDs) > 0 {
		channels, err := c.factory.Alert().Channel().List(ctx, db.WithIDIn(channelIDs...))
		if err == nil {
			for _, ch := range channels {
				chNameByID[ch.Id] = ch.Name
			}
		}
	}

	items := make([]types.AlertRuleExportItem, 0, len(objects))
	for i := range objects {
		exportItem, err := c.modelToExportItem(&objects[i], dsNameByID, chNameByID)
		if err != nil {
			return nil, apierrors.NewError(
				fmt.Errorf("failed to export rule(%d:%s): %w", objects[i].Id, objects[i].Name, err),
				http.StatusInternalServerError,
			)
		}
		items = append(items, exportItem)
	}

	payload := types.AlertRuleExportFile{
		APIVersion: alertRuleExportAPIVersion,
		Kind:       alertRuleExportKind,
		Items:      items,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		klog.Errorf("failed to marshal alert rules export file: %v", err)
		return nil, apierrors.ErrServerInternal
	}
	klog.Infof("exported %d alert rule(s)", len(items))
	return data, nil
}

func (c *controller) Import(ctx context.Context, data []byte) (*types.ImportAlertRulesResult, error) {
	items, err := parseAlertRuleImportPayload(data)
	if err != nil {
		return nil, apierrors.NewError(err, http.StatusBadRequest)
	}
	if len(items) == 0 {
		return nil, apierrors.NewError(fmt.Errorf("import file contains no alert rules"), http.StatusBadRequest)
	}

	result := &types.ImportAlertRulesResult{}

	// Batch load existing rule names for idempotency check.
	existingNames := make(map[string]struct{})
	existingRules, err := c.factory.Alert().Rule().List(ctx)
	if err == nil {
		for _, r := range existingRules {
			existingNames[r.Name] = struct{}{}
		}
	}

	// Batch load datasource name→ID mapping once for all items.
	datasourceNameToID := make(map[string]int64)
	dss, err := c.factory.Datasource().List(ctx)
	if err == nil {
		for _, ds := range dss {
			datasourceNameToID[ds.Name] = ds.Id
		}
	}

	// Batch load channel name→ID mapping once for all items.
	channelNameToID := make(map[string]int64)
	channels, err := c.factory.Alert().Channel().List(ctx)
	if err == nil {
		for _, ch := range channels {
			channelNameToID[ch.Name] = ch.Id
		}
	}

	for i := range items {
		item := items[i]
		// 跳过已存在的告警规则，以名称区分
		if _, exists := existingNames[item.Name]; exists {
			result.Skipped++
			klog.V(2).Infof("import rule(%s): already exists, skipped", item.Name)
			continue
		}

		if err = c.resolveImportIDs(&item, datasourceNameToID, channelNameToID); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", item.Name, err))
			klog.Errorf("failed to resolve import ids for rule(%s): %v", item.Name, err)
			continue
		}
		if err = c.Create(ctx, &item.CreateAlertRuleRequest); err != nil {
			result.Failed++
			name := item.Name
			if name == "" {
				name = fmt.Sprintf("#%d", i+1)
			}
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", name, err))
			klog.Errorf("failed to import alert rule(%s): %v", name, err)
			continue
		}
		result.Created++
	}
	klog.Infof("imported alert rules: created=%d skipped=%d failed=%d warnings=%d", result.Created, result.Skipped, result.Failed, len(result.Warnings))
	return result, nil
}

func (c *controller) resolveImportIDs(item *types.AlertRuleExportItem, datasourceNameToID map[string]int64, channelNameToID map[string]int64) error {
	// 1. Resolve datasource_id by name
	if item.DatasourceName != "" {
		id, ok := datasourceNameToID[item.DatasourceName]
		if !ok {
			return fmt.Errorf("datasource %q not found in target system", item.DatasourceName)
		}
		klog.V(2).Infof("import rule(%s): remapped datasource_id %d -> %d (%s)",
			item.Name, item.DatasourceId, id, item.DatasourceName)
		item.DatasourceId = id
	}

	// 2. Resolve notify_channels by names
	if item.NotifyChannelNames == "" {
		return nil
	}
	names := splitAndTrim(item.NotifyChannelNames, ",")
	ids := make([]string, 0, len(names))
	for _, name := range names {
		id, ok := channelNameToID[name]
		if !ok {
			return fmt.Errorf("channel %q not found in target system", name)
		}
		ids = append(ids, strconv.FormatInt(id, 10))
	}
	item.NotifyChannels = strings.Join(ids, ",")
	return nil
}

func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseAlertRuleImportPayload(data []byte) ([]types.AlertRuleExportItem, error) {
	var file types.AlertRuleExportFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("invalid alert rule import json: %w", err)
	}
	if file.Kind != "" && file.Kind != alertRuleExportKind {
		return nil, fmt.Errorf("unsupported kind %q, want %s", file.Kind, alertRuleExportKind)
	}

	return file.Items, nil
}

func (c *controller) modelToExportItem(object *model.AlertRule, dsNameByID map[int64]string, chNameByID map[int64]string) (types.AlertRuleExportItem, error) {
	// Resolve datasource name from pre-built batch lookup map.
	var datasourceName string
	if object.DatasourceId > 0 {
		name, ok := dsNameByID[object.DatasourceId]
		if !ok {
			return types.AlertRuleExportItem{}, fmt.Errorf("datasource %d not found", object.DatasourceId)
		}
		datasourceName = name
	}

	// Resolve channel names from pre-built batch lookup map.
	var notifyChannelNames string
	channelIDs := parseNotifyChannelIDs(object.NotifyChannels)
	if len(channelIDs) > 0 {
		names := make([]string, 0, len(channelIDs))
		for _, cid := range channelIDs {
			name, ok := chNameByID[cid]
			if !ok {
				return types.AlertRuleExportItem{}, fmt.Errorf("channel %d not found", cid)
			}
			names = append(names, name)
		}
		notifyChannelNames = strings.Join(names, ",")
	}

	enabled := object.Enabled
	notifyRepeatStep := engine.NormalizeNotifyRepeatStep(object.NotifyRepeatStep)
	notifyMaxNumber := engine.NormalizeNotifyMaxNumber(object.NotifyMaxNumber)

	return types.AlertRuleExportItem{
		CreateAlertRuleRequest: types.CreateAlertRuleRequest{
			Name:             object.Name,
			Description:      object.Description,
			RuleType:         object.RuleType,
			Duration:         object.Duration,
			EvalInterval:     engine.NormalizeEvalInterval(object.EvalInterval),
			NotifyRepeatStep: &notifyRepeatStep,
			NotifyMaxNumber:  &notifyMaxNumber,
			ScopeType:        object.ScopeType,
			ScopeValue:       object.ScopeValue,
			NotifyChannels:   object.NotifyChannels,
			NotifyTemplate:   object.NotifyTemplate,
			RuleConfig:       object.RuleConfig,
			EnableDaysOfWeek: object.EnableDaysOfWeek,
			EnableStime:      engine.NormalizeEnableTime(object.EnableStime),
			EnableEtime:      engine.NormalizeEnableTime(object.EnableEtime),
			DatasourceId:     object.DatasourceId,
			Enabled:          &enabled,
			Extension:        object.Extension,
			Labels:           object.Labels,
		},
		DatasourceName:     datasourceName,
		NotifyChannelNames: notifyChannelNames,
	}, nil
}

func parseNotifyChannelIDs(raw string) []int64 {
	parts := strings.Split(raw, ",")
	ids := make([]int64, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		ids = append(ids, id)
	}
	return ids
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
		ScopeType:        object.ScopeType, ScopeValue: object.ScopeValue,
		NotifyChannels:   object.NotifyChannels,
		NotifyTemplate:   object.NotifyTemplate,
		RuleConfig:       object.RuleConfig,
		EnableDaysOfWeek: object.EnableDaysOfWeek,
		EnableStime:      engine.NormalizeEnableTime(object.EnableStime),
		EnableEtime:      engine.NormalizeEnableTime(object.EnableEtime),
		DatasourceId:     object.DatasourceId,
		Enabled:          object.Enabled, CreatedBy: object.CreatedBy, Extension: object.Extension,
		Labels: object.Labels,
	}
}
