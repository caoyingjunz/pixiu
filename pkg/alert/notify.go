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
	"fmt"
	"strings"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

const maxNotifyRetry = 3

// NotifyManager persists and dispatches alert notifications.
type NotifyManager struct {
	factory db.ShareDaoFactory
}

func NewNotifyManager(factory db.ShareDaoFactory) *NotifyManager {
	return &NotifyManager{factory: factory}
}

func (n *NotifyManager) EnqueueForEvent(ctx context.Context, rule *model.AlertRule, event *model.AlertEvent, recovered bool) error {
	channels := parseNotifyChannels(rule.NotifyChannels)
	if len(channels) == 0 {
		channels = []model.AlertNotifyChannel{model.AlertNotifyChannelWebhook}
	}

	notifyCfg := parseRuleNotifyConfig(rule.Extension)
	title := buildNotificationTitle(rule, event, recovered)
	content := buildNotificationContent(rule, event, recovered)

	for _, channel := range channels {
		receiver, extension, err := resolveNotificationTarget(channel, notifyCfg)
		if err != nil {
			klog.Errorf("skip alert notification for rule(%d) channel(%d): %v", rule.Id, channel, err)
			continue
		}

		if _, err = n.factory.Alert().Notification().Create(ctx, &model.AlertNotification{
			EventId:    event.Id,
			RuleId:     rule.Id,
			Channel:    channel,
			Receiver:   receiver,
			Title:      title,
			Content:    content,
			Status:     model.AlertNotificationStatusPending,
			Extension:  extension,
		}); err != nil {
			return err
		}
	}
	return nil
}

func resolveNotificationTarget(channel model.AlertNotifyChannel, cfg RuleNotifyConfig) (receiver, extension string, err error) {
	switch channel {
	case model.AlertNotifyChannelDingTalk:
		receiver = strings.TrimSpace(cfg.Notify.DingTalk.WebhookURL)
		if receiver == "" {
			return "", "", fmt.Errorf("dingtalk webhook_url is not configured")
		}
		extension = marshalNotificationExtension(DingTalkNotificationExtension{
			Secret: cfg.Notify.DingTalk.Secret,
		})
		return receiver, extension, nil
	case model.AlertNotifyChannelWebhook:
		receiver = strings.TrimSpace(cfg.Notify.Webhook.URL)
		if receiver == "" {
			return "", "", fmt.Errorf("webhook url is not configured")
		}
		extension = marshalNotificationExtension(WebhookNotificationExtension{
			Headers: cfg.Notify.Webhook.Headers,
		})
		return receiver, extension, nil
	default:
		return "", "", fmt.Errorf("channel %d does not require receiver config here", channel)
	}
}

func (n *NotifyManager) DispatchPending(ctx context.Context) error {
	items, err := n.factory.Alert().Notification().ListPending(ctx, 100)
	if err != nil {
		return err
	}
	for i := range items {
		item := items[i]
		if err = n.dispatchOne(ctx, &item); err != nil {
			klog.Errorf("failed to dispatch alert notification(%d): %v", item.Id, err)
		}
	}
	return nil
}

func (n *NotifyManager) dispatchOne(ctx context.Context, item *model.AlertNotification) error {
	sendErr := sendByChannel(item)
	updates := map[string]interface{}{}
	if sendErr == nil {
		updates["status"] = model.AlertNotificationStatusSuccess
		updates["error_msg"] = ""
	} else {
		updates["retry_count"] = item.RetryCount + 1
		updates["error_msg"] = sendErr.Error()
		if item.RetryCount+1 >= maxNotifyRetry {
			updates["status"] = model.AlertNotificationStatusFailed
		}
	}
	return n.factory.Alert().Notification().Update(ctx, item.Id, item.ResourceVersion, updates)
}

func sendByChannel(item *model.AlertNotification) error {
	switch item.Channel {
	case model.AlertNotifyChannelEmail:
		return sendEmail(item)
	case model.AlertNotifyChannelDingTalk:
		return sendDingTalk(item)
	case model.AlertNotifyChannelWeCom:
		return sendWeCom(item)
	case model.AlertNotifyChannelWebhook:
		return sendWebhook(item)
	default:
		return fmt.Errorf("unsupported notify channel: %d", item.Channel)
	}
}

func sendEmail(item *model.AlertNotification) error {
	_ = item
	return fmt.Errorf("email notification is not implemented")
}

func sendWeCom(item *model.AlertNotification) error {
	_ = item
	return fmt.Errorf("wecom notification is not implemented")
}
