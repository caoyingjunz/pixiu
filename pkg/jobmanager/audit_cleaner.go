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
)

const (
	DefaultSchedule      = "0 1 * * *" // 每天凌晨 1 点执行
	DefaultDaysReserved  = 7           // 保留 7 天的审计日志
	auditDeleteBatchSize = 5000        // 分批删除，避免单次 DELETE 锁表过久
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

func (ac *AuditsCleaner) LogLevel() AccessLogLevel {
	return AccessLogInfo
}

func (ac *AuditsCleaner) Do(ctx *JobContext) (err error) {
	resv := ac.cfg.DaysReserved
	before := time.Now().AddDate(0, 0, -resv)
	entries := map[string]interface{}{
		"days_reserved": resv,
		"deadline":      before,
	}
	var totalDeleted int64
	for {
		var n int64
		n, err = ac.dao.Audit().BatchDelete(ctx, db.WithCreatedBefore(before), db.WithLimit(auditDeleteBatchSize))
		if err != nil {
			return err
		}
		totalDeleted += n
		if n < auditDeleteBatchSize {
			break
		}
	}
	entries["records_deleted"] = totalDeleted
	ctx.WithLogFields(entries)
	return nil
}

func (a *AuditOptions) Valid() error {
	return nil
}
