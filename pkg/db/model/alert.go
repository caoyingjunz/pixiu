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

package model

import (
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"
)

func init() {
	register(
		&AlertRule{},
		&AlertEvent{},
		&AlertNotification{},
		&AlertChannel{},
		&AlertSilence{},
	)
}

type AlertRuleType int

const (
	AlertRuleTypeMetric AlertRuleType = 1
	AlertRuleTypeLog    AlertRuleType = 2

	// Deprecated aliases kept for existing data compatibility.
	AlertRuleTypeThreshold               = AlertRuleTypeMetric
	AlertRuleTypeStatus                  = AlertRuleTypeLog
	AlertRuleTypeEvent     AlertRuleType = 3
)

type AlertSeverity int

const (
	AlertSeverityInfo      AlertSeverity = 1
	AlertSeverityWarning   AlertSeverity = 2
	AlertSeverityCritical  AlertSeverity = 3
	AlertSeverityEmergency AlertSeverity = 4
)

type AlertScopeType int

const (
	AlertScopeGlobal  AlertScopeType = 1
	AlertScopeCluster AlertScopeType = 2
	AlertScopeTenant  AlertScopeType = 3
	AlertScopeCustom  AlertScopeType = 4
)

type AlertEventStatus int

const (
	AlertEventStatusFiring    AlertEventStatus = 1
	AlertEventStatusRecovered AlertEventStatus = 2
	AlertEventStatusAcked     AlertEventStatus = 3
	AlertEventStatusResolved  AlertEventStatus = 4
)

type AlertNotifyChannel int

const (
	AlertNotifyChannelEmail    AlertNotifyChannel = 1
	AlertNotifyChannelDingTalk AlertNotifyChannel = 2
	AlertNotifyChannelWeCom    AlertNotifyChannel = 3
	AlertNotifyChannelWebhook  AlertNotifyChannel = 4
	AlertNotifyChannelFeishu   AlertNotifyChannel = 5
)

type AlertNotificationStatus int

const (
	AlertNotificationStatusPending AlertNotificationStatus = 0
	AlertNotificationStatusSuccess AlertNotificationStatus = 1
	AlertNotificationStatusFailed  AlertNotificationStatus = 2
)

// AlertRule stores user-defined alert rules.
type AlertRule struct {
	pixiu.Model

	Name             string         `gorm:"column:name;type:varchar(128);not null;index:idx_alert_rules_name" json:"name"`
	Description      string         `gorm:"column:description;type:text" json:"description"`
	RuleType         AlertRuleType  `gorm:"column:rule_type;not null" json:"rule_type"`
	Duration         int            `gorm:"column:duration;default:0" json:"duration"`
	EvalInterval     int            `gorm:"column:eval_interval;default:15" json:"eval_interval"`
	Severity         AlertSeverity  `gorm:"column:severity;not null;index:idx_alert_rules_severity" json:"severity"`
	ScopeType        AlertScopeType `gorm:"column:scope_type;not null" json:"scope_type"`
	ScopeValue       string         `gorm:"column:scope_value;type:text" json:"scope_value"`
	NotifyChannels   string         `gorm:"column:notify_channels;type:varchar(256)" json:"notify_channels"` // alert_channels ID list, comma separated
	NotifyTemplate   string         `gorm:"column:notify_template;type:text" json:"notify_template"`
	RuleConfig       string         `gorm:"column:rule_config;type:text" json:"rule_config"`                                   // JSON: multiple alert conditions
	EnableDaysOfWeek string         `gorm:"column:enable_days_of_week;type:varchar(64);default:''" json:"enable_days_of_week"` // space separated: 0 1 2 3 4 5 6, empty means all days
	EnableStime      string         `gorm:"column:enable_stime;type:varchar(16);default:'00:00'" json:"enable_stime"`
	EnableEtime      string         `gorm:"column:enable_etime;type:varchar(16);default:'00:00'" json:"enable_etime"`
	DatasourceId     int64          `gorm:"column:datasource_id;default:0" json:"datasource_id"`
	Enabled          bool           `gorm:"column:enabled;default:true;not null;index:idx_alert_rules_enabled" json:"enabled"`
	CreatedBy        string         `gorm:"column:created_by;type:varchar(128)" json:"created_by"`
	Extension        string         `gorm:"column:extension;type:text" json:"extension"`
}

func (AlertRule) TableName() string {
	return "alert_rules"
}

// AlertEvent records each triggered alert event.
type AlertEvent struct {
	pixiu.Model

	RuleId            int64            `gorm:"column:rule_id;not null;index:idx_alert_events_rule_id" json:"rule_id"`
	RuleName          string           `gorm:"column:rule_name;type:varchar(128)" json:"rule_name"`
	Status            AlertEventStatus `gorm:"column:status;not null;index:idx_alert_events_status" json:"status"`
	Severity          AlertSeverity    `gorm:"column:severity;not null" json:"severity"`
	TriggerValue      string           `gorm:"column:trigger_value;type:varchar(256)" json:"trigger_value"`
	TriggerExpr       string           `gorm:"column:trigger_expr;type:varchar(512)" json:"trigger_expr"`
	ResourceType      string           `gorm:"column:resource_type;type:varchar(64);index:idx_alert_events_resource,priority:1" json:"resource_type"`
	ResourceName      string           `gorm:"column:resource_name;type:varchar(256);index:idx_alert_events_resource,priority:2" json:"resource_name"`
	ResourceNamespace string           `gorm:"column:resource_namespace;type:varchar(128)" json:"resource_namespace"`
	ClusterId         int64            `gorm:"column:cluster_id;index:idx_alert_events_cluster_id" json:"cluster_id"`
	TenantId          int64            `gorm:"column:tenant_id" json:"tenant_id"`
	RecoverValue      string           `gorm:"column:recover_value;type:varchar(256)" json:"recover_value"`
	RecoverTime       *time.Time       `gorm:"column:recover_time;type:datetime" json:"recover_time"`
	Labels            string           `gorm:"column:labels;type:text" json:"labels"`
	Annotations       string           `gorm:"column:annotations;type:text" json:"annotations"`
	Extension         string           `gorm:"column:extension;type:text" json:"extension"`
}

func (AlertEvent) TableName() string {
	return "alert_events"
}

// AlertNotification records each notification attempt.
type AlertNotification struct {
	pixiu.Model

	EventId    int64                   `gorm:"column:event_id;not null;index:idx_alert_notifications_event_id" json:"event_id"`
	RuleId     int64                   `gorm:"column:rule_id;not null" json:"rule_id"`
	Channel    AlertNotifyChannel      `gorm:"column:channel;not null" json:"channel"`
	Receiver   string                  `gorm:"column:receiver;type:varchar(256)" json:"receiver"`
	Title      string                  `gorm:"column:title;type:varchar(256)" json:"title"`
	Content    string                  `gorm:"column:content;type:text" json:"content"`
	Status     AlertNotificationStatus `gorm:"column:status;not null;index:idx_alert_notifications_status" json:"status"`
	RetryCount int                     `gorm:"column:retry_count;default:0" json:"retry_count"`
	ErrorMsg   string                  `gorm:"column:error_msg;type:text" json:"error_msg"`
	Extension  string                  `gorm:"column:extension;type:text" json:"extension"`
}

func (AlertNotification) TableName() string {
	return "alert_notifications"
}

// AlertChannel stores reusable notification channel configurations.
type AlertChannel struct {
	pixiu.Model

	Name        string             `gorm:"column:name;type:varchar(128);not null;index:idx_alert_channels_name" json:"name"`
	Description string             `gorm:"column:description;type:text" json:"description"`
	ChannelType AlertNotifyChannel `gorm:"column:channel_type;not null;index:idx_alert_channels_type" json:"channel_type"`
	Config      string             `gorm:"column:config;type:text" json:"config"`
	Enabled     bool               `gorm:"column:enabled;default:true;not null;index:idx_alert_channels_enabled" json:"enabled"`
	CreatedBy   string             `gorm:"column:created_by;type:varchar(128)" json:"created_by"`
	Extension   string             `gorm:"column:extension;type:text" json:"extension"`
}

func (AlertChannel) TableName() string {
	return "alert_channels"
}

// AlertSilence stores silence windows for maintenance.
type AlertSilence struct {
	pixiu.Model

	Name             string    `gorm:"column:name;type:varchar(128);not null" json:"name"`
	MatchLabels      string    `gorm:"column:match_labels;type:text" json:"match_labels"`
	MatchExpressions string    `gorm:"column:match_expressions;type:text" json:"match_expressions"`
	StartsAt         time.Time `gorm:"column:starts_at;type:datetime;not null;index:idx_alert_silences_time_range,priority:1" json:"starts_at"`
	EndsAt           time.Time `gorm:"column:ends_at;type:datetime;not null;index:idx_alert_silences_time_range,priority:2" json:"ends_at"`
	Enabled          bool      `gorm:"column:enabled;default:true;not null;index:idx_alert_silences_enabled" json:"enabled"`
	CreatedBy        string    `gorm:"column:created_by;type:varchar(128)" json:"created_by"`
	Comment          string    `gorm:"column:comment;type:text" json:"comment"`
	Extension        string    `gorm:"column:extension;type:text" json:"extension"`
}

func (AlertSilence) TableName() string {
	return "alert_silences"
}
