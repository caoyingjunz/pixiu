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

package engine

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

const ruleSyncInterval = 9 * time.Second

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
	if len(rules) == 0 {
		klog.V(2).Infoln("no enabled alert rules to sync")
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
		klog.V(2).Infof("alert rule(%d:%s) worker created/updated", rule.Id, rule.Name)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// desired 最新的数据
	for ruleID, worker := range desired {
		// 新增或更新规则
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

	// 移除删除的规则
	for ruleID, worker := range s.workers {
		if _, exists := desired[ruleID]; !exists {
			klog.V(2).Infof("removed alert rule worker(%d)", ruleID)
			worker.Stop()
			delete(s.workers, ruleID)
		}
	}
}

func (s *Scheduler) Manager() *Manager {
	return s.manager
}

type RuleWorker struct {
	ruleID  int64
	hash    string
	manager *Manager
	cron    *cron.Cron
}

func NewRuleWorker(rule *model.AlertRule, manager *Manager) (*RuleWorker, error) {
	ruleCopy := *rule
	worker := &RuleWorker{
		ruleID:  ruleCopy.Id,
		hash:    ruleWorkerHash(&ruleCopy),
		manager: manager,
	}

	cronSpec := EvalCronSpec(ruleCopy.EvalInterval)
	worker.cron = cron.New(cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)))
	if _, err := worker.cron.AddFunc(cronSpec, func() {
		if err := manager.EvaluateRule(context.Background(), &ruleCopy); err != nil {
			klog.Errorf("failed to evaluate alert rule(%d:%s): %v", ruleCopy.Id, ruleCopy.Name, err)
		}
	}); err != nil {
		return nil, fmt.Errorf("failed to add cron for alert rule(%d): %w", ruleCopy.Id, err)
	}
	return worker, nil
}

func (w *RuleWorker) Start() {
	w.cron.Start()
}

func (w *RuleWorker) Stop() {
	ctx := w.cron.Stop()
	<-ctx.Done()
}

func ruleWorkerHash(rule *model.AlertRule) string {
	return strconv.FormatInt(rule.Id, 10) + "_" +
		strconv.FormatInt(rule.ResourceVersion, 10) + "_" +
		strconv.Itoa(NormalizeEvalInterval(rule.EvalInterval)) + "_" +
		strconv.FormatBool(rule.Enabled) + "_" +
		strconv.Itoa(int(rule.RuleType)) + "_" +
		strconv.Itoa(rule.Duration) + "_" +
		strconv.Itoa(int(rule.ScopeType)) + "_" +
		rule.ScopeValue + "_" +
		rule.NotifyChannels + "_" +
		strconv.Itoa(NormalizeNotifyRepeatStep(rule.NotifyRepeatStep)) + "_" +
		strconv.Itoa(NormalizeNotifyMaxNumber(rule.NotifyMaxNumber)) + "_" +
		rule.RuleConfig + "_" +
		rule.EnableDaysOfWeek + "_" +
		NormalizeEnableTime(rule.EnableStime) + "_" +
		NormalizeEnableTime(rule.EnableEtime)
}
