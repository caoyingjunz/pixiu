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

	"github.com/caoyingjunz/pixiu/pkg/controller/alert/notify"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

type Trigger struct {
	factory db.ShareDaoFactory
	notify  *notify.Manager
}

func NewTrigger(factory db.ShareDaoFactory, notifyManager *notify.Manager) *Trigger {
	return &Trigger{factory: factory, notify: notifyManager}
}

func (t *Trigger) Fire(ctx context.Context, rule *model.AlertRule, sample MetricSample, trigger EvaluableTrigger) error {
	resourceType := sample.ResourceType
	if resourceType == "" {
		resourceType = "metric"
	}
	resourceName := sample.ResourceName
	if resourceName == "" {
		resourceName = formatTriggerKey(trigger)
	}

	active, err := t.factory.Alert().Event().GetActive(ctx, rule.Id, resourceType, resourceName)
	if err != nil {
		return err
	}

	now := time.Now()
	if active != nil {
		return t.fireRepeat(ctx, rule, active, sample, now)
	}

	if trigger.Duration > 0 && !t.durationSatisfied(nil, trigger.Duration) {
		return nil
	}

	lastSent := now
	event, err := t.factory.Alert().Event().Create(ctx, &model.AlertEvent{
		RuleId:            rule.Id,
		RuleName:          rule.Name,
		Status:            model.AlertEventStatusFiring,
		Severity:          trigger.Severity,
		TriggerValue:      sample.Value,
		TriggerExpr:       formatTriggerExpr(trigger),
		ResourceType:      resourceType,
		ResourceName:      resourceName,
		ResourceNamespace: sample.Namespace,
		ClusterId:         sample.ClusterId,
		TenantId:          sample.TenantId,
		LastSentAt:        &lastSent,
		NotifyCurNumber:   1,
		Labels:            encodeJSONMap(sample.Labels),
		Annotations:       encodeJSONMap(sample.Annotations),
	})
	if err != nil {
		return err
	}
	return t.notify.EnqueueForEvent(ctx, rule, event, false)
}

// fireRepeat keeps the active event and re-notifies by notify_repeat_step / notify_max_number.
func (t *Trigger) fireRepeat(ctx context.Context, rule *model.AlertRule, active *model.AlertEvent, sample MetricSample, now time.Time) error {
	updates := map[string]interface{}{
		"trigger_value": sample.Value,
	}

	repeatStep := NormalizeNotifyRepeatStep(rule.NotifyRepeatStep)
	if repeatStep == 0 {
		return t.factory.Alert().Event().Update(ctx, active.Id, active.ResourceVersion, updates)
	}

	lastSent := active.LastSentAt
	if lastSent == nil {
		lastSent = &active.GmtCreate
	}
	if now.Before(lastSent.Add(time.Duration(repeatStep) * time.Minute)) {
		return t.factory.Alert().Event().Update(ctx, active.Id, active.ResourceVersion, updates)
	}

	maxNumber := NormalizeNotifyMaxNumber(rule.NotifyMaxNumber)
	if maxNumber > 0 && active.NotifyCurNumber >= maxNumber {
		return t.factory.Alert().Event().Update(ctx, active.Id, active.ResourceVersion, updates)
	}

	nextCount := active.NotifyCurNumber + 1
	if nextCount <= 0 {
		nextCount = 1
	}
	updates["notify_cur_number"] = nextCount
	updates["last_sent_at"] = now
	if err := t.factory.Alert().Event().Update(ctx, active.Id, active.ResourceVersion, updates); err != nil {
		return err
	}

	active.TriggerValue = sample.Value
	active.NotifyCurNumber = nextCount
	active.LastSentAt = &now
	active.ResourceVersion++
	return t.notify.EnqueueForEvent(ctx, rule, active, false)
}

func (t *Trigger) Recover(ctx context.Context, rule *model.AlertRule, sample MetricSample, trigger EvaluableTrigger) error {
	resourceType := sample.ResourceType
	if resourceType == "" {
		resourceType = "metric"
	}
	resourceName := sample.ResourceName
	if resourceName == "" {
		resourceName = formatTriggerKey(trigger)
	}

	active, err := t.factory.Alert().Event().GetActive(ctx, rule.Id, resourceType, resourceName)
	if err != nil || active == nil {
		return err
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":        model.AlertEventStatusRecovered,
		"recover_value": sample.Value,
		"recover_time":  now,
	}
	if err = t.factory.Alert().Event().Update(ctx, active.Id, active.ResourceVersion, updates); err != nil {
		return err
	}

	active.Status = model.AlertEventStatusRecovered
	active.RecoverValue = sample.Value
	active.RecoverTime = &now
	return t.notify.EnqueueForEvent(ctx, rule, active, true)
}

func (t *Trigger) durationSatisfied(_ *model.AlertEvent, _ int) bool {
	return true
}
