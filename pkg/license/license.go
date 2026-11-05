/*
Copyright 2021 The Pixiu Authors.

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

// Package license 实现 Pixiu 的商业 License 机制。
//
// License 采用 Ed25519 离线签发的文本文件（license.key），格式为：
//
//	<base64url(payload JSON)>.<base64url(signature)>
//
// 服务端内嵌验签公钥，启动时加载并验签，运行期支持热更新。
// 未配置 license 时自动降级为社区版（Community），仅放开基础功能。
//
// 版本与功能划分见 docs/license.md。
package license

import (
	"errors"
	"time"
)

// ErrLicenseRequired 付费功能未授权（license 缺失/过期/无该功能）
var ErrLicenseRequired = errors.New("该功能需要商业授权，请配置有效的 license")

// Edition License 版本（价位）划分
type Edition string

const (
	// EditionCommunity 社区版：无 license 或 license 失效时的兜底版本
	EditionCommunity Edition = "community"
	// EditionProfessional 专业版
	EditionProfessional Edition = "professional"
	// EditionEnterprise 企业版：默认拥有全部付费功能
	EditionEnterprise Edition = "enterprise"
)

// Feature 付费功能标识，与 URL 前缀映射见 middleware/license.go
type Feature string

const (
	// FeatureBackup 集群备份恢复
	FeatureBackup Feature = "backup"
	// FeatureAlert 告警中心
	FeatureAlert Feature = "alert"
	// FeatureExtension 扩展（定时扩缩容等）
	FeatureExtension Feature = "extension"
	// FeatureDistribution 多集群分发
	FeatureDistribution Feature = "distribution"
	// FeatureApps 应用商店高级能力
	FeatureApps Feature = "apps"
	// FeatureAssistant AI 助手
	FeatureAssistant Feature = "assistant"
)

// AllFeatures 全部付费功能集合
var AllFeatures = []Feature{
	FeatureBackup,
	FeatureAlert,
	FeatureExtension,
	FeatureDistribution,
	FeatureApps,
	FeatureAssistant,
}

// 社区版默认配额（未配置 license 时生效）
const (
	DefaultCommunityMaxClusters = 3
	DefaultCommunityMaxNodes    = 50
)

// Payload license 载荷，签发与验签的统一数据结构。
// 序列化后参与签名，字段顺序不影响验签（对整个 JSON 字节串签名）。
type Payload struct {
	// Customer 客户/授权对象标识
	Customer string `json:"customer"`
	// Edition 版本：community / professional / enterprise
	Edition Edition `json:"edition"`
	// Features 授权的付费功能列表（enterprise 版可省略，默认全量）
	Features []Feature `json:"features,omitempty"`
	// MaxClusters 允许纳管的最大集群数，0 表示不限制
	MaxClusters int `json:"max_clusters,omitempty"`
	// MaxNodes 允许纳管的最大主机数，0 表示不限制
	MaxNodes int `json:"max_nodes,omitempty"`
	// NotBefore 生效时间（Unix 秒），0 表示立即生效
	NotBefore int64 `json:"not_before,omitempty"`
	// ExpiresAt 过期时间（Unix 秒），0 表示永不过期
	ExpiresAt int64 `json:"expires_at,omitempty"`
	// Fingerprint 机器指纹（部署环境标识），空表示不绑定部署环境
	Fingerprint string `json:"fingerprint,omitempty"`
	// IssuedAt 签发时间（Unix 秒）
	IssuedAt int64 `json:"issued_at"`
}

// IsExpired 判断 license 是否已过期
func (p *Payload) IsExpired() bool {
	if p == nil || p.ExpiresAt == 0 {
		return false
	}
	return time.Now().Unix() >= p.ExpiresAt
}

// IsNotEffective 判断 license 是否尚未生效
func (p *Payload) IsNotEffective() bool {
	if p == nil || p.NotBefore == 0 {
		return false
	}
	return time.Now().Unix() < p.NotBefore
}

// HasFeature 判断载荷是否包含指定付费功能
func (p *Payload) HasFeature(f Feature) bool {
	if p == nil {
		return false
	}
	// 企业版默认全量放开
	if p.Edition == EditionEnterprise {
		return true
	}
	for _, item := range p.Features {
		if item == f {
			return true
		}
	}
	return false
}

// Quota 配额信息（-1 表示不限制）
type Quota struct {
	MaxClusters int `json:"max_clusters"`
	MaxNodes    int `json:"max_nodes"`
}

// State license 当前状态
type State string

const (
	// StateValid license 有效
	StateValid State = "valid"
	// StateCommunity 未配置或未找到 license，运行在社区版
	StateCommunity State = "community"
	// StateExpired license 已过期
	StateExpired State = "expired"
	// StateInvalid license 验签失败或格式非法
	StateInvalid State = "invalid"
	// StateNotEffective license 尚未生效
	StateNotEffective State = "not_effective"
	// StateFingerprintMismatch 机器指纹不匹配
	StateFingerprintMismatch State = "fingerprint_mismatch"
)

// Summary license 概要信息，用于 API 返回与前端门控展示
type Summary struct {
	// State 当前 license 状态
	State State `json:"state"`
	// Edition 生效版本（community / professional / enterprise）
	Edition Edition `json:"edition"`
	// Customer 客户标识
	Customer string `json:"customer,omitempty"`
	// Features 已授权的付费功能列表
	Features []Feature `json:"features,omitempty"`
	// Quota 配额信息
	Quota Quota `json:"quota"`
	// ExpiresAt 过期时间（Unix 秒），0 表示永不过期
	ExpiresAt int64 `json:"expires_at"`
	// Fingerprint 本机指纹（便于客户上报后签发）
	Fingerprint string `json:"fingerprint"`
	// Message 异常状态的补充说明
	Message string `json:"message,omitempty"`
}
