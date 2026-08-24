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
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/accesslog"
)

type Job interface {
	// Name returns the job name
	Name() string

	// CronSpec returns the cron expression of the job
	// e.g. "* * * * *"
	CronSpec() string

	// LogLevel returns the access log level of the job
	LogLevel() AccessLogLevel

	// Do is the job handler
	Do(ctx *JobContext) error
}

type Manager struct {
	cron *cron.Cron
}

// tracedJob 包装单个 job，提供每 job 独立的 SkipIfStillRunning 语义：
// 上一次执行未结束时跳过本次节拍，并以 Warning 显式记录。
// 注意：不能使用 cron.SkipIfStillRunning + cron.WithChain 的组合——
// v3.0.0 的 SkipIfStillRunning 在 wrapper 外层创建令牌 channel，
// 经 WithChain 应用后所有 job 共享同一令牌，任一 job 执行期间
// 其他全部 job 的节拍都会被误跳过（定时扩缩容丢触发的根因）。
func tracedJob(name string, lc *accesslog.Options, job Job) func() {
	var mu sync.Mutex
	return func() {
		if !mu.TryLock() {
			klog.Warningf("[JobTrace] job %s 上一次执行尚未结束，本次节拍被跳过", name)
			return
		}
		defer mu.Unlock()
		start := time.Now()
		ctx := NewJobContext(name, lc)
		ctx.Log(job.LogLevel(), job.Do(ctx))
		if cost := time.Since(start); cost > 5*time.Second {
			klog.Warningf("[JobTrace] job %s 执行耗时 %s，后续节拍可能被跳过", name, cost)
		}
	}
}

func NewManager(lc *accesslog.Options, jobs ...Job) *Manager {
	logger := accesslog.CronLogger{}
	c := cron.New(
		cron.WithLogger(logger),
	)
	for _, job := range jobs {
		_, _ = c.AddFunc(job.CronSpec(), tracedJob(job.Name(), lc, job))
	}
	return &Manager{c}
}

func (m *Manager) Run() {
	m.cron.Start()
}

func (m *Manager) Stop() {
	ctx := m.cron.Stop()
	<-ctx.Done()
}
