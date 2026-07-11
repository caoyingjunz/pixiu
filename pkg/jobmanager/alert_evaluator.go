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

	"github.com/caoyingjunz/pixiu/pkg/alert"
	"github.com/caoyingjunz/pixiu/pkg/db"
	logutil "github.com/caoyingjunz/pixiu/pkg/util/log"
)

const DefaultAlertEvaluateInterval = "@every 1m"

type AlertEvaluator struct {
	factory db.ShareDaoFactory
	manager *alert.Manager
}

func NewAlertEvaluator(f db.ShareDaoFactory) *AlertEvaluator {
	return &AlertEvaluator{
		factory: f,
		manager: alert.NewManager(f, &alert.StaticMetricProvider{}),
	}
}

func (a *AlertEvaluator) Name() string {
	return "alert-evaluator"
}

func (a *AlertEvaluator) CronSpec() string {
	return DefaultAlertEvaluateInterval
}

func (a *AlertEvaluator) LogLevel() logutil.LogLevel {
	return logutil.InfoLevel
}

func (a *AlertEvaluator) Do(ctx *JobContext) error {
	return a.manager.Run(context.Background())
}
