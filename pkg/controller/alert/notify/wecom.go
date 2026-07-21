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
	"fmt"
	"strings"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

func sendWeCom(item *model.AlertNotification) error {
	webhookURL := strings.TrimSpace(item.Receiver)
	if webhookURL == "" {
		return fmt.Errorf("wecom webhook_url is empty")
	}

	payload := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": normalizeNotifyNewlines(item.Content),
		},
	}

	body, _, err := postJSON(webhookURL, nil, payload)
	if err != nil {
		return err
	}

	var resp botResponse
	if err = json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("invalid wecom response: %w", err)
	}
	if resp.ErrCode != 0 {
		return fmt.Errorf("wecom api error: code=%d msg=%s", resp.ErrCode, resp.ErrMsg)
	}
	return nil
}
