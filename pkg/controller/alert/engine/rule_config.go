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
	"encoding/json"
	"strconv"
	"strings"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

// AlertRuleConfig stores one PromQL query and multiple trigger conditions.
type AlertRuleConfig struct {
	PromQl   string         `json:"prom_ql"`
	Triggers []AlertTrigger `json:"triggers"`

	// Queries is legacy multi-promql format kept for migration only.
	Queries []legacyAlertQuery `json:"queries,omitempty"`
}

// AlertTrigger is a severity / threshold / duration condition for the shared PromQL.
type AlertTrigger struct {
	Severity  model.AlertSeverity `json:"severity"`
	Condition string              `json:"condition"`
	Duration  int                 `json:"duration"`
}

type legacyAlertQuery struct {
	PromQl   string              `json:"prom_ql"`
	Severity model.AlertSeverity `json:"severity"`
}

// EvaluableTrigger is the expanded form used by the alert engine.
type EvaluableTrigger struct {
	PromQl    string
	Severity  model.AlertSeverity
	Condition string
	Duration  int
}

func ParseAlertRuleConfig(raw string) AlertRuleConfig {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return AlertRuleConfig{}
	}
	var cfg AlertRuleConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return AlertRuleConfig{}
	}
	return cfg
}

func EncodeAlertRuleConfig(cfg AlertRuleConfig) string {
	promQl := strings.TrimSpace(cfg.PromQl)
	if promQl == "" || len(cfg.Triggers) == 0 {
		return ""
	}
	out := AlertRuleConfig{
		PromQl:   promQl,
		Triggers: cfg.Triggers,
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return ""
	}
	return string(raw)
}

func normalizeSeverity(sev, fallback model.AlertSeverity) model.AlertSeverity {
	if sev >= model.AlertSeverityInfo && sev <= model.AlertSeverityEmergency {
		return sev
	}
	if fallback >= model.AlertSeverityInfo && fallback <= model.AlertSeverityEmergency {
		return fallback
	}
	return model.AlertSeverityWarning
}

func normalizeDuration(d int) int {
	if d < 0 {
		return 0
	}
	return d
}

func looksLikeThresholdCondition(expr string) bool {
	_, _, ok := splitThresholdExpr(strings.TrimSpace(expr))
	return ok
}

// isValidThresholdCondition matches matchThreshold: operator + numeric value.
func isValidThresholdCondition(expr string) bool {
	op, expected, ok := splitThresholdExpr(strings.TrimSpace(expr))
	if !ok || op == "" {
		return false
	}
	_, err := strconv.ParseFloat(strings.TrimSpace(expected), 64)
	return err == nil
}

func migrateLegacyQueries(cfg AlertRuleConfig) AlertRuleConfig {
	if strings.TrimSpace(cfg.PromQl) != "" && len(cfg.Triggers) > 0 {
		return cfg
	}
	if len(cfg.Queries) == 0 {
		return cfg
	}

	promQl := strings.TrimSpace(cfg.PromQl)
	triggers := make([]AlertTrigger, 0, len(cfg.Queries))
	for _, q := range cfg.Queries {
		raw := strings.TrimSpace(q.PromQl)
		if raw == "" {
			continue
		}
		if looksLikeThresholdCondition(raw) {
			triggers = append(triggers, AlertTrigger{
				Severity:  normalizeSeverity(q.Severity, model.AlertSeverityWarning),
				Condition: raw,
				Duration:  0,
			})
			continue
		}
		if promQl == "" {
			promQl = raw
		}
		triggers = append(triggers, AlertTrigger{
			Severity:  normalizeSeverity(q.Severity, model.AlertSeverityWarning),
			Condition: "> 0",
			Duration:  0,
		})
	}
	return AlertRuleConfig{PromQl: promQl, Triggers: triggers}
}

// NormalizeRuleConfig normalizes rule_config and returns severity / duration of the first trigger.
func NormalizeRuleConfig(ruleConfig string) (normalizedConfig string, normalizedSeverity model.AlertSeverity, normalizedDuration int, ok bool) {
	cfg := migrateLegacyQueries(ParseAlertRuleConfig(ruleConfig))
	promQl := strings.TrimSpace(cfg.PromQl)
	if promQl == "" {
		return "", model.AlertSeverityWarning, 0, false
	}

	triggers := make([]AlertTrigger, 0, len(cfg.Triggers))
	for _, t := range cfg.Triggers {
		condition := strings.TrimSpace(t.Condition)
		if condition == "" {
			continue
		}
		if !isValidThresholdCondition(condition) {
			return "", model.AlertSeverityWarning, 0, false
		}
		triggers = append(triggers, AlertTrigger{
			Severity:  normalizeSeverity(t.Severity, model.AlertSeverityWarning),
			Condition: condition,
			Duration:  normalizeDuration(t.Duration),
		})
	}
	if len(triggers) == 0 {
		return "", model.AlertSeverityWarning, 0, false
	}

	cfg = AlertRuleConfig{PromQl: promQl, Triggers: triggers}
	return EncodeAlertRuleConfig(cfg), triggers[0].Severity, triggers[0].Duration, true
}

func GetRulePromQl(rule *model.AlertRule) string {
	if rule == nil {
		return ""
	}
	cfg := migrateLegacyQueries(ParseAlertRuleConfig(rule.RuleConfig))
	return strings.TrimSpace(cfg.PromQl)
}

// ListAlertTriggers returns configured triggers for evaluation.
func ListAlertTriggers(rule *model.AlertRule) []EvaluableTrigger {
	if rule == nil {
		return nil
	}
	cfg := migrateLegacyQueries(ParseAlertRuleConfig(rule.RuleConfig))
	promQl := strings.TrimSpace(cfg.PromQl)
	if promQl == "" {
		return nil
	}

	out := make([]EvaluableTrigger, 0, len(cfg.Triggers))
	for _, t := range cfg.Triggers {
		condition := strings.TrimSpace(t.Condition)
		if condition == "" {
			continue
		}
		out = append(out, EvaluableTrigger{
			PromQl:    promQl,
			Severity:  normalizeSeverity(t.Severity, model.AlertSeverityWarning),
			Condition: condition,
			Duration:  normalizeDuration(t.Duration),
		})
	}
	return out
}

// ListAlertQueries is kept as a thin wrapper for call sites that need PromQL + severity.
func ListAlertQueries(rule *model.AlertRule) []EvaluableTrigger {
	return ListAlertTriggers(rule)
}
