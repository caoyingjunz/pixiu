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

package notify

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

const maxNotifyRetry = 3

type Manager struct {
	factory db.ShareDaoFactory
}

func NewManager(factory db.ShareDaoFactory) *Manager {
	return &Manager{factory: factory}
}

func (n *Manager) EnqueueForEvent(ctx context.Context, rule *model.AlertRule, event *model.AlertEvent, recovered bool) error {
	channelIDs := parseNotifyChannelIDs(rule.NotifyChannels)
	if len(channelIDs) == 0 {
		return nil
	}

	channels, err := n.factory.Alert().Channel().List(ctx, db.WithIDIn(channelIDs...), db.WithEnabled(true))
	if err != nil {
		return err
	}

	channelByID := make(map[int64]model.AlertChannel, len(channels))
	for i := range channels {
		channelByID[channels[i].Id] = channels[i]
	}

	title := buildNotificationTitle(rule, event, recovered)
	content := buildNotificationContent(rule, event, recovered)

	for _, channelID := range channelIDs {
		channel, ok := channelByID[channelID]
		if !ok {
			klog.Errorf("skip alert notification for rule(%d): channel(%d) not found or disabled", rule.Id, channelID)
			continue
		}

		receiver, extension, err := resolveNotificationTargetFromChannel(&channel)
		if err != nil {
			klog.Errorf("skip alert notification for rule(%d) channel(%d): %v", rule.Id, channelID, err)
			continue
		}

		if _, err = n.factory.Alert().Notification().Create(ctx, &model.AlertNotification{
			EventId:   event.Id,
			RuleId:    rule.Id,
			Channel:   channel.ChannelType,
			Receiver:  receiver,
			Title:     title,
			Content:   content,
			Status:    model.AlertNotificationStatusPending,
			Extension: extension,
		}); err != nil {
			return err
		}
	}
	return nil
}

func resolveNotificationTargetFromChannel(channel *model.AlertChannel) (receiver, extension string, err error) {
	if channel == nil {
		return "", "", fmt.Errorf("alert channel is nil")
	}
	if !channel.Enabled {
		return "", "", fmt.Errorf("alert channel(%d) is disabled", channel.Id)
	}

	switch channel.ChannelType {
	case model.AlertNotifyChannelDingTalk:
		cfg := parseDingTalkChannelConfig(channel.Config)
		receiver = strings.TrimSpace(cfg.WebhookURL)
		if receiver == "" {
			return "", "", fmt.Errorf("dingtalk webhook_url is not configured in channel(%d)", channel.Id)
		}
		extension = marshalNotificationExtension(DingTalkNotificationExtension{
			Secret: cfg.Secret,
		})
		return receiver, extension, nil
	case model.AlertNotifyChannelWebhook:
		cfg := parseWebhookChannelConfig(channel.Config)
		receiver = strings.TrimSpace(cfg.URL)
		if receiver == "" {
			return "", "", fmt.Errorf("webhook url is not configured in channel(%d)", channel.Id)
		}
		extension = marshalNotificationExtension(WebhookNotificationExtension{
			Headers: cfg.Headers,
		})
		return receiver, extension, nil
	case model.AlertNotifyChannelFeishu:
		cfg := parseFeishuChannelConfig(channel.Config)
		receiver = strings.TrimSpace(cfg.WebhookURL)
		if receiver == "" {
			return "", "", fmt.Errorf("feishu webhook_url is not configured in channel(%d)", channel.Id)
		}
		extension = marshalNotificationExtension(FeishuNotificationExtension{
			Secret: cfg.Secret,
		})
		return receiver, extension, nil
	default:
		return "", "", fmt.Errorf("channel type %d is not supported in channel(%d)", channel.ChannelType, channel.Id)
	}
}

func (n *Manager) DispatchPending(ctx context.Context) error {
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

func (n *Manager) dispatchOne(ctx context.Context, item *model.AlertNotification) error {
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
	case model.AlertNotifyChannelFeishu:
		return sendFeishu(item)
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

func sendFeishu(item *model.AlertNotification) error {
	_ = item
	return fmt.Errorf("feishu notification is not implemented")
}
