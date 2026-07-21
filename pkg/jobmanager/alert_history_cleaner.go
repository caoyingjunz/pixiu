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
	logutil "github.com/caoyingjunz/pixiu/pkg/util/log"
)

const (
	DefaultAlertHistorySchedule     = "0 2 * * *"
	DefaultAlertHistoryDaysReserved = 3
)

type AlertHistoryCleaner struct {
	cfg AlertHistoryOptions
	dao db.ShareDaoFactory
}

type AlertHistoryOptions struct {
	Schedule     string `yaml:"schedule"`
	DaysReserved int    `yaml:"days_reserved"`
}

func NewAlertHistoryCleaner(cfg AlertHistoryOptions, dao db.ShareDaoFactory) *AlertHistoryCleaner {
	return &AlertHistoryCleaner{cfg: cfg, dao: dao}
}

func (ac *AlertHistoryCleaner) Name() string {
	return "alert-history-cleaner"
}

func (ac *AlertHistoryCleaner) CronSpec() string {
	return ac.cfg.Schedule
}

func (ac *AlertHistoryCleaner) LogLevel() logutil.LogLevel {
	return logutil.InfoLevel
}

func (ac *AlertHistoryCleaner) Do(ctx *JobContext) (err error) {
	resv := ac.cfg.DaysReserved
	before := time.Now().AddDate(0, 0, -resv)
	entries := map[string]interface{}{
		"days_reserved": resv,
		"deadline":      before,
	}
	entries["records_deleted"], err = ac.dao.Alert().Notification().Delete(ctx, db.WithCreatedBefore(before))
	ctx.WithLogFields(entries)
	return
}

func (a *AlertHistoryOptions) Valid() error {
	return nil
}
