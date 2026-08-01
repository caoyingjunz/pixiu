/*
Copyright 2024 The Pixiu Authors.

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

package accesslog

import (
	"github.com/robfig/cron/v3"
	"k8s.io/klog/v2"
)

// CronLogger 将 robfig/cron 日志桥接到 klog。
type CronLogger struct{}

var _ cron.Logger = CronLogger{}

func (CronLogger) Info(msg string, keysAndValues ...interface{}) {
	klog.V(4).InfoS(msg, keysAndValues...)
}

func (CronLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	klog.ErrorS(err, msg, keysAndValues...)
}
