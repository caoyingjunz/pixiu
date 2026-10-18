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

// 阿里云 OSS。端点填服务级端点（oss-<region>.aliyuncs.com，或以 s3.oss- 开头的 S3 兼容端点，
// 适配器会自动重写）；区域由用户显式填写。
const ossProviderKey = "oss"

type ossProvider struct {
	publicCloudProvider
}

func (ossProvider) Key() string { return ossProviderKey }

// NewAdapter 构造阿里云 OSS 原生适配器。
func (ossProvider) NewAdapter(cfg adapter.Config) (adapter.Adapter, error) {
	return adapter.NewOSS(cfg)
}

// Capabilities OSS 原生适配器支持授权策略（Bucket Policy）与默认加密（SSE-OSS），标签沿用。
func (ossProvider) Capabilities() Capabilities {
	return Capabilities{
		BucketPolicy:  true,
		SSEEncryption: true,
		BucketTagging: true,
	}
}

// OSS 的匿名访问策略词汇（Bucket Policy）：
//   - Version 固定 "1"；Principal 直接是字符串 "*"；
//   - 动作形如 oss:GetObject / oss:PutObject / oss:DeleteObject；
//   - 资源形如 acs:oss:*:*:<bucket>/*。
type ossPolicyStatement struct {
	Effect    string   `json:"Effect"`
	Principal string   `json:"Principal"`
	Action    []string `json:"Action"`
	Resource  []string `json:"Resource"`
}

type ossPolicyDoc struct {
	Version   string               `json:"Version"`
	Statement []ossPolicyStatement `json:"Statement"`
}

// normalizeOSSAction 归一化 OSS 动作名：转小写（oss:GetObject → oss:getobject）。
func normalizeOSSAction(action string) string {
	return strings.ToLower(strings.TrimSpace(action))
}

// ClassifyAnonymousPolicy 将 OSS 匿名访问策略归类为读写权限级别；解析失败按最小权限处理。
func (ossProvider) ClassifyAnonymousPolicy(policyText string) string {
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
		// 只归类「匿名可访问」的策略：Principal 必须是 *
		if !containsAny(flattenPolicyValues(stmt.Principal), "*") {
			continue
		}
		for _, action := range flattenPolicyValues(stmt.Action) {
			switch normalizeOSSAction(action) {
			case "*", "oss:*":
				hasGet, hasWrite = true, true
			case "oss:getobject":
				hasGet = true
			case "oss:putobject", "oss:deleteobject":
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

// BuildAnonymousPolicy 生成 OSS 匿名访问策略（Version 固定为 1，资源为对象级通配）。
func (ossProvider) BuildAnonymousPolicy(bucket, region, acl string) string {
	actions := []string{"oss:GetObject"}
	if acl == "public-read-write" {
		actions = append(actions, "oss:PutObject", "oss:DeleteObject")
	}
	doc := ossPolicyDoc{
		Version: "1",
		Statement: []ossPolicyStatement{
			{
				Effect:    "Allow",
				Principal: "*",
				Action:    actions,
				Resource:  []string{fmt.Sprintf("acs:oss:*:*:%s/*", bucket)},
			},
		},
	}
	data, _ := json.Marshal(doc)
	return string(data)
}
