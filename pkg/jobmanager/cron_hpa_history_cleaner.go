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
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db"
)

const (
	DefaultCronHpaHistorySchedule     = "0 3 * * *"
	DefaultCronHpaHistoryDaysReserved = 30
)

type CronHpaHistoryCleaner struct {
	cfg CronHpaHistoryOptions
	dao db.ShareDaoFactory
}

type CronHpaHistoryOptions struct {
	Schedule     string `yaml:"schedule"`
	DaysReserved int    `yaml:"days_reserved"`
}

func NewCronHpaHistoryCleaner(cfg CronHpaHistoryOptions, dao db.ShareDaoFactory) *CronHpaHistoryCleaner {
	return &CronHpaHistoryCleaner{cfg: cfg, dao: dao}
}

func (cc *CronHpaHistoryCleaner) Name() string {
	return "cron-hpa-history-cleaner"
}

func (cc *CronHpaHistoryCleaner) CronSpec() string {
	return cc.cfg.Schedule
}

func (cc *CronHpaHistoryCleaner) LogLevel() AccessLogLevel {
	return AccessLogInfo
}

func (cc *CronHpaHistoryCleaner) Do(ctx *JobContext) (err error) {
	resv := cc.cfg.DaysReserved
	before := time.Now().AddDate(0, 0, -resv)
	err = cc.dao.CronHpa().CleanHistories(ctx, before)
	ctx.WithLogFields(map[string]interface{}{
		"days_reserved": resv,
		"deadline":      before,
	})
	return
}

func (c *CronHpaHistoryOptions) Valid() error {
	return nil
}
