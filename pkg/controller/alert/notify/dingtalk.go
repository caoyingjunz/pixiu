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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

type dingTalkResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func sendDingTalk(item *model.AlertNotification) error {
	webhookURL := strings.TrimSpace(item.Receiver)
	if webhookURL == "" {
		return fmt.Errorf("dingtalk webhook_url is empty")
	}

	ext := parseDingTalkNotificationExtension(item.Extension)
	signedURL, err := signDingTalkWebhookURL(webhookURL, ext.Secret)
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": item.Title,
			"text":  buildDingTalkMarkdown(item),
		},
	}

	body, _, err := postJSON(signedURL, nil, payload)
	if err != nil {
		return err
	}

	var resp dingTalkResponse
	if err = json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("invalid dingtalk response: %w", err)
	}
	if resp.ErrCode != 0 {
		return fmt.Errorf("dingtalk api error: code=%d msg=%s", resp.ErrCode, resp.ErrMsg)
	}
	return nil
}

func buildDingTalkMarkdown(item *model.AlertNotification) string {
	lines := []string{
		fmt.Sprintf("### %s", item.Title),
		"",
		item.Content,
		"",
		fmt.Sprintf("- 事件ID: %d", item.EventId),
		fmt.Sprintf("- 规则ID: %d", item.RuleId),
	}
	return strings.Join(lines, "\n")
}

func signDingTalkWebhookURL(webhookURL, secret string) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return webhookURL, nil
	}

	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	stringToSign := timestamp + "\n" + secret
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(stringToSign)); err != nil {
		return "", err
	}
	sign := url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))

	parsed, err := url.Parse(webhookURL)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("timestamp", timestamp)
	query.Set("sign", sign)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
