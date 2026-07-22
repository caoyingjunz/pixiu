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

package labelmap

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

func Encode(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(raw)
}

func Decode(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]string{}
	}
	out := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return map[string]string{}
	}
	return out
}

// Merge merges label maps with later maps taking precedence on key conflicts.
// Priority example (notification > event > rule): Merge(rule, event, notification).
func Merge(parts ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, part := range parts {
		for k, v := range part {
			if strings.TrimSpace(k) == "" {
				continue
			}
			out[k] = v
		}
	}
	return out
}

// NormalizeJSON validates and canonicalizes rule labels JSON object.
func NormalizeJSON(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	values := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return "", fmt.Errorf("labels must be a JSON object of string key-values: %w", err)
	}
	return Encode(values), nil
}

// BuildEvent merges rule labels with sample labels; sample/event wins on conflicts.
func BuildEvent(rule *model.AlertRule, sampleLabels map[string]string) string {
	ruleLabels := map[string]string{}
	if rule != nil {
		ruleLabels = Decode(rule.Labels)
	}
	return Encode(Merge(ruleLabels, sampleLabels))
}

// BuildNotification merges rule/event/notification labels with priority notify > event > rule.
func BuildNotification(rule *model.AlertRule, event *model.AlertEvent, notificationLabels map[string]string) string {
	ruleLabels := map[string]string{}
	if rule != nil {
		ruleLabels = Decode(rule.Labels)
	}
	eventLabels := map[string]string{}
	if event != nil {
		eventLabels = Decode(event.Labels)
	}
	return Encode(Merge(ruleLabels, eventLabels, notificationLabels))
}
