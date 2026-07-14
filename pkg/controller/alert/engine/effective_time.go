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
	"strconv"
	"strings"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

const defaultEnableTime = "00:00"

func NormalizeEnableTime(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultEnableTime
	}
	return value
}

func NormalizeEnableDaysOfWeek(value string) string {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 {
		return ""
	}
	normalized := make([]string, 0, len(fields))
	seen := map[string]struct{}{}
	for _, field := range fields {
		day := strings.ReplaceAll(field, "7", "0")
		if _, err := strconv.Atoi(day); err != nil {
			continue
		}
		if _, ok := seen[day]; ok {
			continue
		}
		seen[day] = struct{}{}
		normalized = append(normalized, day)
	}
	return strings.Join(normalized, " ")
}

// IsWithinEffectiveTime reports whether the rule is active at the given time.
// Empty enable_days_of_week means all-day/all-week.
func IsWithinEffectiveTime(rule *model.AlertRule, now time.Time) bool {
	if rule == nil {
		return false
	}

	days := strings.Fields(NormalizeEnableDaysOfWeek(rule.EnableDaysOfWeek))
	stime := NormalizeEnableTime(rule.EnableStime)
	etime := NormalizeEnableTime(rule.EnableEtime)

	if len(days) == 0 {
		return true
	}

	triggerWeek := strconv.Itoa(int(now.Weekday()))
	matchedDay := false
	for _, day := range days {
		if day == triggerWeek {
			matchedDay = true
			break
		}
	}
	if !matchedDay {
		return false
	}

	// Same start/end means all day within the selected weekdays.
	if stime == etime {
		return true
	}

	triggerTime := now.Format("15:04")
	if stime < etime {
		if etime == "23:59" {
			return triggerTime >= stime
		}
		return triggerTime >= stime && triggerTime < etime
	}

	// Cross midnight, e.g. 21:00 - 09:00
	return triggerTime >= stime || triggerTime < etime
}
