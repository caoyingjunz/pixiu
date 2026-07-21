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
		klog.Errorf("failed to get active event for rule(%d:%s) resource(%s/%s): %v", rule.Id, rule.Name, resourceType, resourceName, err)
		return err
	}

	now := time.Now()
	if active != nil {
		if active.NotifyCurNumber == 0 {
			if !t.durationSatisfied(active, trigger.Duration) {
				klog.V(1).Infof("alert rule(%d:%s) resource(%s/%s): waiting for duration %ds to be satisfied",
					rule.Id, rule.Name, resourceType, resourceName, trigger.Duration)
				return t.factory.Alert().Event().Update(ctx, active.Id, active.ResourceVersion, map[string]interface{}{"trigger_value": sample.Value})
			}
			klog.Infof("alert rule(%d:%s) resource(%s/%s): duration satisfied, sending first notification for event(%d)",
				rule.Id, rule.Name, resourceType, resourceName, active.Id)
			return t.sendFirstNotification(ctx, rule, active, sample, now)
		}

		klog.V(2).Infof("alert rule(%d:%s) resource(%s/%s): evaluating repeat for event(%d)",
			rule.Id, rule.Name, resourceType, resourceName, active.Id)
		return t.fireRepeat(ctx, rule, active, sample, now)
	}

	if trigger.Duration > 0 {
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
			LastSentAt:        nil,
			NotifyCurNumber:   0,
			Labels:            encodeJSONMap(sample.Labels),
			Annotations:       encodeJSONMap(sample.Annotations),
		})
		if err != nil {
			return err
		}
		if t.durationSatisfied(event, trigger.Duration) {
			klog.Infof("alert rule(%d:%s) resource(%s/%s): duration already satisfied, sending first notification for event(%d)",
				rule.Id, rule.Name, resourceType, resourceName, event.Id)
			return t.sendFirstNotification(ctx, rule, event, sample, now)
		}
		klog.Infof("alert rule(%d:%s) resource(%s/%s): created pending event(%d), waiting for duration %ds",
			rule.Id, rule.Name, resourceType, resourceName, event.Id, trigger.Duration)
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
	klog.Infof("alert rule(%d:%s) resource(%s/%s): fired immediately, event(%d) created",
		rule.Id, rule.Name, resourceType, resourceName, event.Id)
	return t.notify.EnqueueForEvent(ctx, rule, event, false)
}

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
	klog.V(1).Infof("alert rule(%d:%s) resource(%s/%s): repeat notification for event(%d), count=%d",
		rule.Id, rule.Name, active.ResourceType, active.ResourceName, active.Id, active.NotifyCurNumber)
	return t.notify.EnqueueForEvent(ctx, rule, active, false)
}

func (t *Trigger) sendFirstNotification(ctx context.Context, rule *model.AlertRule, active *model.AlertEvent, sample MetricSample, now time.Time) error {
	updates := map[string]interface{}{
		"trigger_value":     sample.Value,
		"notify_cur_number": 1,
		"last_sent_at":      now,
	}
	if err := t.factory.Alert().Event().Update(ctx, active.Id, active.ResourceVersion, updates); err != nil {
		return err
	}

	active.TriggerValue = sample.Value
	active.NotifyCurNumber = 1
	active.LastSentAt = &now
	active.ResourceVersion++
	return t.notify.EnqueueForEvent(ctx, rule, active, false)
}

func (t *Trigger) Recover(ctx context.Context, rule *model.AlertRule, sample MetricSample, trigger EvaluableTrigger) error {
	return nil
	//resourceType := sample.ResourceType
	//if resourceType == "" {
	//	resourceType = "metric"
	//}
	//resourceName := sample.ResourceName
	//if resourceName == "" {
	//	resourceName = formatTriggerKey(trigger)
	//}
	//
	//active, err := t.factory.Alert().Event().GetActive(ctx, rule.Id, resourceType, resourceName)
	//if err != nil {
	//	klog.Errorf("failed to get active event for rule(%d:%s) resource(%s/%s): %v", rule.Id, rule.Name, resourceType, resourceName, err)
	//	return err
	//}
	//if active == nil {
	//	return nil
	//}
	//
	//now := time.Now()
	//if active.NotifyCurNumber == 0 {
	//	klog.V(1).Infof("alert rule(%d:%s) resource(%s/%s): recovered before duration satisfied, event(%d) closed silently",
	//		rule.Id, rule.Name, resourceType, resourceName, active.Id)
	//	return t.markRecovered(ctx, rule, active, sample.Value, now, false)
	//}
	//
	//if !t.recoveryDurationSatisfied(active, trigger.Duration) {
	//	klog.V(0).Infof("alert rule(%d:%s) resource(%s/%s): condition cleared, waiting for recovery duration %ds (last sent: %s)",
	//		rule.Id, rule.Name, resourceType, resourceName, trigger.Duration,
	//		active.LastSentAt.Format("2006-01-02 15:04:05"))
	//	return nil
	//}
	//
	//klog.Infof("alert rule(%d:%s) resource(%s/%s): recovery duration satisfied, sending recovery notification for event(%d)",
	//	rule.Id, rule.Name, resourceType, resourceName, active.Id)
	//return t.markRecovered(ctx, rule, active, sample.Value, now, true)
}

func (t *Trigger) markRecovered(ctx context.Context, rule *model.AlertRule, active *model.AlertEvent, recoverValue string, now time.Time, notifyRecovery bool) error {
	updates := map[string]interface{}{
		"status":        model.AlertEventStatusRecovered,
		"recover_value": recoverValue,
		"recover_time":  now,
	}
	if err := t.factory.Alert().Event().Update(ctx, active.Id, active.ResourceVersion, updates); err != nil {
		return err
	}

	active.Status = model.AlertEventStatusRecovered
	active.RecoverValue = recoverValue
	active.RecoverTime = &now
	if notifyRecovery {
		return t.notify.EnqueueForEvent(ctx, rule, active, true)
	}
	return nil
}

// minRecoveryDurationSeconds 是 Duration=0 时的最小恢复稳定窗口，
// 防止立即触发规则在条件震荡时反复 Fire→Recover→Fire。
const minRecoveryDurationSeconds = 300

// recoveryDurationSatisfied 使用 LastSentAt 作为最后一次通知的时间锚点。
func (t *Trigger) recoveryDurationSatisfied(event *model.AlertEvent, durationSeconds int) bool {
	if event == nil || event.LastSentAt == nil {
		return false
	}
	if durationSeconds <= 0 {
		durationSeconds = minRecoveryDurationSeconds
	}
	return time.Since(*event.LastSentAt) >= time.Duration(durationSeconds)*time.Second
}

func (t *Trigger) durationSatisfied(event *model.AlertEvent, durationSeconds int) bool {
	if durationSeconds <= 0 {
		return true
	}
	if event == nil {
		return false
	}
	return time.Since(event.GmtCreate) >= time.Duration(durationSeconds)*time.Second
}
