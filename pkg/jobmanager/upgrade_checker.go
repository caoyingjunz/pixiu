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
	"fmt"
	"time"
)

// upgradeCheckTimeout 单次版本检查上限，与启动一次性检查一致。
const upgradeCheckTimeout = 60 * time.Second

// UpgradeChecker 将自动升级的版本检查接入 JobManager 周期调度。
type UpgradeChecker struct {
	checker interface {
		ProbeVersion(ctx context.Context) error
	}
	interval time.Duration
}

func NewUpgradeChecker(checker interface {
	ProbeVersion(ctx context.Context) error
}, interval time.Duration) *UpgradeChecker {
	return &UpgradeChecker{
		checker:  checker,
		interval: interval,
	}
}

func (c *UpgradeChecker) Name() string {
	return "upgrade-checker"
}

func (c *UpgradeChecker) CronSpec() string {
	return fmt.Sprintf("@every %s", c.interval)
}

func (c *UpgradeChecker) LogLevel() AccessLogLevel {
	return AccessLogInfo
}

func (c *UpgradeChecker) Do(ctx *JobContext) error {
	checkCtx, cancel := context.WithTimeout(ctx, upgradeCheckTimeout)
	defer cancel()
	return c.checker.ProbeVersion(checkCtx)
}
