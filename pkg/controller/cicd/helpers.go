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

package cicd

import (
	"context"
	"encoding/json"
	"time"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func isRoot(ctx context.Context) bool {
	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return false
	}
	return user.Role == model.RoleRoot
}

func currentUserID(ctx context.Context) (int64, error) {
	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return 0, err
	}
	return user.Id, nil
}

func (c *controller) now() *time.Time {
	now := time.Now()
	return &now
}

// ---- config 默认值访问（设计第 11 节）----

func (c *controller) maxRunningPerPipeline() int64 {
	v := c.cc.CICD.MaxRunningPerPipeline
	if v <= 0 {
		return 1
	}
	return int64(v)
}

func (c *controller) maxPendingRuns() int64 {
	return int64(c.cc.CICD.MaxPendingRuns)
}

func (c *controller) workerCount() int {
	v := c.cc.CICD.Workers
	if v <= 0 {
		return 2
	}
	return v
}

func (c *controller) pollInterval() time.Duration {
	d, err := time.ParseDuration(c.cc.CICD.PollInterval)
	if err != nil || d <= 0 {
		return 3 * time.Second
	}
	return d
}

func (c *controller) logRetention() time.Duration {
	days := c.cc.CICD.LogRetentionDays
	if days <= 0 {
		days = 14
	}
	return time.Duration(days) * 24 * time.Hour
}

// jsonUnmarshal 忽略反序列化错误的轻量包装。
func jsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// pipelineConfig 将存储的 config JSON 解析为强类型配置。
func pipelineConfig(raw string) types.PipelineConfig {
	var cfg types.PipelineConfig
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &cfg)
	}
	return cfg
}
