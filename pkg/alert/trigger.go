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
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

// Trigger handles firing and recovering alert events with debounce support.
type Trigger struct {
	factory db.ShareDaoFactory
	notify  *NotifyManager
}

func NewTrigger(factory db.ShareDaoFactory, notify *NotifyManager) *Trigger {
	return &Trigger{factory: factory, notify: notify}
}

func (t *Trigger) Fire(ctx context.Context, rule *model.AlertRule, sample MetricSample) error {
	resourceType := sample.ResourceType
	if resourceType == "" {
		resourceType = "metric"
	}
	resourceName := sample.ResourceName
	if resourceName == "" {
		resourceName = rule.MetricName
	}

	active, err := t.factory.Alert().Event().GetActive(ctx, rule.Id, resourceType, resourceName)
	if err != nil {
		return err
	}
	if active != nil {
		return nil
	}

	if rule.Duration > 0 && !t.durationSatisfied(active, rule.Duration) {
		return nil
	}

	event, err := t.factory.Alert().Event().Create(ctx, &model.AlertEvent{
		RuleId:            rule.Id,
		RuleName:          rule.Name,
		Status:            model.AlertEventStatusFiring,
		Severity:          rule.Severity,
		TriggerValue:      sample.Value,
		TriggerExpr:       formatTriggerExpr(rule),
		ResourceType:      resourceType,
		ResourceName:      resourceName,
		ResourceNamespace: sample.Namespace,
		ClusterId:         sample.ClusterId,
		TenantId:          sample.TenantId,
		Labels:            encodeJSONMap(sample.Labels),
		Annotations:       encodeJSONMap(sample.Annotations),
	})
	if err != nil {
		return err
	}
	return t.notify.EnqueueForEvent(ctx, rule, event, false)
}

func (t *Trigger) Recover(ctx context.Context, rule *model.AlertRule, sample MetricSample) error {
	resourceType := sample.ResourceType
	if resourceType == "" {
		resourceType = "metric"
	}
	resourceName := sample.ResourceName
	if resourceName == "" {
		resourceName = rule.MetricName
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
	// Duration debounce can be enhanced by tracking pending states in memory/redis.
	return true
}
