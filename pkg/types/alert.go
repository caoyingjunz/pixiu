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

package types

import (
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

type AlertRule struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Name             string               `json:"name"`
	Description      string               `json:"description"`
	RuleType         model.AlertRuleType  `json:"rule_type"`
	Duration         int                  `json:"duration"`
	EvalInterval     int                  `json:"eval_interval"`
	NotifyRepeatStep int                  `json:"notify_repeat_step"` // 警报间隔时间（分钟），0 表示不重复
	NotifyMaxNumber  int                  `json:"notify_max_number"`  // 最大通知次数，0 表示不限制
	ScopeType        model.AlertScopeType `json:"scope_type"`
	ScopeValue       string               `json:"scope_value"`
	NotifyChannels   string               `json:"notify_channels"` // 关联 alert_channels 表 ID，逗号分隔，如 "1,2"
	NotifyTemplate   string               `json:"notify_template"`
	RuleConfig       string               `json:"rule_config"`         // JSON，多条告警条件
	EnableDaysOfWeek string               `json:"enable_days_of_week"` // 空格分隔，如 "1 2 3 4 5"；空表示全天
	EnableStime      string               `json:"enable_stime"`        // HH:mm，默认 00:00
	EnableEtime      string               `json:"enable_etime"`        // HH:mm，默认 00:00（与开始相同时表示整天）
	DatasourceId     int64                `json:"datasource_id"`
	Enabled          bool                 `json:"enabled"`
	CreatedBy        string               `json:"created_by"`
	Extension        string               `json:"extension"`
}

type AlertEvent struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	RuleId            int64                  `json:"rule_id"`
	RuleName          string                 `json:"rule_name"`
	Status            model.AlertEventStatus `json:"status"`
	Severity          model.AlertSeverity    `json:"severity"`
	TriggerValue      string                 `json:"trigger_value"`
	TriggerExpr       string                 `json:"trigger_expr"`
	ResourceType      string                 `json:"resource_type"`
	ResourceName      string                 `json:"resource_name"`
	ResourceNamespace string                 `json:"resource_namespace"`
	ClusterId         int64                  `json:"cluster_id"`
	TenantId          int64                  `json:"tenant_id"`
	RecoverValue      string                 `json:"recover_value"`
	RecoverTime       *time.Time             `json:"recover_time"`
	LastSentAt        *time.Time             `json:"last_sent_at"`
	NotifyCurNumber   int                    `json:"notify_cur_number"`
	Labels            string                 `json:"labels"`
	Annotations       string                 `json:"annotations"`
	Extension         string                 `json:"extension"`
}

type AlertNotification struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	EventId    int64                         `json:"event_id"`
	RuleId     int64                         `json:"rule_id"`
	Channel    model.AlertNotifyChannel      `json:"channel"`
	Receiver   string                        `json:"receiver"`
	Title      string                        `json:"title"`
	Content    string                        `json:"content"`
	Status     model.AlertNotificationStatus `json:"status"`
	RetryCount int                           `json:"retry_count"`
	ErrorMsg   string                        `json:"error_msg"`
	Extension  string                        `json:"extension"`
	Severity   model.AlertSeverity           `json:"severity"`
	Labels     string                        `json:"labels"`
	ChannelName string                       `json:"channel_name"`
}

type AlertChannel struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	ChannelType model.AlertNotifyChannel `json:"channel_type"`
	Config      string                   `json:"config"`
	Enabled     bool                     `json:"enabled"`
	CreatedBy   string                   `json:"created_by"`
	Extension   string                   `json:"extension"`
}

type AlertSilence struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Name             string    `json:"name"`
	MatchLabels      string    `json:"match_labels"`
	MatchExpressions string    `json:"match_expressions"`
	StartsAt         time.Time `json:"starts_at"`
	EndsAt           time.Time `json:"ends_at"`
	Enabled          bool      `json:"enabled"`
	CreatedBy        string    `json:"created_by"`
	Comment          string    `json:"comment"`
	Extension        string    `json:"extension"`
}

type CreateAlertRuleRequest struct {
	Name             string               `json:"name" binding:"required"`
	Description      string               `json:"description"`
	RuleType         model.AlertRuleType  `json:"rule_type" binding:"required"`
	Duration         int                  `json:"duration"`
	EvalInterval     int                  `json:"eval_interval"`
	NotifyRepeatStep *int                 `json:"notify_repeat_step"` // nil → 默认 5；显式 0 → 不重复
	NotifyMaxNumber  *int                 `json:"notify_max_number"`  // nil/0 → 不限制
	ScopeType        model.AlertScopeType `json:"scope_type" binding:"required"`
	ScopeValue       string               `json:"scope_value"`
	NotifyChannels   string               `json:"notify_channels"` // 关联 alert_channels 表 ID，逗号分隔，如 "1,2"
	NotifyTemplate   string               `json:"notify_template"`
	RuleConfig       string               `json:"rule_config" binding:"required"`
	EnableDaysOfWeek string               `json:"enable_days_of_week"`
	EnableStime      string               `json:"enable_stime"`
	EnableEtime      string               `json:"enable_etime"`
	DatasourceId     int64                `json:"datasource_id"`
	Enabled          *bool                `json:"enabled"`
	Extension        string               `json:"extension"`
}

type UpdateAlertRuleRequest struct {
	PixiuMeta `json:",inline"`

	Name             *string               `json:"name"`
	Description      *string               `json:"description"`
	RuleType         *model.AlertRuleType  `json:"rule_type"`
	Duration         *int                  `json:"duration"`
	EvalInterval     *int                  `json:"eval_interval"`
	NotifyRepeatStep *int                  `json:"notify_repeat_step"`
	NotifyMaxNumber  *int                  `json:"notify_max_number"`
	ScopeType        *model.AlertScopeType `json:"scope_type"`
	ScopeValue       *string               `json:"scope_value"`
	NotifyChannels   *string               `json:"notify_channels"`
	NotifyTemplate   *string               `json:"notify_template"`
	RuleConfig       *string               `json:"rule_config"`
	EnableDaysOfWeek *string               `json:"enable_days_of_week"`
	EnableStime      *string               `json:"enable_stime"`
	EnableEtime      *string               `json:"enable_etime"`
	DatasourceId     *int64                `json:"datasource_id"`
	Enabled          *bool                 `json:"enabled"`
	Extension        *string               `json:"extension"`
}

type CreateAlertSilenceRequest struct {
	Name             string    `json:"name" binding:"required"`
	MatchLabels      string    `json:"match_labels"`
	MatchExpressions string    `json:"match_expressions"`
	StartsAt         time.Time `json:"starts_at" binding:"required"`
	EndsAt           time.Time `json:"ends_at" binding:"required"`
	Enabled          *bool     `json:"enabled"`
	Comment          string    `json:"comment"`
	Extension        string    `json:"extension"`
}

type UpdateAlertSilenceRequest struct {
	PixiuMeta `json:",inline"`

	Name             *string    `json:"name"`
	MatchLabels      *string    `json:"match_labels"`
	MatchExpressions *string    `json:"match_expressions"`
	StartsAt         *time.Time `json:"starts_at"`
	EndsAt           *time.Time `json:"ends_at"`
	Enabled          *bool      `json:"enabled"`
	Comment          *string    `json:"comment"`
	Extension        *string    `json:"extension"`
}

type UpdateAlertEventStatusRequest struct {
	ResourceVersion int64                  `json:"resource_version" binding:"required"`
	Status          model.AlertEventStatus `json:"status" binding:"required"`
}

type CreateAlertChannelRequest struct {
	Name        string                   `json:"name" binding:"required"`
	Description string                   `json:"description"`
	ChannelType model.AlertNotifyChannel `json:"channel_type" binding:"required"`
	Config      string                   `json:"config"`
	Enabled     *bool                    `json:"enabled"`
	Extension   string                   `json:"extension"`
}

type UpdateAlertChannelRequest struct {
	PixiuMeta `json:",inline"`

	Name        *string                   `json:"name"`
	Description *string                   `json:"description"`
	ChannelType *model.AlertNotifyChannel `json:"channel_type"`
	Config      *string                   `json:"config"`
	Enabled     *bool                     `json:"enabled"`
	Extension   *string                   `json:"extension"`
}

type PingAlertChannelRequest struct {
	ChannelType model.AlertNotifyChannel `json:"channel_type" binding:"required"`
	Config      string                   `json:"config"`
}
