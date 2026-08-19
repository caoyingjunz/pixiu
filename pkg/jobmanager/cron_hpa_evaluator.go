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

package jobmanager

import (
	"context"
)

const (
	// DefaultCronHpaSchedule 定时扩缩容评估周期，到期任务最大延迟一个周期
	DefaultCronHpaSchedule = "@every 30s"
)

// CronHpaEvaluator 定时扩缩容评估器：周期扫描启用中的规则，触发到期任务执行。
// 以窄接口持有 autoscaling 能力，避免 jobmanager 与 cmd/app/config 的循环依赖。
type CronHpaEvaluator struct {
	autoscaling interface {
		EvaluateOnce(ctx context.Context) error
	}
}

func NewCronHpaEvaluator(as interface{ EvaluateOnce(ctx context.Context) error }) *CronHpaEvaluator {
	return &CronHpaEvaluator{autoscaling: as}
}

func (e *CronHpaEvaluator) Name() string {
	return "cron-hpa-evaluator"
}

func (e *CronHpaEvaluator) CronSpec() string {
	return DefaultCronHpaSchedule
}

func (e *CronHpaEvaluator) LogLevel() AccessLogLevel {
	return AccessLogDebug
}

func (e *CronHpaEvaluator) Do(ctx *JobContext) error {
	return e.autoscaling.EvaluateOnce(ctx)
}
