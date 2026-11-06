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

package upgrade

import (
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/jobmanager"
)

// Job 自动升级守护 job：实现 jobmanager.Job，按 cron 周期执行一轮升级检查。
// 仅在 upgrade.enabled 为 true 时注册（见 options.Complete）。
type Job struct {
	up  *Upgrader
	cfg config.UpgradeOptions
}

func NewJob(cfg config.UpgradeOptions, rtOpts config.RuntimeOptions) *Job {
	return &Job{
		up:  New(cfg, rtOpts),
		cfg: cfg,
	}
}

func (j *Job) Name() string {
	return "pixiu-upgrader"
}

func (j *Job) CronSpec() string {
	return j.cfg.Schedule
}

func (j *Job) LogLevel() jobmanager.AccessLogLevel {
	return jobmanager.AccessLogInfo
}

func (j *Job) Do(ctx *jobmanager.JobContext) error {
	return j.up.Upgrade(ctx)
}
