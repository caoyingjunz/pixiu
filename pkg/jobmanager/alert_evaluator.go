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

	"github.com/caoyingjunz/pixiu/pkg/controller/alert/engine"
	"github.com/caoyingjunz/pixiu/pkg/db"
	logutil "github.com/caoyingjunz/pixiu/pkg/util/log"
)

const DefaultAlertNotifyDispatchInterval = "@every 15s"

type AlertEvaluator struct {
	scheduler *engine.Scheduler
}

func NewAlertEvaluator(f db.ShareDaoFactory) *AlertEvaluator {
	scheduler := engine.NewScheduler(f, &engine.StaticMetricProvider{})
	scheduler.Start(context.Background())
	return &AlertEvaluator{scheduler: scheduler}
}

func (a *AlertEvaluator) Name() string {
	return "alert-evaluator"
}

func (a *AlertEvaluator) CronSpec() string {
	return DefaultAlertNotifyDispatchInterval
}

func (a *AlertEvaluator) LogLevel() logutil.LogLevel {
	return logutil.InfoLevel
}

func (a *AlertEvaluator) Do(ctx *JobContext) error {
	return a.scheduler.Manager().DispatchPending(context.Background())
}

func (a *AlertEvaluator) Stop() {
	if a.scheduler != nil {
		a.scheduler.Stop()
	}
}
