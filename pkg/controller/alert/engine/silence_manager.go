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

package engine

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

type SilenceManager struct {
	factory db.ShareDaoFactory
}

func NewSilenceManager(factory db.ShareDaoFactory) *SilenceManager {
	return &SilenceManager{factory: factory}
}

func (s *SilenceManager) LoadActive(ctx context.Context, now time.Time) ([]model.AlertSilence, error) {
	return s.factory.Alert().Silence().ListActive(ctx, now)
}

func (s *SilenceManager) IsSilenced(silences []model.AlertSilence, rule *model.AlertRule, sample MetricSample) bool {
	labels := map[string]string{
		"rule_id":       strconv.FormatInt(rule.Id, 10),
		"rule_name":     rule.Name,
		"resource_type": sample.ResourceType,
		"resource_name": sample.ResourceName,
		"namespace":     sample.Namespace,
	}
	if promQl := GetRulePromQl(rule); promQl != "" {
		labels["prom_ql"] = promQl
	}
	for key, value := range sample.Labels {
		labels[key] = value
	}

	for i := range silences {
		if matchSilence(&silences[i], labels) {
			return true
		}
	}
	return false
}

func matchSilence(silence *model.AlertSilence, labels map[string]string) bool {
	if !silence.Enabled {
		return false
	}
	now := time.Now()
	if now.Before(silence.StartsAt) || now.After(silence.EndsAt) {
		return false
	}

	matchLabels := map[string]string{}
	if strings.TrimSpace(silence.MatchLabels) != "" {
		_ = json.Unmarshal([]byte(silence.MatchLabels), &matchLabels)
	}
	for key, expected := range matchLabels {
		if labels[key] != expected {
			return false
		}
	}
	return len(matchLabels) > 0 || strings.TrimSpace(silence.MatchExpressions) != ""
}
