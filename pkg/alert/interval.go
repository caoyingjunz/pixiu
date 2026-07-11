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

package alert

import "fmt"

const (
	// DefaultEvalInterval is the default per-rule evaluation interval in seconds.
	DefaultEvalInterval = 15
)

// NormalizeEvalInterval returns a valid evaluation interval in seconds.
func NormalizeEvalInterval(interval int) int {
	if interval <= 0 {
		return DefaultEvalInterval
	}
	return interval
}

// EvalCronSpec builds a cron expression for the given evaluation interval.
func EvalCronSpec(interval int) string {
	return fmt.Sprintf("@every %ds", NormalizeEvalInterval(interval))
}
