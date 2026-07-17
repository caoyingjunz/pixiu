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

package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"text/template"
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

const notificationTimeLayout = "2006-01-02 15:04:05"

// Matches $name or ${name}. Name follows common Prometheus label naming.
var labelPlaceholderPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

// NotificationTemplateData is the context for rule.NotifyTemplate (text/template).
type NotificationTemplateData struct {
	RuleId       int64
	RuleName     string
	Severity     int
	IsRecovered  bool
	TriggerValue string
	TriggerExpr  string
	RecoverValue string
	ResourceType string
	ResourceName string
	Namespace    string
	ClusterId    int64
	TenantId     int64
	Labels       map[string]string
	Annotations  string
	FireTime     string // FireTime is when the alert first fired (event.gmt_create), formatted locally.
	RecoverTime  string // RecoverTime is when the alert recovered, if any.
}

func buildNotificationTitle(rule *model.AlertRule, event *model.AlertEvent, recovered bool) string {
	if rule == nil {
		if recovered {
			return "[恢复] 未知规则"
		}
		return "[告警] 未知规则"
	}
	if recovered {
		return fmt.Sprintf("[恢复] %s", rule.Name)
	}
	return fmt.Sprintf("[告警] %s", rule.Name)
}

// 构造真实通知内容
func buildNotificationContent(rule *model.AlertRule, event *model.AlertEvent, recovered bool) string {
	if rule != nil && strings.TrimSpace(rule.NotifyTemplate) != "" {
		if rendered, err := renderNotifyTemplate(rule.NotifyTemplate, rule, event, recovered); err == nil {
			return rendered
		} else {
			klog.Errorf("failed to render notify template for rule(%d): %v, fallback to default content", rule.Id, err)
		}
	}

	klog.V(0).Infof("未发现指定模板，使用默认通知内容")
	return defaultNotificationContent(rule, event, recovered)
}

func defaultNotificationContent(rule *model.AlertRule, event *model.AlertEvent, recovered bool) string {
	ruleName := "未知规则"
	if rule != nil {
		ruleName = rule.Name
	}
	triggerValue, triggerExpr, recoverValue := "", "", ""
	if event != nil {
		triggerValue = event.TriggerValue
		triggerExpr = event.TriggerExpr
		recoverValue = event.RecoverValue
	}
	if recovered {
		return fmt.Sprintf("规则 %s 已恢复，当前值: %s", ruleName, recoverValue)
	}
	return fmt.Sprintf("规则 %s 触发，当前值: %s，条件: %s", ruleName, triggerValue, triggerExpr)
}

func renderNotifyTemplate(tplText string, rule *model.AlertRule, event *model.AlertEvent, recovered bool) (string, error) {
	tpl, err := template.New("alert-notify").Option("missingkey=zero").Parse(tplText)
	if err != nil {
		return "", err
	}

	data := buildNotificationTemplateData(rule, event, recovered)
	var buf bytes.Buffer
	if err = tpl.Execute(&buf, data); err != nil {
		if rule != nil {
			klog.Errorf("failed to render notify template for rule(%d): %v", rule.Id, err)
		}
		return "", err
	}

	return expandLabelPlaceholders(buf.String(), data.Labels), nil
}

func buildNotificationTemplateData(rule *model.AlertRule, event *model.AlertEvent, recovered bool) NotificationTemplateData {
	data := NotificationTemplateData{IsRecovered: recovered, Labels: map[string]string{}}
	if rule != nil {
		data.RuleId = rule.Id
		data.RuleName = rule.Name
		data.Severity = int(rule.Severity)
	}
	if event != nil {
		if data.RuleId == 0 {
			data.RuleId = event.RuleId
		}
		if data.RuleName == "" {
			data.RuleName = event.RuleName
		}
		if data.Severity == 0 {
			data.Severity = int(event.Severity)
		}
		data.TriggerValue = event.TriggerValue
		data.TriggerExpr = event.TriggerExpr
		data.RecoverValue = event.RecoverValue
		data.ResourceType = event.ResourceType
		data.ResourceName = event.ResourceName
		data.Namespace = event.ResourceNamespace
		data.ClusterId = event.ClusterId
		data.TenantId = event.TenantId
		data.Labels = parseLabelMap(event.Labels)
		data.Annotations = event.Annotations
		data.FireTime = formatNotificationTime(event.GmtCreate)
		if event.RecoverTime != nil {
			data.RecoverTime = formatNotificationTime(*event.RecoverTime)
		}
	}
	if data.FireTime == "" {
		data.FireTime = formatNotificationTime(time.Now())
	}
	return data
}

func formatNotificationTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format(notificationTimeLayout)
}

func parseLabelMap(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]string{}
	}
	out := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		klog.V(2).Infof("failed to parse alert event labels json: %v", err)
		return map[string]string{}
	}
	return out
}

// expandLabelPlaceholders replaces $name / ${name} with Labels values; missing keys become empty.
func expandLabelPlaceholders(text string, labels map[string]string) string {
	if text == "" || !strings.Contains(text, "$") {
		return text
	}
	if labels == nil {
		labels = map[string]string{}
	}
	return labelPlaceholderPattern.ReplaceAllStringFunc(text, func(match string) string {
		sub := labelPlaceholderPattern.FindStringSubmatch(match)
		if len(sub) < 3 {
			return ""
		}
		key := sub[1]
		if key == "" {
			key = sub[2]
		}
		return labels[key]
	})
}
