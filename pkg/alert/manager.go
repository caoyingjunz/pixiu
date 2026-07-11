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
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

// MetricSample is a normalized metric value for rule evaluation.
type MetricSample struct {
	Value        string
	ResourceType string
	ResourceName string
	Namespace    string
	ClusterId    int64
	TenantId     int64
	Labels       map[string]string
	Annotations  map[string]string
}

// MetricProvider fetches metric samples for a rule.
type MetricProvider interface {
	Query(ctx context.Context, rule *model.AlertRule) ([]MetricSample, error)
}

// Manager coordinates rule evaluation and notification dispatch.
type Manager struct {
	factory  db.ShareDaoFactory
	provider MetricProvider
	eval     *Evaluator
	trigger  *Trigger
	silence  *SilenceManager
	notify   *NotifyManager
}

func NewManager(factory db.ShareDaoFactory, provider MetricProvider) *Manager {
	notify := NewNotifyManager(factory)
	return &Manager{
		factory:  factory,
		provider: provider,
		eval:     NewEvaluator(),
		trigger:  NewTrigger(factory, notify),
		silence:  NewSilenceManager(factory),
		notify:   notify,
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

// EvaluateRule evaluates a single alert rule.
func (m *Manager) EvaluateRule(ctx context.Context, rule *model.AlertRule) error {
	silences, err := m.silence.LoadActive(ctx, time.Now())
	if err != nil {
		klog.Errorf("failed to load active alert silences: %v", err)
	}
	return m.evaluateRule(ctx, rule, silences)
}

// DispatchPending sends pending alert notifications.
func (m *Manager) DispatchPending(ctx context.Context) error {
	if err := m.notify.DispatchPending(ctx); err != nil {
		klog.Errorf("failed to dispatch pending alert notifications: %v", err)
		return err
	}
	return nil
}

func (m *Manager) evaluateRule(ctx context.Context, rule *model.AlertRule, silences []model.AlertSilence) error {
	samples, err := m.provider.Query(ctx, rule)
	if err != nil {
		return err
	}
	if len(samples) == 0 {
		return nil
	}

	for _, sample := range samples {
		matched := m.eval.Match(rule, sample.Value)
		resourceType := sample.ResourceType
		if resourceType == "" {
			resourceType = "metric"
		}
		resourceName := sample.ResourceName
		if resourceName == "" {
			resourceName = rule.MetricName
		}

		if matched {
			if m.silence.IsSilenced(silences, rule, sample) {
				continue
			}
			if err = m.trigger.Fire(ctx, rule, sample); err != nil {
				return err
			}
			continue
		}

		if err = m.trigger.Recover(ctx, rule, sample); err != nil {
			return err
		}
	}
	return nil
}

// StaticMetricProvider is a placeholder provider for bootstrapping.
type StaticMetricProvider struct{}

func (p *StaticMetricProvider) Query(_ context.Context, _ *model.AlertRule) ([]MetricSample, error) {
	return nil, nil
}

func encodeJSONMap(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(raw)
}

func parseNotifyChannels(raw string) []model.AlertNotifyChannel {
	parts := strings.Split(raw, ",")
	channels := make([]model.AlertNotifyChannel, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			continue
		}
		channels = append(channels, model.AlertNotifyChannel(n))
	}
	return channels
}

func buildNotificationTitle(rule *model.AlertRule, event *model.AlertEvent, recovered bool) string {
	if recovered {
		return fmt.Sprintf("[恢复] %s", rule.Name)
	}
	return fmt.Sprintf("[告警] %s", rule.Name)
}

func buildNotificationContent(rule *model.AlertRule, event *model.AlertEvent, recovered bool) string {
	if strings.TrimSpace(rule.NotifyTemplate) != "" {
		return rule.NotifyTemplate
	}
	if recovered {
		return fmt.Sprintf("规则 %s 已恢复，当前值: %s", rule.Name, event.RecoverValue)
	}
	return fmt.Sprintf("规则 %s 触发，当前值: %s，条件: %s", rule.Name, event.TriggerValue, event.TriggerExpr)
}
