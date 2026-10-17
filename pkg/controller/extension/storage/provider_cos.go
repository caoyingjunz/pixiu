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
	"regexp"
	"strings"
)

// 腾讯云 COS。区域由用户显式填写；Bucket 名必须带所属账号的 AppID 后缀。
const cosProviderKey = "cos"

var (
	// cosBucketNameWithAppID Bucket 名是否已带 AppID 后缀
	cosBucketNameWithAppID = regexp.MustCompile(`-(\d+)$`)
)

type cosProvider struct {
	publicCloudProvider
}

func (cosProvider) Key() string { return cosProviderKey }

// Capabilities COS 原生适配器支持授权策略（Bucket Policy v2.0）与默认加密（SSE-COS），标签沿用。
func (cosProvider) Capabilities() Capabilities {
	return Capabilities{
		BucketPolicy:  true,
		SSEEncryption: true,
		BucketTagging: true,
	}
}

// NormalizeBucketName COS 的 Bucket 名必须带 AppID 后缀（由用户完整填写，不从 Endpoint 推导）。
func (cosProvider) NormalizeBucketName(appID, name string) (string, error) {
	if appID != "" {
		if strings.HasSuffix(name, "-"+appID) {
			return name, nil
		}
		return name + "-" + appID, nil
	}
	if cosBucketNameWithAppID.MatchString(name) {
		return name, nil
	}
	return "", fmt.Errorf("COS bucket name must include AppID in the format <bucketname>-<appid>")
}

// COS 的匿名访问策略使用私有词汇（Bucket Policy v2.0）：
//   - 动作形如 name/cos:GetObject（兼容 cos:GetObject 写法），桶操作前缀为 name/cos:；
//   - 匿名 Principal 是 qcs 身份 qcs::cam::anyone:anyone；
//   - 资源形如 qcs::cos:<region>:uid/<appid>:<bucket>/*。
const cosAnonymousPrincipal = "qcs::cam::anyone:anyone"

// cosPolicyStatement / cosPolicyDoc 是 COS 策略文档的最小化生成模型（字段名与 COS 文档一致）。
type cosPolicyStatement struct {
	Principal map[string][]string `json:"principal"`
	Action    []string            `json:"action"`
	Effect    string              `json:"effect"`
	Resource  []string            `json:"resource"`
}

type cosPolicyDoc struct {
	Statement []cosPolicyStatement `json:"statement"`
	Version   string               `json:"version"`
}

// normalizeCosAction 归一化 COS 动作名：去 name/ 前缀、转小写。
func normalizeCosAction(action string) string {
	a := strings.ToLower(strings.TrimSpace(action))
	return strings.TrimPrefix(a, "name/")
}

// ClassifyAnonymousPolicy 将 COS 匿名访问策略归类为读写权限级别；解析失败按最小权限处理。
// 兼容 COS 的大小写习惯（Effect 常见小写 allow）与 name/cos:、cos: 两种动作前缀。
func (cosProvider) ClassifyAnonymousPolicy(policyText string) string {
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
		// 只归类「匿名可访问」的策略：Principal 必须是 * 或 COS 的 everyone 身份
		anonymous := false
		for _, principal := range flattenPolicyValues(stmt.Principal) {
			if principal == "*" || principal == cosAnonymousPrincipal {
				anonymous = true
				break
			}
		}
		if !anonymous {
			continue
		}
		for _, action := range flattenPolicyValues(stmt.Action) {
			switch normalizeCosAction(action) {
			case "*", "cos:*":
				hasGet, hasWrite = true, true
			case "cos:getobject":
				hasGet = true
			case "cos:putobject", "cos:deleteobject":
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

// BuildAnonymousPolicy 生成 COS 匿名访问策略（Bucket Policy v2.0）。
// 资源 qcs::cos:<region>:uid/<appid>:<bucket>/*；appid 从 bucket 名的 -<appid> 后缀提取，
// 提取不到（历史数据）时退化为 uid/* 通配。
func (cosProvider) BuildAnonymousPolicy(bucket, region, acl string) string {
	actions := []string{"name/cos:GetObject"}
	if acl == "public-read-write" {
		actions = append(actions, "name/cos:PutObject", "name/cos:DeleteObject")
	}
	region = strings.TrimSpace(region)
	if region == "" {
		region = "*"
	}
	appID := "*"
	if m := cosBucketNameWithAppID.FindStringSubmatch(bucket); len(m) == 2 {
		appID = m[1]
	}
	doc := cosPolicyDoc{
		Version: "2.0",
		Statement: []cosPolicyStatement{
			{
				Principal: map[string][]string{"qcs": {cosAnonymousPrincipal}},
				Action:    actions,
				Effect:    "Allow",
				Resource:  []string{fmt.Sprintf("qcs::cos:%s:uid/%s:%s/*", region, appID, bucket)},
			},
		},
	}
	data, _ := json.Marshal(doc)
	return string(data)
}
