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
	"fmt"
	"strings"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

func buildNotificationTitle(rule *model.AlertRule, event *model.AlertEvent, recovered bool) string {
	if recovered {
		return fmt.Sprintf("[恢复] %s", rule.Name)
	}
	return fmt.Sprintf("[告警] %s", rule.Name)
}

func buildNotificationContent(rule *model.AlertRule, event *model.AlertEvent, recovered bool) string {
	if strings.TrimSpace(rule.NotifyTemplate) != "" {
		return rule.NotifyTemplate
	}
	if recovered {
		return fmt.Sprintf("规则 %s 已恢复，当前值: %s", rule.Name, event.RecoverValue)
	}
	return fmt.Sprintf("规则 %s 触发，当前值: %s，条件: %s", rule.Name, event.TriggerValue, event.TriggerExpr)
}
