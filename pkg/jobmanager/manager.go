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
	"github.com/robfig/cron/v3"

	logutil "github.com/caoyingjunz/pixiu/pkg/util/log"
)

type Job interface {
	// Name returns the job name
	Name() string

	// CronSpec returns the cron expression of the job
	// e.g. "* * * * *"
	CronSpec() string

	// LogLevel returns the log level of the job
	LogLevel() logutil.LogLevel

	// Do is the job handler
	Do(ctx *JobContext) error
}

type Manager struct {
	cron *cron.Cron
}

func NewManager(lc *logutil.LogOptions, jobs ...Job) *Manager {
	c := cron.New()
	for _, job := range jobs {
		c.AddFunc(job.CronSpec(), func() {
			ctx := NewJobContext(job.Name(), lc)
			ctx.Log(job.LogLevel(), job.Do(ctx))
		})
	}
	return &Manager{
		c,
	}
}

func (m *Manager) Run() {
	m.cron.Start()
}

func (m *Manager) Stop() {
	ctx := m.cron.Stop()
	<-ctx.Done()
}
