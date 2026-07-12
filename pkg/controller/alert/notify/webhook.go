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
	"fmt"
	"strings"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

type webhookPayload struct {
	Title     string `json:"title"`
	Content   string `json:"content"`
	Channel   string `json:"channel"`
	EventId   int64  `json:"event_id"`
	RuleId    int64  `json:"rule_id"`
	Timestamp string `json:"timestamp"`
}

func sendWebhook(item *model.AlertNotification) error {
	targetURL := strings.TrimSpace(item.Receiver)
	if targetURL == "" {
		return fmt.Errorf("webhook url is empty")
	}

	ext := parseWebhookNotificationExtension(item.Extension)
	payload := webhookPayload{
		Title:     item.Title,
		Content:   item.Content,
		Channel:   "webhook",
		EventId:   item.EventId,
		RuleId:    item.RuleId,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	_, _, err := postJSON(targetURL, ext.Headers, payload)
	return err
}
