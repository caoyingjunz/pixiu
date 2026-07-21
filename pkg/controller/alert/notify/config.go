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
	"encoding/json"
	"strconv"
	"strings"
)

type DingTalkChannelConfig struct {
	WebhookURL string `json:"webhook_url"`
	Secret     string `json:"secret"`
}

type WebhookChannelConfig struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type WeComChannelConfig struct {
	WebhookURL string `json:"webhook_url"`
}

type FeishuChannelConfig struct {
	WebhookURL string `json:"webhook_url"`
	Secret     string `json:"secret"`
}

type DingTalkNotificationExtension struct {
	Secret string `json:"secret"`
}

type WebhookNotificationExtension struct {
	Headers map[string]string `json:"headers"`
}

type FeishuNotificationExtension struct {
	Secret string `json:"secret"`
}

func parseNotifyChannelIDs(raw string) []int64 {
	parts := strings.Split(raw, ",")
	ids := make([]int64, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

func parseDingTalkChannelConfig(raw string) DingTalkChannelConfig {
	cfg := DingTalkChannelConfig{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cfg
	}
	_ = json.Unmarshal([]byte(raw), &cfg)
	return cfg
}

func parseWebhookChannelConfig(raw string) WebhookChannelConfig {
	cfg := WebhookChannelConfig{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cfg
	}
	_ = json.Unmarshal([]byte(raw), &cfg)
	return cfg
}

func parseWeComChannelConfig(raw string) WeComChannelConfig {
	cfg := WeComChannelConfig{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cfg
	}
	_ = json.Unmarshal([]byte(raw), &cfg)
	return cfg
}

func parseFeishuChannelConfig(raw string) FeishuChannelConfig {
	cfg := FeishuChannelConfig{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cfg
	}
	_ = json.Unmarshal([]byte(raw), &cfg)
	return cfg
}

func parseDingTalkNotificationExtension(raw string) DingTalkNotificationExtension {
	ext := DingTalkNotificationExtension{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ext
	}
	_ = json.Unmarshal([]byte(raw), &ext)
	return ext
}

func parseWebhookNotificationExtension(raw string) WebhookNotificationExtension {
	ext := WebhookNotificationExtension{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ext
	}
	_ = json.Unmarshal([]byte(raw), &ext)
	return ext
}

func parseFeishuNotificationExtension(raw string) FeishuNotificationExtension {
	ext := FeishuNotificationExtension{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ext
	}
	_ = json.Unmarshal([]byte(raw), &ext)
	return ext
}

func marshalNotificationExtension(v interface{}) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(raw)
}
