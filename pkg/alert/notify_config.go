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
	"encoding/json"
	"strings"
)

// Rule extension example:
// {
//   "notify": {
//     "dingtalk": {
//       "webhook_url": "https://oapi.dingtalk.com/robot/send?access_token=xxx",
//       "secret": "SEC..."
//     },
//     "webhook": {
//       "url": "https://example.com/hooks/alert",
//       "headers": {"Authorization": "Bearer token"}
//     }
//   }
// }

type RuleNotifyConfig struct {
	Notify NotifyChannelConfig `json:"notify"`
}

type NotifyChannelConfig struct {
	DingTalk DingTalkNotifyConfig `json:"dingtalk"`
	Webhook  WebhookNotifyConfig  `json:"webhook"`
}

type DingTalkNotifyConfig struct {
	WebhookURL string `json:"webhook_url"`
	Secret     string `json:"secret"`
}

type WebhookNotifyConfig struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type DingTalkNotificationExtension struct {
	Secret string `json:"secret"`
}

type WebhookNotificationExtension struct {
	Headers map[string]string `json:"headers"`
}

func parseRuleNotifyConfig(raw string) RuleNotifyConfig {
	cfg := RuleNotifyConfig{}
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

func marshalNotificationExtension(v interface{}) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(raw)
}
