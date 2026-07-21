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
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/controller/alert/notify"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

type Manager struct {
	factory  db.ShareDaoFactory
	provider MetricProvider
	eval     *Evaluator
	trigger  *Trigger
	silence  *SilenceManager
	notify   *notify.Manager
}

func NewManager(factory db.ShareDaoFactory, provider MetricProvider) *Manager {
	notifyManager := notify.NewManager(factory)
	return &Manager{
		factory:  factory,
		provider: provider,
		eval:     NewEvaluator(),
		trigger:  NewTrigger(factory, notifyManager),
		silence:  NewSilenceManager(factory),
		notify:   notifyManager,
	}
}

func (m *Manager) Run(ctx context.Context) error {
	rules, err := m.factory.Alert().Rule().List(ctx, db.WithEnabled(true))
	if err != nil {
		return err
	}

	for i := range rules {
		rule := rules[i]
		if err = m.EvaluateRule(ctx, &rule); err != nil {
			klog.Errorf("failed to evaluate alert rule(%d:%s): %v", rule.Id, rule.Name, err)
		}
	}

	return m.DispatchPending(ctx)
}

func (m *Manager) EvaluateRule(ctx context.Context, rule *model.AlertRule) error {
	silences, err := m.silence.LoadActive(ctx, time.Now())
	if err != nil {
		klog.Errorf("failed to load active alert silences: %v", err)
	}
	return m.evaluateRule(ctx, rule, silences)
}

func (m *Manager) DispatchPending(ctx context.Context) error {
	if err := m.notify.DispatchPending(ctx); err != nil {
		klog.Errorf("failed to dispatch pending alert notifications: %v", err)
		return err
	}
	return nil
}

func (m *Manager) evaluateRule(ctx context.Context, rule *model.AlertRule, silences []model.AlertSilence) error {
	// 查到之后按哪几档规则判、判出来算多严重
	triggers := ListAlertTriggers(rule)
	if len(triggers) == 0 {
		klog.V(2).Infof("skip evaluating alert rule(%d:%s): no valid triggers in rule_config", rule.Id, rule.Name)
		return nil
	}

	samples, err := m.provider.Query(ctx, rule)
	if err != nil {
		klog.Errorf("failed to query metrics for rule(%d:%s): %v", rule.Id, rule.Name, err)
		return err
	}
	if len(samples) == 0 {
		klog.V(2).Infof("skip evaluating alert rule(%d:%s): query returned no samples (datasource=%d rule_type=%d)",
			rule.Id, rule.Name, rule.DatasourceId, rule.RuleType)
		return nil
	}

	klog.V(4).Infof("evaluating alert rule(%d:%s): triggers=%d samples=%d", rule.Id, rule.Name, len(triggers), len(samples))
	for _, trigger := range triggers {
		ruleCopy := *rule
		ruleCopy.Duration = trigger.Duration

		for _, sample := range samples {
			matched := m.eval.MatchExpr(ruleCopy.RuleType, trigger.Condition, sample.Value)
			// 符合条件
			if matched {
				// 静默告警，则忽略
				if m.silence.IsSilenced(silences, &ruleCopy, sample) {
					klog.Infof("skip firing alert rule(%d:%s) by silence", rule.Id, rule.Name)
					continue
				}
				// 不在指定时间周期内则忽略
				if !IsWithinEffectiveTime(&ruleCopy, time.Now()) {
					klog.Infof("skip firing alert rule(%d:%s) value(%s): outside effective time window", rule.Id, rule.Name, sample.Value)
					continue
				}
				klog.V(2).Infof("firing alert rule(%d:%s) value(%s) condition(%s) severity(%d)", rule.Id, rule.Name, sample.Value, formatTriggerExpr(trigger), trigger.Severity)
				// 告警和推送入库
				if err = m.trigger.Fire(ctx, &ruleCopy, sample, trigger); err != nil {
					return err
				}
				continue
			}

			klog.V(2).Infof("alert rule(%d:%s) value(%s) does not match condition(%s), attempting recovery", rule.Id, rule.Name, sample.Value, formatTriggerExpr(trigger))
			if err = m.trigger.Recover(ctx, &ruleCopy, sample, trigger); err != nil {
				return err
			}
		}
	}
	return nil
}
