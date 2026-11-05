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

package license

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

// Manager license 运行期管理器：加载、验签、热更新与配额判定。
// 全局单例，业务侧直接调用包级函数。
type Manager struct {
	mu sync.RWMutex

	// path license 文件路径
	path string
	// state 当前状态
	state State
	// payload 验签通过后的载荷，state != StateValid 时为 nil
	payload *Payload
	// detail 状态补充说明（加载失败原因等）
	detail string
}

var globalManager = &Manager{state: StateCommunity}

// Init 加载 license 文件并启动热更新。
// 文件不存在或校验失败不会阻断启动，服务自动降级为社区版。
func Init(path string) {
	if path == "" {
		klog.Info("[LICENSE] 未配置 license 路径，运行在社区版")
		return
	}
	globalManager.path = path

	if err := globalManager.Reload(); err != nil {
		klog.Warningf("[LICENSE] 初始加载失败，运行在社区版: %v", err)
	} else {
		klog.Infof("[LICENSE] 加载成功: edition=%s state=%s", globalManager.payloadEdition(), globalManager.state)
	}
}

// StartAutoReload 周期性重读 license 文件，支持续费后免重启换证。
func StartAutoReload(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := globalManager.Reload(); err != nil {
					// 只在状态发生变化时打日志，避免噪音
					globalManager.mu.RLock()
					changed := globalManager.state != StateValid
					globalManager.mu.RUnlock()
					if changed {
						klog.Warningf("[LICENSE] 热更新失败: %v", err)
					}
				}
			}
		}
	}()
}

// Get 返回全局 Manager 实例
func Get() *Manager { return globalManager }

// Reload 重新加载 license 文件
func (m *Manager) Reload() error {
	if m.path == "" {
		return errors.New("license 路径未配置")
	}
	data, err := os.ReadFile(m.path)
	if err != nil {
		m.set(StateCommunity, nil, "license 文件不存在")
		return fmt.Errorf("读取 license 文件失败: %w", err)
	}

	payload, err := VerifyLicense(EmbeddedPublicKey, data)
	if err != nil {
		m.set(StateInvalid, nil, err.Error())
		return err
	}
	if err = validate(payload); err != nil {
		switch {
		case errors.Is(err, ErrNotEffective):
			m.set(StateNotEffective, nil, "license 尚未生效")
		case errors.Is(err, ErrExpired):
			m.set(StateExpired, nil, "license 已过期")
		default:
			m.set(StateFingerprintMismatch, nil, "license 与当前部署环境不匹配")
		}
		return err
	}

	m.set(StateValid, payload, "")
	return nil
}

// license 校验错误哨兵
var (
	ErrNotEffective = errors.New("license 尚未生效")
	ErrExpired      = errors.New("license 已过期")
	ErrMismatch     = errors.New("license 与当前部署环境不匹配")
)

// validate 校验载荷有效期与机器指纹
func validate(p *Payload) error {
	if p.IsNotEffective() {
		return ErrNotEffective
	}
	if p.IsExpired() {
		return ErrExpired
	}
	if p.Fingerprint != "" && p.Fingerprint != Fingerprint() {
		return ErrMismatch
	}
	return nil
}

// set 更新状态（内部使用）
func (m *Manager) set(state State, payload *Payload, detail string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = state
	m.payload = payload
	m.detail = detail
}

// State 返回当前 license 状态
func (m *Manager) State() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// Payload 返回当前生效的载荷（无效时为 nil）
func (m *Manager) Payload() *Payload {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.payload
}

// IsValid 判断当前 license 是否有效
func (m *Manager) IsValid() bool {
	return m.State() == StateValid
}

// HasFeature 判断当前是否授权指定付费功能。
// 仅 valid 状态的 license 放开付费功能，其余状态一律视为社区版。
func (m *Manager) HasFeature(f Feature) bool {
	if !m.IsValid() {
		return false
	}
	return m.Payload().HasFeature(f)
}

// Quota 返回当前配额，-1 表示不限制。
// 社区版（未授权）使用内置默认配额。
func (m *Manager) Quota() Quota {
	if !m.IsValid() {
		return Quota{MaxClusters: DefaultCommunityMaxClusters, MaxNodes: DefaultCommunityMaxNodes}
	}
	p := m.Payload()
	return Quota{
		MaxClusters: quotaOrDefault(p.MaxClusters),
		MaxNodes:    quotaOrDefault(p.MaxNodes),
	}
}

// quotaOrDefault 0（未填写）表示不限制，统一转为 -1 便于判定
func quotaOrDefault(v int) int {
	if v <= 0 {
		return -1
	}
	return v
}

// CheckClusterQuota 校验集群数配额，existing 为当前已纳管集群数
func (m *Manager) CheckClusterQuota(existing int) error {
	limit := m.Quota().MaxClusters
	if limit < 0 || existing < limit {
		return nil
	}
	return fmt.Errorf("已达 license 集群数上限（%d），请升级或续费 license", limit)
}

// CheckNodeQuota 校验主机数配额，existing 为当前已纳管主机数
func (m *Manager) CheckNodeQuota(existing int) error {
	limit := m.Quota().MaxNodes
	if limit < 0 || existing < limit {
		return nil
	}
	return fmt.Errorf("已达 license 主机数上限（%d），请升级或续费 license", limit)
}

// Summary 返回 license 概要信息（API 用）
func (m *Manager) Summary() Summary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := Summary{
		State:       m.state,
		Fingerprint: Fingerprint(),
	}
	if m.state == StateValid && m.payload != nil {
		summary.Edition = m.payload.Edition
		summary.Customer = m.payload.Customer
		summary.ExpiresAt = m.payload.ExpiresAt
		summary.Quota = m.Quota()
		if m.payload.Edition == EditionEnterprise {
			summary.Features = AllFeatures
		} else {
			summary.Features = m.payload.Features
		}
		return summary
	}

	// 社区版兜底
	summary.Edition = EditionCommunity
	summary.Quota = m.Quota()
	if m.detail != "" {
		summary.Message = m.detail
	}
	return summary
}

// payloadEdition 返回载荷版本（日志用）
func (m *Manager) payloadEdition() Edition {
	if m.payload == nil {
		return EditionCommunity
	}
	return m.payload.Edition
}
