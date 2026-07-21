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
	"strconv"
	"strings"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

func sendFeishu(item *model.AlertNotification) error {
	webhookURL := strings.TrimSpace(item.Receiver)
	if webhookURL == "" {
		return fmt.Errorf("feishu webhook_url is empty")
	}

	ext := parseFeishuNotificationExtension(item.Extension)

	payload := map[string]interface{}{
		"msg_type": "text",
		"content": map[string]string{
			"text": normalizeNotifyNewlines(item.Content),
		},
	}

	secret := strings.TrimSpace(ext.Secret)
	if secret != "" {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		stringToSign := timestamp + "\n" + secret
		mac := hmac.New(sha256.New, []byte(stringToSign))
		sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		payload["timestamp"] = timestamp
		payload["sign"] = sign
	}

	body, _, err := postJSON(webhookURL, nil, payload)
	if err != nil {
		return err
	}

	var resp feishuResponse
	if err = json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("invalid feishu response: %w", err)
	}
	if resp.Code != 0 {
		return fmt.Errorf("feishu api error: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}
