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

package alert

import (
	"context"
	"sync"
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db"
)

const ruleSyncInterval = 9 * time.Second

// Scheduler manages per-rule evaluation workers, similar to Nightingale's alert engine.
type Scheduler struct {
	factory db.ShareDaoFactory
	manager *Manager

	mu      sync.Mutex
	workers map[int64]*RuleWorker

	cancel context.CancelFunc
}

func NewScheduler(factory db.ShareDaoFactory, provider MetricProvider) *Scheduler {
	return &Scheduler{
		factory: factory,
		manager: NewManager(factory, provider),
		workers: make(map[int64]*RuleWorker),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)
	go s.loopSyncRules(ctx)
}

func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for ruleID, worker := range s.workers {
		worker.Stop()
		delete(s.workers, ruleID)
	}
}

func (s *Scheduler) loopSyncRules(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(ruleSyncInterval):
			s.syncRules(ctx)
			if err := s.manager.DispatchPending(ctx); err != nil {
				klog.Errorf("failed to dispatch pending alert notifications: %v", err)
			}
		}
	}
}

func (s *Scheduler) syncRules(ctx context.Context) {
	rules, err := s.factory.Alert().Rule().List(ctx, db.WithEnabled(true))
	if err != nil {
		klog.Errorf("failed to list enabled alert rules: %v", err)
		return
	}

	desired := make(map[int64]*RuleWorker, len(rules))
	for i := range rules {
		rule := rules[i]
		hash := ruleWorkerHash(&rule)

		s.mu.Lock()
		current, exists := s.workers[rule.Id]
		s.mu.Unlock()
		if exists && current.hash == hash {
			desired[rule.Id] = current
			continue
		}

		worker, err := NewRuleWorker(&rule, s.manager)
		if err != nil {
			klog.Errorf("failed to create alert rule worker(%d:%s): %v", rule.Id, rule.Name, err)
			continue
		}
		desired[rule.Id] = worker
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for ruleID, worker := range desired {
		current, exists := s.workers[ruleID]
		if exists && current == worker {
			continue
		}
		if exists {
			current.Stop()
		}
		worker.Start()
		s.workers[ruleID] = worker
	}

	for ruleID, worker := range s.workers {
		if _, exists := desired[ruleID]; !exists {
			worker.Stop()
			delete(s.workers, ruleID)
		}
	}
}

func (s *Scheduler) Manager() *Manager {
	return s.manager
}
