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

import "fmt"

const (
	DefaultEvalInterval     = 15
	DefaultNotifyRepeatStep = 5 // minutes
)

func NormalizeEvalInterval(interval int) int {
	if interval <= 0 {
		return DefaultEvalInterval
	}
	return interval
}

// NormalizeNotifyRepeatStep keeps 0 as "disable repeat"; negative values fall back to default 5.
func NormalizeNotifyRepeatStep(step int) int {
	if step < 0 {
		return DefaultNotifyRepeatStep
	}
	return step
}

// ResolveNotifyRepeatStep returns default 5 when step is nil (create omitted).
func ResolveNotifyRepeatStep(step *int) int {
	if step == nil {
		return DefaultNotifyRepeatStep
	}
	return NormalizeNotifyRepeatStep(*step)
}

func NormalizeNotifyMaxNumber(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

func ResolveNotifyMaxNumber(n *int) int {
	if n == nil {
		return 0
	}
	return NormalizeNotifyMaxNumber(*n)
}

func EvalCronSpec(interval int) string {
	return fmt.Sprintf("@every %ds", NormalizeEvalInterval(interval))
}
