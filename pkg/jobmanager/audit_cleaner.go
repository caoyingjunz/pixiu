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

package jobmanager

import (
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db"
	logutil "github.com/caoyingjunz/pixiu/pkg/util/log"
)

const (
	DefaultSchedule     = "0 0 * * 6" // 每周六 0 点执行
	DefaultDaysReserved = 30          // 保留 30 天的审计日志
)

type AuditsCleaner struct {
	cfg AuditOptions
	dao db.ShareDaoFactory
}

type AuditOptions struct {
	Schedule     string `yaml:"schedule"`
	DaysReserved int    `yaml:"days_reserved"`
}

func DefaultOptions() AuditOptions {
	return AuditOptions{
		Schedule:     DefaultSchedule,
		DaysReserved: DefaultDaysReserved,
	}
}

func NewAuditsCleaner(cfg AuditOptions, dao db.ShareDaoFactory) *AuditsCleaner {
	return &AuditsCleaner{
		cfg: cfg,
		dao: dao,
	}
}

func (ac *AuditsCleaner) Name() string {
	return "audits-cleaner"
}

func (ac *AuditsCleaner) CronSpec() string {
	return ac.cfg.Schedule
}

func (ac *AuditsCleaner) LogLevel() logutil.LogLevel {
	return logutil.InfoLevel
}

func (ac *AuditsCleaner) Do(ctx *JobContext) (err error) {
	resv := ac.cfg.DaysReserved
	before := time.Now().AddDate(0, 0, -resv)
	entries := map[string]interface{}{
		"days_reserved": resv,
		"deadline":      before,
	}
	entries["records_deleted"], err = ac.dao.Audit().BatchDelete(ctx, db.WithCreatedBefore(before))
	ctx.WithLogFields(entries)

	return
}

func (a *AuditOptions) Valid() error {
	// TODO
	return nil
}
