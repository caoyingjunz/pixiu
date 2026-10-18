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

package storage

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/caoyingjunz/pixiu/pkg/controller/extension/storage/adapter"
)

// 华为云 OBS。区域由用户显式填写；OBS 使用虚拟主机寻址。
const obsProviderKey = "obs"

type obsProvider struct {
	publicCloudProvider
}

func (obsProvider) Key() string { return obsProviderKey }

// NewAdapter 构造华为云 OBS 原生适配器。
func (obsProvider) NewAdapter(cfg adapter.Config) (adapter.Adapter, error) {
	return adapter.NewOBS(cfg)
}

// Capabilities OBS 原生适配器支持授权策略（S3 格式桶策略）与默认加密（SSE-OBS），标签沿用。
func (obsProvider) Capabilities() Capabilities {
	return Capabilities{
		BucketPolicy:  true,
		SSEEncryption: true,
		BucketTagging: true,
	}
}

// OBS 的桶策略同时兼容两种词汇：OBS 原生格式（Action 无前缀如 GetObject、Resource 如 bucket/*，
// Principal 形如 {"ID":["*"]}）与 S3 格式（Action 带 s3: 前缀、Resource 为 arn:aws:s3:::bucket/*）。
// 适配层统一按 S3 格式生成（与前端模板的 S3 语言一致），归类时对两种词汇都识别。

// normalizeOBSAction 归一化 OBS 动作名：去 s3: 前缀、转小写。
func normalizeOBSAction(action string) string {
	a := strings.ToLower(strings.TrimSpace(action))
	return strings.TrimPrefix(a, "s3:")
}

// ClassifyAnonymousPolicy 归类 OBS 匿名访问策略；解析失败按最小权限处理。
func (obsProvider) ClassifyAnonymousPolicy(policyText string) string {
	if strings.TrimSpace(policyText) == "" {
		return "private"
	}
	var doc bucketPolicyDoc
	if err := json.Unmarshal([]byte(policyText), &doc); err != nil {
		return "private"
	}
	hasGet, hasWrite := false, false
	for _, stmt := range doc.Statement {
		if !strings.EqualFold(strings.TrimSpace(stmt.Effect), "allow") {
			continue
		}
		// 只归类「匿名可访问」的策略：Principal 摊平后必须命中 *（兼容 {"AWS":["*"]} 与 {"ID":["*"]}）
		if !containsAny(flattenPolicyValues(stmt.Principal), "*") {
			continue
		}
		for _, action := range flattenPolicyValues(stmt.Action) {
			switch normalizeOBSAction(action) {
			case "*":
				hasGet, hasWrite = true, true
			case "getobject":
				hasGet = true
			case "putobject", "deleteobject":
				hasWrite = true
			}
		}
	}
	switch {
	case hasGet && hasWrite:
		return "public-read-write"
	case hasGet:
		return "public-read"
	default:
		return "private"
	}
}

// BuildAnonymousPolicy 生成 OBS 匿名访问策略（S3 格式：Version 2008-10-17 + arn:aws 资源）。
func (obsProvider) BuildAnonymousPolicy(bucket, region, acl string) string {
	actions := []string{"s3:GetObject"}
	if acl == "public-read-write" {
		actions = append(actions, "s3:PutObject", "s3:DeleteObject")
	}
	doc := bucketPolicyDoc{
		Version: "2008-10-17",
		Statement: []policyStatement{
			{
				Effect:    "Allow",
				Principal: map[string][]string{"AWS": {"*"}},
				Action:    actions,
				Resource:  []string{fmt.Sprintf("arn:aws:s3:::%s/*", bucket)},
			},
		},
	}
	data, _ := json.Marshal(doc)
	return string(data)
}
