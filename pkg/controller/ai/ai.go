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

package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type Getter interface {
	AI() Interface
}

type Interface interface {
	Respond(ctx context.Context, req *types.AIRespondRequest) (*types.AIRespondResponse, error)
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory
	client  *http.Client
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{
		cc:      cfg,
		factory: f,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *controller) Respond(ctx context.Context, req *types.AIRespondRequest) (*types.AIRespondResponse, error) {
	userId, err := httputils.GetUserIdFromContext(ctx)
	if err != nil {
		return nil, apierrors.ErrUnauthorized
	}

	account, err := c.getEnabledAccount(ctx, userId, req.Provider)
	if err != nil {

		return nil, err
	}

	modelName := req.Model
	if modelName == "" {
		modelName = account.ModelName
	}
	if modelName == "" {
		return nil, apierrors.NewError(fmt.Errorf("model is required"), http.StatusBadRequest)
	}
	if strings.TrimSpace(account.BaseURL) == "" {
		return nil, apierrors.NewError(fmt.Errorf("base_url is empty"), http.StatusBadRequest)
	}
	if strings.TrimSpace(account.APIKey) == "" {
		return nil, apierrors.NewError(fmt.Errorf("api_key is empty"), http.StatusBadRequest)
	}

	payload := map[string]interface{}{
		"model": modelName,
		"input": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type": "input_text",
						"text": req.Input,
					},
				},
			},
		},
		"stream": true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}

	endpoint := strings.TrimRight(account.BaseURL, "/") + "/v1/responses"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+account.APIKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		klog.Errorf("failed to request ai endpoint %s: %v", endpoint, err)
		return nil, apierrors.NewError(fmt.Errorf("request ai endpoint failed: %v", err), http.StatusBadGateway)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		klog.Errorf("ai endpoint returned status=%d body=%s", resp.StatusCode, string(respBody))
		return nil, apierrors.NewError(fmt.Errorf("ai endpoint returned status %d: %s", resp.StatusCode, string(respBody)), http.StatusBadGateway)
	}

	raw, text, err := parseResponseBody(respBody)
	if err != nil {
		klog.Errorf("failed to parse ai response: %v", err)
		return nil, apierrors.NewError(fmt.Errorf("invalid ai response"), http.StatusBadGateway)
	}
	return &types.AIRespondResponse{
		Text:  text,
		Model: modelName,
		Raw:   raw,
	}, nil
}

func (c *controller) getEnabledAccount(ctx context.Context, userId int64, provider string) (*model.AIAccount, error) {
	opts := []db.Options{
		db.WithUser(userId),
		db.WithEnabled(true),
		db.WithModifyOrderByDesc(),
		db.WithLimit(1),
	}
	if provider != "" {
		opts = append(opts, db.WithProvider(provider))
	}

	accounts, err := c.factory.AIAccount().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list ai accounts for user(%d): %v", userId, err)
		return nil, apierrors.ErrServerInternal
	}
	if len(accounts) == 0 {
		return nil, apierrors.NewError(fmt.Errorf("no enabled ai account found"), http.StatusNotFound)
	}
	return &accounts[0], nil
}

func parseResponseBody(respBody []byte) (map[string]interface{}, string, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(respBody, &raw); err == nil {
		return raw, extractResponseText(raw), nil
	}

	raw, text, err := parseSSEResponse(respBody)
	if err != nil {
		return nil, "", err
	}
	return raw, text, nil
}

func extractResponseText(raw map[string]interface{}) string {
	output, ok := raw["output"].([]interface{})
	if !ok {
		return ""
	}

	var result strings.Builder
	for _, item := range output {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		content, ok := obj["content"].([]interface{})
		if !ok {
			continue
		}
		for _, c := range content {
			contentObj, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			text, _ := contentObj["text"].(string)
			if text == "" {
				continue
			}
			if result.Len() > 0 {
				result.WriteString("\n")
			}
			result.WriteString(text)
		}
	}
	return result.String()
}

func parseSSEResponse(respBody []byte) (map[string]interface{}, string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(respBody))
	var (
		lastEvent map[string]interface{}
		texts     []string
	)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}

		var event map[string]interface{}
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}
		lastEvent = event

		if delta, ok := event["delta"].(string); ok && delta != "" {
			texts = append(texts, delta)
			continue
		}

		if itemType, _ := event["type"].(string); itemType == "response.output_text.delta" {
			if delta, _ := event["delta"].(string); delta != "" {
				texts = append(texts, delta)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, "", err
	}
	if lastEvent == nil {
		return nil, "", fmt.Errorf("empty sse response")
	}
	return lastEvent, strings.Join(texts, ""), nil
}
