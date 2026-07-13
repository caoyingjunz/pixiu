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

type feishuResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func PingChannel(channelType model.AlertNotifyChannel, config string) error {
	switch channelType {
	case model.AlertNotifyChannelDingTalk:
		return pingDingTalkChannel(config)
	case model.AlertNotifyChannelWebhook:
		return pingWebhookChannel(config)
	case model.AlertNotifyChannelEmail:
		return fmt.Errorf("邮件渠道暂不支持连通性测试")
	case model.AlertNotifyChannelWeCom:
		return pingWeComChannel(config)
	case model.AlertNotifyChannelFeishu:
		return pingFeishuChannel(config)
	default:
		return fmt.Errorf("不支持的渠道类型: %d", channelType)
	}
}

func pingDingTalkChannel(config string) error {
	cfg := parseDingTalkChannelConfig(config)
	webhookURL := strings.TrimSpace(cfg.WebhookURL)
	if webhookURL == "" {
		return fmt.Errorf("钉钉 Webhook URL 未配置")
	}

	signedURL, err := signDingTalkWebhookURL(webhookURL, cfg.Secret)
	if err != nil {
		return fmt.Errorf("签名失败: %w", err)
	}

	payload := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": fmt.Sprintf("Pixiu 连通性测试 - %s", time.Now().Format("2006-01-02 15:04:05")),
		},
	}

	body, _, err := postJSON(signedURL, nil, payload)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}

	var resp botResponse
	if err = json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("解析钉钉响应失败: %w", err)
	}
	if resp.ErrCode != 0 {
		return fmt.Errorf("钉钉返回错误: code=%d msg=%s", resp.ErrCode, resp.ErrMsg)
	}
	return nil
}

func pingWebhookChannel(config string) error {
	cfg := parseWebhookChannelConfig(config)
	targetURL := strings.TrimSpace(cfg.URL)
	if targetURL == "" {
		return fmt.Errorf("Webhook URL 未配置")
	}

	payload := webhookPayload{
		Title:     "Pixiu 连通性测试",
		Content:   fmt.Sprintf("通知渠道连通性测试 - %s", time.Now().Format("2006-01-02 15:04:05")),
		Channel:   "webhook",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	_, _, err := postJSON(targetURL, cfg.Headers, payload)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	return nil
}

func pingWeComChannel(config string) error {
	cfg := parseWeComChannelConfig(config)
	webhookURL := strings.TrimSpace(cfg.WebhookURL)
	if webhookURL == "" {
		return fmt.Errorf("企业微信 Webhook URL 未配置")
	}

	payload := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": fmt.Sprintf("Pixiu 连通性测试 - %s", time.Now().Format("2006-01-02 15:04:05")),
		},
	}

	body, _, err := postJSON(webhookURL, nil, payload)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}

	var resp botResponse
	if err = json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("解析企业微信响应失败: %w", err)
	}
	if resp.ErrCode != 0 {
		return fmt.Errorf("企业微信返回错误: code=%d msg=%s", resp.ErrCode, resp.ErrMsg)
	}
	return nil
}

func pingFeishuChannel(config string) error {
	cfg := parseFeishuChannelConfig(config)
	webhookURL := strings.TrimSpace(cfg.WebhookURL)
	if webhookURL == "" {
		return fmt.Errorf("飞书 Webhook URL 未配置")
	}

	payload := map[string]interface{}{
		"msg_type": "text",
		"content": map[string]string{
			"text": fmt.Sprintf("Pixiu 连通性测试 - %s", time.Now().Format("2006-01-02 15:04:05")),
		},
	}

	secret := strings.TrimSpace(cfg.Secret)
	if secret != "" {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		stringToSign := timestamp + "\n" + secret
		mac := hmac.New(sha256.New, []byte(secret))
		if _, err := mac.Write([]byte(stringToSign)); err != nil {
			return fmt.Errorf("飞书签名失败: %w", err)
		}
		sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		payload["timestamp"] = timestamp
		payload["sign"] = sign
	}

	body, _, err := postJSON(webhookURL, nil, payload)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}

	var resp feishuResponse
	if err = json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("解析飞书响应失败: %w", err)
	}
	if resp.Code != 0 {
		return fmt.Errorf("飞书返回错误: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}
