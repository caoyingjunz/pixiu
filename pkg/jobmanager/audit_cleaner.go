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

	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
)

const (
	CronSpec            = "0 0 * * 6" // 每周六凌晨0点清理一次
	AuditCleanLimit     = 1000
	AuditCleanKeepMonth = 1
)

type AuditsCleaner struct {
	cc  config.Config
	dao db.ShareDaoFactory
}

func NewAuditsCleaner(cfg config.Config, dao db.ShareDaoFactory) *AuditsCleaner {
	return &AuditsCleaner{
		cc:  cfg,
		dao: dao,
	}
}

func (ac *AuditsCleaner) Name() string {
	return "audits-cleaner"
}

func (ac *AuditsCleaner) CronSpec() string {
	return ac.cc.Audit.Cron
}

func (ac *AuditsCleaner) Do(ctx *JobContext) (err error) {
	timeAgo := time.Now().AddDate(0, -ac.cc.Audit.KeepMonth, 0)
	for {
		num, err := ac.dao.Audit().AuditCleanUp(ctx, ac.cc.Audit.Limit, timeAgo)
		if err != nil {
			return err
		}

		// 如果已经清理的数据小于配置的清理数量，则退出
		if num < ac.cc.Audit.Limit {
			return nil
		}

		// 为了减轻数据库的压力，可以在批次之间添加延迟
		time.Sleep(1 * time.Second)
	}
}
