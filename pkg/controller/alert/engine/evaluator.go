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
	"fmt"
	"strconv"
	"strings"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

type Evaluator struct{}

func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

func (e *Evaluator) Match(rule *model.AlertRule, value string) bool {
	expr := strings.TrimSpace(rule.ConditionExpr)
	if expr == "" {
		return false
	}

	switch rule.RuleType {
	case model.AlertRuleTypeThreshold:
		return matchThreshold(value, expr)
	case model.AlertRuleTypeStatus, model.AlertRuleTypeEvent:
		return matchTextCondition(value, expr)
	default:
		return false
	}
}

func matchThreshold(value, expr string) bool {
	trimmed := strings.TrimSpace(expr)
	operator, expectedRaw, ok := splitThresholdExpr(trimmed)
	if !ok {
		return false
	}

	actual, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return false
	}
	expected, err := strconv.ParseFloat(strings.TrimSpace(expectedRaw), 64)
	if err != nil {
		return false
	}

	switch operator {
	case ">":
		return actual > expected
	case ">=":
		return actual >= expected
	case "<":
		return actual < expected
	case "<=":
		return actual <= expected
	case "=", "==":
		return actual == expected
	case "!=", "<>":
		return actual != expected
	default:
		return false
	}
}

func splitThresholdExpr(expr string) (operator, expected string, ok bool) {
	operators := []string{">=", "<=", "!=", "<>", "==", ">", "<", "="}
	for _, op := range operators {
		if strings.HasPrefix(expr, op) {
			return op, strings.TrimSpace(strings.TrimPrefix(expr, op)), true
		}
	}
	return "", "", false
}

func matchTextCondition(value, expr string) bool {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return false
	}

	operator, expected, ok := splitThresholdExpr(trimmed)
	if !ok {
		return strings.EqualFold(strings.TrimSpace(value), strings.Trim(trimmed, `"'`))
	}

	actual := strings.TrimSpace(value)
	expected = strings.Trim(expected, `"'`)
	switch operator {
	case "=", "==":
		return actual == expected
	case "!=", "<>":
		return actual != expected
	default:
		return false
	}
}

func formatTriggerExpr(rule *model.AlertRule) string {
	if strings.TrimSpace(rule.ConditionExpr) == "" {
		return ""
	}
	return fmt.Sprintf("%s %s", rule.MetricName, strings.TrimSpace(rule.ConditionExpr))
}
