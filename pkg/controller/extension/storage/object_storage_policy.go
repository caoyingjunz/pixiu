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
	"context"
	"encoding/json"
	"fmt"
	"strings"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

// Bucket Policy 原始策略读写。
// 与 Bucket 配置里的「私有 / 公共读 / 公共读写」三档归类互补：这里是完整策略 JSON，支持精细授权。

// maxBucketPolicyBytes S3 对策略文档的限制是 20KB
const maxBucketPolicyBytes = 20 << 10

// GetBucketPolicy 读取 Bucket 策略原文；无策略时返回 Exists=false 的空 Policy 而不是报错。
func (c *controller) GetBucketPolicy(ctx context.Context, datasourceID int64, bucket string) (*types.StorageBucketPolicy, error) {
	client, cc, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	if !cc.provider.Capabilities().BucketPolicy {
		return nil, apierrors.NewError(fmt.Errorf("bucket policy management is not supported for provider %s; use the cloud provider console", cc.provider.Key()), 400)
	}
	bucket, err = requireBucket(bucket)
	if err != nil {
		return nil, err
	}
	// 无策略态已由适配器归一化为 ("", nil)，走同一路径：Policy 空 / Exists=false / ACL=private
	policy, err := client.GetBucketPolicy(ctx, bucket)
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("get bucket policy failed: %w", err), 502)
	}
	return &types.StorageBucketPolicy{
		Policy: policy,
		Exists: strings.TrimSpace(policy) != "",
		ACL:    cc.provider.ClassifyAnonymousPolicy(policy),
	}, nil
}

// SetBucketPolicy 保存策略原文，空串表示删除策略（回到「仅授权用户可访问」）。
// 这里改的是匿名访问策略，与 Bucket 配置的「读写权限」是同一份底层数据，两处修改会互相覆盖。
func (c *controller) SetBucketPolicy(ctx context.Context, datasourceID int64, bucket, policy string) error {
	client, cc, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return err
	}
	if !cc.provider.Capabilities().BucketPolicy {
		return apierrors.NewError(fmt.Errorf("bucket policy management is not supported for provider %s; use the cloud provider console", cc.provider.Key()), 400)
	}
	bucket, err = requireBucket(bucket)
	if err != nil {
		return err
	}
	policy = strings.TrimSpace(policy)
	if policy == "" {
		if err := client.SetBucketPolicy(ctx, bucket, ""); err != nil {
			return apierrors.NewError(fmt.Errorf("remove bucket policy failed: %w", err), 502)
		}
		return nil
	}
	if len(policy) > maxBucketPolicyBytes {
		return apierrors.NewError(
			fmt.Errorf("bucket policy is too large: %d bytes, limit is %d", len(policy), maxBucketPolicyBytes), 400)
	}
	if err := validateBucketPolicy(policy, cc.provider.Key() == cosProviderKey); err != nil {
		return err
	}
	if err := client.SetBucketPolicy(ctx, bucket, policy); err != nil {
		return apierrors.NewError(fmt.Errorf("set bucket policy failed: %w", err), 502)
	}
	return nil
}

// policyStatement 策略语句：只解析归类与校验需要的字段，其余原样透传给 S3。
// Sid / Condition 带 omitempty：空值若被序列化成 null 会被 S3 判为 MalformedPolicy。
type policyStatement struct {
	Sid       string      `json:"Sid,omitempty"`
	Effect    string      `json:"Effect"`
	Principal interface{} `json:"Principal"`
	Action    interface{} `json:"Action"`
	Resource  interface{} `json:"Resource"`
	Condition interface{} `json:"Condition,omitempty"`
}

type bucketPolicyDoc struct {
	Version string `json:"Version"`
	// Principal 是 COS Bucket Policy v2.0 支持的顶层授权对象（statement 内未声明 Principal 时生效）；
	// 其余厂商的文档没有该字段，序列化时省略（omitempty）。
	Principal interface{}       `json:"principal,omitempty"`
	Statement []policyStatement `json:"Statement"`
}

// flattenPolicyValues 把策略字段的三种合法写法（字符串 / 数组 / 对象）摊平成字符串列表。
func flattenPolicyValues(value interface{}) []string {
	switch v := value.(type) {
	case string:
		return []string{v}
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case map[string]interface{}:
		result := make([]string, 0)
		for _, item := range v {
			result = append(result, flattenPolicyValues(item)...)
		}
		return result
	}
	return nil
}

func containsAny(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

// validateBucketPolicy 提交给 S3 之前先做本地结构校验，省去 MalformedPolicy 往返定位。
// allowTopLevelPrincipal 为 true 时（腾讯云 COS），statement 缺 Principal 可用文档顶层 Principal 兜底。
func validateBucketPolicy(text string, allowTopLevelPrincipal bool) error {
	var doc bucketPolicyDoc
	if err := json.Unmarshal([]byte(text), &doc); err != nil {
		return apierrors.NewError(fmt.Errorf("bucket policy is not valid JSON: %w", err), 400)
	}
	if len(doc.Statement) == 0 {
		return apierrors.NewError(fmt.Errorf("bucket policy requires a non-empty Statement array"), 400)
	}
	for i, stmt := range doc.Statement {
		switch strings.ToLower(strings.TrimSpace(stmt.Effect)) {
		case "allow", "deny":
		default:
			return apierrors.NewError(fmt.Errorf("statement[%d]: Effect must be Allow or Deny", i), 400)
		}
		if stmt.Principal == nil && !(allowTopLevelPrincipal && doc.Principal != nil) {
			return apierrors.NewError(fmt.Errorf("statement[%d]: Principal is required", i), 400)
		}
		if len(flattenPolicyValues(stmt.Action)) == 0 {
			return apierrors.NewError(fmt.Errorf("statement[%d]: Action is required", i), 400)
		}
		if len(flattenPolicyValues(stmt.Resource)) == 0 {
			return apierrors.NewError(fmt.Errorf("statement[%d]: Resource is required", i), 400)
		}
	}
	return nil
}
